package worker

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/checkfox/go_lead/internal/client"
	"github.com/checkfox/go_lead/internal/logger"
	"github.com/checkfox/go_lead/internal/models"
	"github.com/checkfox/go_lead/internal/queue"
	"github.com/checkfox/go_lead/internal/repository"
	"github.com/checkfox/go_lead/internal/services"
)

// Processor handles background job processing for leads
type Processor struct {
	queue                     queue.Queue
	leadRepo                  repository.LeadRepository
	deliveryAttemptRepo       repository.DeliveryAttemptRepository
	validator                 *services.Validator
	normalizer                *services.Normalizer
	mapper                    *services.Mapper
	customerAPIClient         *client.CustomerAPIClient
	pollInterval              time.Duration
	shutdownChan              chan struct{}
	maxDeliveryAttempts       int
	exponentialBackoffDelays  []time.Duration
}

// ProcessorConfig holds configuration for the worker processor
type ProcessorConfig struct {
	Queue                    queue.Queue
	LeadRepo                 repository.LeadRepository
	DeliveryAttemptRepo      repository.DeliveryAttemptRepository
	Validator                *services.Validator
	Normalizer               *services.Normalizer
	Mapper                   *services.Mapper
	CustomerAPIClient        *client.CustomerAPIClient
	PollInterval             time.Duration
	MaxDeliveryAttempts      int
	ExponentialBackoffDelays []time.Duration
}

// NewProcessor creates a new worker processor
func NewProcessor(config ProcessorConfig) *Processor {
	// Set default poll interval if not provided
	if config.PollInterval == 0 {
		config.PollInterval = 5 * time.Second
	}

	// Set default max delivery attempts if not provided
	if config.MaxDeliveryAttempts == 0 {
		config.MaxDeliveryAttempts = 5
	}

	// Set default exponential backoff delays if not provided
	if len(config.ExponentialBackoffDelays) == 0 {
		config.ExponentialBackoffDelays = []time.Duration{
			30 * time.Second,
			60 * time.Second,
			120 * time.Second,
			240 * time.Second,
			480 * time.Second,
		}
	}

	return &Processor{
		queue:                    config.Queue,
		leadRepo:                 config.LeadRepo,
		deliveryAttemptRepo:      config.DeliveryAttemptRepo,
		validator:                config.Validator,
		normalizer:               config.Normalizer,
		mapper:                   config.Mapper,
		customerAPIClient:        config.CustomerAPIClient,
		pollInterval:             config.PollInterval,
		shutdownChan:             make(chan struct{}),
		maxDeliveryAttempts:      config.MaxDeliveryAttempts,
		exponentialBackoffDelays: config.ExponentialBackoffDelays,
	}
}

// Start begins the worker polling loop with graceful shutdown
// Requirements: 5.1, 5.2, 5.5
func (p *Processor) Start(ctx context.Context) error {
	logger.Info(ctx, "Starting worker processor", "poll_interval", p.pollInterval)

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Create a ticker for polling
	ticker := time.NewTicker(p.pollInterval)
	defer ticker.Stop()

	// Start the polling loop
	for {
		select {
		case <-ctx.Done():
			logger.Info(ctx, "Context cancelled, shutting down gracefully")
			return ctx.Err()

		case <-sigChan:
			logger.Info(ctx, "Received shutdown signal, shutting down gracefully")
			return nil

		case <-p.shutdownChan:
			logger.Info(ctx, "Shutdown requested, shutting down gracefully")
			return nil

		case <-ticker.C:
			// Poll for jobs
			if err := p.pollAndProcess(ctx); err != nil {
				logger.LogError(ctx, "Error polling and processing jobs", err)
				// Continue polling even if there's an error
			}
		}
	}
}

// Shutdown signals the worker to stop gracefully
func (p *Processor) Shutdown() {
	close(p.shutdownChan)
}

// pollAndProcess polls for a job and processes it
func (p *Processor) pollAndProcess(ctx context.Context) error {
	// Dequeue the next job
	job, err := p.queue.Dequeue(ctx)
	if err != nil {
		return fmt.Errorf("failed to dequeue job: %w", err)
	}

	// No jobs available
	if job == nil {
		return nil
	}

	logger.Info(ctx, "Processing job", "job_id", job.ID, "job_type", job.Type)

	// Process the job based on its type
	var processErr error
	switch job.Type {
	case "process_lead":
		processErr = p.processLead(ctx, job)
	default:
		processErr = fmt.Errorf("unknown job type: %s", job.Type)
	}

	// Handle job completion or failure
	if processErr != nil {
		logger.LogError(ctx, "Job failed", processErr, "job_id", job.ID)
		// Mark job as failed
		if err := p.queue.Fail(ctx, job.ID, processErr.Error()); err != nil {
			logger.LogError(ctx, "Failed to mark job as failed", err, "job_id", job.ID)
		}
		return processErr
	}

	// Mark job as completed
	if err := p.queue.Complete(ctx, job.ID); err != nil {
		logger.LogError(ctx, "Failed to mark job as completed", err, "job_id", job.ID)
		return err
	}

	logger.Info(ctx, "Job completed successfully", "job_id", job.ID)
	return nil
}

// processLead processes a single lead through the validation, transformation, and delivery pipeline
// Requirements: 5.1, 5.2, 5.5
func (p *Processor) processLead(ctx context.Context, job *queue.Job) error {
	startTime := time.Now()
	
	// Extract lead_id from job payload
	leadID, ok := queue.GetLeadID(job.Payload)
	if !ok {
		return fmt.Errorf("invalid job payload: missing lead_id")
	}

	// Add lead_id to context for logging
	ctx = context.WithValue(ctx, logger.LeadIDKey, leadID)
	
	logger.Info(ctx, "Processing lead")

	// Load lead from database
	lead, err := p.leadRepo.GetLeadByID(ctx, leadID)
	if err != nil {
		logger.LogError(ctx, "Failed to load lead", err)
		return fmt.Errorf("failed to load lead %d: %w", leadID, err)
	}

	logger.Info(ctx, "Loaded lead", "status", lead.Status)

	// Execute validation stage
	if err := p.executeValidationStage(ctx, lead); err != nil {
		logger.LogError(ctx, "Validation stage failed", err)
		return err
	}

	// If lead was rejected, stop processing
	if lead.Status == models.LeadStatusRejected {
		logger.Info(ctx, "Lead was rejected, stopping processing")
		logger.LogSlowOperation(ctx, "process_lead", time.Since(startTime))
		return nil
	}

	// Execute transformation stage
	if err := p.executeTransformationStage(ctx, lead); err != nil {
		logger.LogError(ctx, "Transformation stage failed", err)
		return err
	}

	// If transformation failed (missing core fields), stop processing
	if lead.Status == models.LeadStatusFailed || lead.Status == models.LeadStatusPermanentlyFailed {
		logger.Info(ctx, "Lead transformation failed, stopping processing")
		logger.LogSlowOperation(ctx, "process_lead", time.Since(startTime))
		return nil
	}

	// Execute delivery stage
	if err := p.executeDeliveryStage(ctx, lead); err != nil {
		logger.LogError(ctx, "Delivery stage failed", err)
		return err
	}

	logger.Info(ctx, "Lead processed successfully", "final_status", lead.Status)
	logger.LogSlowOperation(ctx, "process_lead", time.Since(startTime))
	return nil
}

// executeValidationStage executes the validation stage for a lead
// Requirements: 2.3, 2.4, 2.5, 6.2
func (p *Processor) executeValidationStage(ctx context.Context, lead *models.InboundLead) error {
	logger.Info(ctx, "Executing validation stage")

	// Call validation service
	result := p.validator.ValidateLead(lead.RawPayload)

	if !result.Valid {
		// Mark lead as REJECTED on validation failure
		// Store rejection reason
		if result.RejectionReason != nil {
			logger.Info(ctx, "Lead validation failed", "rejection_reason", *result.RejectionReason)
			if err := p.leadRepo.UpdateLeadRejection(ctx, lead.ID, *result.RejectionReason); err != nil {
				return fmt.Errorf("failed to update lead rejection: %w", err)
			}
			oldStatus := lead.Status
			lead.Status = models.LeadStatusRejected
			reasonStr := result.RejectionReason.String()
			lead.RejectionReason = &reasonStr
			logger.LogStatusTransition(ctx, lead.ID, string(oldStatus), string(lead.Status))
		} else {
			return fmt.Errorf("validation failed but no rejection reason provided")
		}
		return nil
	}

	// Mark lead as READY on validation success
	logger.Info(ctx, "Lead validation passed, marking as READY")
	if err := p.leadRepo.UpdateLeadStatus(ctx, lead.ID, models.LeadStatusReady); err != nil {
		return fmt.Errorf("failed to update lead status to READY: %w", err)
	}
	oldStatus := lead.Status
	lead.Status = models.LeadStatusReady
	logger.LogStatusTransition(ctx, lead.ID, string(oldStatus), string(lead.Status))

	return nil
}

// executeTransformationStage executes the transformation stage for a lead
// Requirements: 3.4, 3.5, 3.6, 3.7, 6.3, 9.3, 9.4
func (p *Processor) executeTransformationStage(ctx context.Context, lead *models.InboundLead) error {
	logger.Info(ctx, "Executing transformation stage")

	// Call normalization service
	normalizedPayload := p.normalizer.NormalizeLeadWithFieldMapping(lead.RawPayload)
	logger.Info(ctx, "Lead normalized successfully")

	// Call mapping service
	mappingResult := p.mapper.MapToCustomerFormat(normalizedPayload)

	if !mappingResult.Success {
		// Mark lead as FAILED if core fields missing
		logger.Info(ctx, "Lead mapping failed", "errors", mappingResult.Errors)
		if err := p.leadRepo.UpdateLeadStatus(ctx, lead.ID, models.LeadStatusPermanentlyFailed); err != nil {
			return fmt.Errorf("failed to update lead status to PERMANENTLY_FAILED: %w", err)
		}
		oldStatus := lead.Status
		lead.Status = models.LeadStatusPermanentlyFailed
		logger.LogStatusTransition(ctx, lead.ID, string(oldStatus), string(lead.Status))
		return nil
	}

	// Continue if optional attributes invalid (permissive)
	if len(mappingResult.OmittedAttributes) > 0 {
		logger.Info(ctx, "Invalid optional attributes omitted",
			"count", len(mappingResult.OmittedAttributes),
			"attributes", mappingResult.OmittedAttributes)
	}

	// Store normalized and customer payloads
	if err := p.leadRepo.UpdateLeadWithPayloads(ctx, lead.ID, normalizedPayload, mappingResult.CustomerPayload); err != nil {
		return fmt.Errorf("failed to update lead payloads: %w", err)
	}
	lead.NormalizedPayload = normalizedPayload
	lead.CustomerPayload = mappingResult.CustomerPayload

	logger.Info(ctx, "Lead transformation completed successfully")
	return nil
}

// executeDeliveryStage executes the delivery stage for a lead with retry logic
// Requirements: 4.2, 4.3, 4.4, 4.5, 4.6, 4.7, 5.3, 5.4, 6.4, 9.2
func (p *Processor) executeDeliveryStage(ctx context.Context, lead *models.InboundLead) error {
	logger.Info(ctx, "Executing delivery stage")

	// Get the current attempt count
	attemptCount, err := p.deliveryAttemptRepo.CountDeliveryAttempts(ctx, lead.ID)
	if err != nil {
		return fmt.Errorf("failed to count delivery attempts: %w", err)
	}

	// Check if we've already exhausted retries
	if attemptCount >= p.maxDeliveryAttempts {
		logger.Info(ctx, "Max delivery attempts exhausted, marking as PERMANENTLY_FAILED",
			"attempt_count", attemptCount,
			"max_attempts", p.maxDeliveryAttempts)
		if err := p.leadRepo.UpdateLeadStatus(ctx, lead.ID, models.LeadStatusPermanentlyFailed); err != nil {
			return fmt.Errorf("failed to update lead status to PERMANENTLY_FAILED: %w", err)
		}
		oldStatus := lead.Status
		lead.Status = models.LeadStatusPermanentlyFailed
		logger.LogStatusTransition(ctx, lead.ID, string(oldStatus), string(lead.Status))
		return nil
	}

	// Calculate the next attempt number (1-indexed)
	nextAttemptNo := attemptCount + 1

	// If this is a retry (not the first attempt), apply exponential backoff
	if attemptCount > 0 {
		// Get the delay for this retry (attemptCount is 0-indexed for delays)
		delayIndex := attemptCount - 1
		if delayIndex < len(p.exponentialBackoffDelays) {
			delay := p.exponentialBackoffDelays[delayIndex]
			logger.Info(ctx, "Applying exponential backoff delay",
				"attempt_no", nextAttemptNo,
				"delay", delay)
			
			select {
			case <-time.After(delay):
				// Delay completed
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	logger.Info(ctx, "Attempting delivery",
		"attempt_no", nextAttemptNo,
		"max_attempts", p.maxDeliveryAttempts)

	// Attempt delivery to Customer API
	response, deliveryErr := p.customerAPIClient.SendLead(ctx, lead.CustomerPayload)

	// Create delivery attempt record
	attempt := models.NewDeliveryAttempt(lead.ID, nextAttemptNo)

	// Start a transaction to atomically update lead status and create delivery attempt
	tx, err := p.leadRepo.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Handle the delivery response
	if deliveryErr != nil {
		// Check if the error is a DeliveryError with retriability information
		if delErr, ok := deliveryErr.(*models.DeliveryError); ok {
			logger.Info(ctx, "Delivery attempt failed",
				"attempt_no", nextAttemptNo,
				"error", delErr.Message,
				"retriable", delErr.Retriable,
				"status_code", delErr.StatusCode)

			// Record the failure in the delivery attempt
			statusCodePtr := delErr.StatusCode
			if statusCodePtr == 0 {
				attempt.MarkFailure(nil, delErr.Message)
			} else {
				attempt.MarkFailure(&statusCodePtr, delErr.Message)
			}

			if !delErr.Retriable {
				// Non-retriable error (4xx except 429) - mark as PERMANENTLY_FAILED
				logger.Info(ctx, "Non-retriable error encountered, marking as PERMANENTLY_FAILED")
				if err := p.leadRepo.UpdateLeadStatusTx(ctx, tx, lead.ID, models.LeadStatusPermanentlyFailed); err != nil {
					return fmt.Errorf("failed to update lead status to PERMANENTLY_FAILED: %w", err)
				}
				oldStatus := lead.Status
				lead.Status = models.LeadStatusPermanentlyFailed
				logger.LogStatusTransition(ctx, lead.ID, string(oldStatus), string(lead.Status))
			} else {
				// Retriable error (5xx, network error, 429)
				if nextAttemptNo >= p.maxDeliveryAttempts {
					// Max retries exhausted
					logger.Info(ctx, "Max retries exhausted, marking as PERMANENTLY_FAILED")
					if err := p.leadRepo.UpdateLeadStatusTx(ctx, tx, lead.ID, models.LeadStatusPermanentlyFailed); err != nil {
						return fmt.Errorf("failed to update lead status to PERMANENTLY_FAILED: %w", err)
					}
					oldStatus := lead.Status
					lead.Status = models.LeadStatusPermanentlyFailed
					logger.LogStatusTransition(ctx, lead.ID, string(oldStatus), string(lead.Status))
				} else {
					// Mark as FAILED for retry
					logger.Info(ctx, "Marking as FAILED for retry")
					if err := p.leadRepo.UpdateLeadStatusTx(ctx, tx, lead.ID, models.LeadStatusFailed); err != nil {
						return fmt.Errorf("failed to update lead status to FAILED: %w", err)
					}
					oldStatus := lead.Status
					lead.Status = models.LeadStatusFailed
					logger.LogStatusTransition(ctx, lead.ID, string(oldStatus), string(lead.Status))
				}
			}
		} else {
			// Unknown error type - treat as retriable
			logger.LogError(ctx, "Delivery attempt failed with unknown error", deliveryErr,
				"attempt_no", nextAttemptNo)
			errorMsg := deliveryErr.Error()
			attempt.MarkFailure(nil, errorMsg)

			if nextAttemptNo >= p.maxDeliveryAttempts {
				logger.Info(ctx, "Max retries exhausted, marking as PERMANENTLY_FAILED")
				if err := p.leadRepo.UpdateLeadStatusTx(ctx, tx, lead.ID, models.LeadStatusPermanentlyFailed); err != nil {
					return fmt.Errorf("failed to update lead status to PERMANENTLY_FAILED: %w", err)
				}
				oldStatus := lead.Status
				lead.Status = models.LeadStatusPermanentlyFailed
				logger.LogStatusTransition(ctx, lead.ID, string(oldStatus), string(lead.Status))
			} else {
				logger.Info(ctx, "Marking as FAILED for retry")
				if err := p.leadRepo.UpdateLeadStatusTx(ctx, tx, lead.ID, models.LeadStatusFailed); err != nil {
					return fmt.Errorf("failed to update lead status to FAILED: %w", err)
				}
				oldStatus := lead.Status
				lead.Status = models.LeadStatusFailed
				logger.LogStatusTransition(ctx, lead.ID, string(oldStatus), string(lead.Status))
			}
		}
	} else if response != nil && response.Success {
		// Successful delivery (2xx response)
		logger.Info(ctx, "Lead delivered successfully",
			"status_code", response.StatusCode)
		attempt.MarkSuccess(response.StatusCode, response.Body)

		// Mark lead as DELIVERED
		if err := p.leadRepo.UpdateLeadStatusTx(ctx, tx, lead.ID, models.LeadStatusDelivered); err != nil {
			return fmt.Errorf("failed to update lead status to DELIVERED: %w", err)
		}
		oldStatus := lead.Status
		lead.Status = models.LeadStatusDelivered
		logger.LogStatusTransition(ctx, lead.ID, string(oldStatus), string(lead.Status))
	} else {
		// Unexpected case - response is not nil but not successful
		logger.Warn(ctx, "Delivery attempt returned unexpected response",
			"attempt_no", nextAttemptNo,
			"response", response)
		errorMsg := fmt.Sprintf("unexpected response: %v", response)
		attempt.MarkFailure(nil, errorMsg)

		if nextAttemptNo >= p.maxDeliveryAttempts {
			logger.Info(ctx, "Max retries exhausted, marking as PERMANENTLY_FAILED")
			if err := p.leadRepo.UpdateLeadStatusTx(ctx, tx, lead.ID, models.LeadStatusPermanentlyFailed); err != nil {
				return fmt.Errorf("failed to update lead status to PERMANENTLY_FAILED: %w", err)
			}
			oldStatus := lead.Status
			lead.Status = models.LeadStatusPermanentlyFailed
			logger.LogStatusTransition(ctx, lead.ID, string(oldStatus), string(lead.Status))
		} else {
			logger.Info(ctx, "Marking as FAILED for retry")
			if err := p.leadRepo.UpdateLeadStatusTx(ctx, tx, lead.ID, models.LeadStatusFailed); err != nil {
				return fmt.Errorf("failed to update lead status to FAILED: %w", err)
			}
			oldStatus := lead.Status
			lead.Status = models.LeadStatusFailed
			logger.LogStatusTransition(ctx, lead.ID, string(oldStatus), string(lead.Status))
		}
	}

	// Create the delivery attempt record within the transaction
	if err := p.deliveryAttemptRepo.CreateDeliveryAttemptTx(ctx, tx, attempt); err != nil {
		return fmt.Errorf("failed to create delivery attempt: %w", err)
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	logger.Info(ctx, "Delivery stage completed", "final_status", lead.Status)
	return nil
}
