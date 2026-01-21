package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/checkfox/go_lead/internal/client"
	"github.com/checkfox/go_lead/internal/handlers"
	"github.com/checkfox/go_lead/internal/models"
	"github.com/checkfox/go_lead/internal/queue"
	"github.com/checkfox/go_lead/internal/repository"
	"github.com/checkfox/go_lead/internal/services"
	"github.com/checkfox/go_lead/internal/worker"
)

// TestRetryLogicWith5xxErrors tests retry behavior with failing Customer API
// Requirements: 4.4, 4.6, 5.3, 5.4
func TestRetryLogicWith5xxErrors(t *testing.T) {
	// Setup test environment
	ctx := context.Background()
	cfg, dbWrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Track number of delivery attempts and timing
	var attemptCount int32
	attemptTimes := make([]time.Time, 0, 5)
	var timeMutex sync.Mutex

	// Create mock Customer API server that returns 503 Service Unavailable
	mockCustomerAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&attemptCount, 1)
		
		// Record attempt time
		timeMutex.Lock()
		attemptTimes = append(attemptTimes, time.Now())
		timeMutex.Unlock()
		
		t.Logf("Mock API received attempt %d", count)
		
		// Always return 503 to trigger retries
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"error": "service temporarily unavailable"}`))
	}))
	defer mockCustomerAPI.Close()

	// Update config to use mock Customer API
	cfg.CustomerAPI.URL = mockCustomerAPI.URL

	// Initialize components
	jobQueue, err := queue.NewDBQueue(dbWrapper.DB)
	if err != nil {
		t.Fatalf("Failed to initialize queue: %v", err)
	}
	defer jobQueue.Close()

	leadRepo := repository.NewLeadRepository(dbWrapper.DB)
	deliveryAttemptRepo := repository.NewDeliveryAttemptRepository(dbWrapper.DB)

	// Initialize webhook handler
	webhookHandler := handlers.NewWebhookHandler(leadRepo, jobQueue)

	// Initialize worker processor with very short backoff delays for testing
	// Use minimal delays to speed up the test: 10ms, 20ms, 40ms, 80ms, 160ms
	validator := services.NewValidator()
	normalizer := services.NewNormalizer()
	mapper := services.NewMapper(cfg)
	customerAPIClient := client.NewCustomerAPIClient(
		cfg.CustomerAPI.URL,
		cfg.CustomerAPI.Token,
		30*time.Second,
	)

	shortBackoffDelays := []time.Duration{
		10 * time.Millisecond,
		20 * time.Millisecond,
		40 * time.Millisecond,
		80 * time.Millisecond,
		160 * time.Millisecond,
	}

	processor := worker.NewProcessor(worker.ProcessorConfig{
		Queue:                    jobQueue,
		LeadRepo:                 leadRepo,
		DeliveryAttemptRepo:      deliveryAttemptRepo,
		Validator:                validator,
		Normalizer:               normalizer,
		Mapper:                   mapper,
		CustomerAPIClient:        customerAPIClient,
		PollInterval:             100 * time.Millisecond,
		MaxDeliveryAttempts:      5,
		ExponentialBackoffDelays: shortBackoffDelays,
	})

	// Step 1: Send webhook request with valid lead
	leadPayload := map[string]interface{}{
		"email":   "retry@example.com",
		"phone":   "1234567890",
		"zipcode": "66123",
		"house": map[string]interface{}{
			"is_owner": true,
		},
	}

	payloadBytes, _ := json.Marshal(leadPayload)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/leads", bytes.NewReader(payloadBytes))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	webhookHandler.HandleLeadWebhook(rr, req)

	// Verify webhook response
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", rr.Code)
	}

	var webhookResponse handlers.WebhookResponse
	if err := json.NewDecoder(rr.Body).Decode(&webhookResponse); err != nil {
		t.Fatalf("Failed to decode webhook response: %v", err)
	}

	leadID := webhookResponse.LeadID
	t.Logf("Created lead with ID: %d", leadID)

	// Step 2: Process the job multiple times to trigger retries
	// Force the job's next_run_at to be in the past
	_, err = dbWrapper.DB.ExecContext(ctx, "UPDATE background_jobs SET next_run_at = NOW() - INTERVAL '1 second' WHERE status = 'pending'")
	if err != nil {
		t.Fatalf("Failed to update job next_run_at: %v", err)
	}

	// Process the job 5 times (max attempts)
	for i := 0; i < 5; i++ {
		t.Logf("Processing attempt %d", i+1)
		
		// Dequeue and process the job
		job, err := jobQueue.Dequeue(ctx)
		if err != nil {
			t.Fatalf("Failed to dequeue job on attempt %d: %v", i+1, err)
		}

		if job == nil {
			t.Fatalf("Expected job to be available on attempt %d, got nil", i+1)
		}

		// Process the job
		err = processor.ProcessJobForTest(ctx, job)
		if err != nil {
			t.Logf("Job processing returned error on attempt %d: %v", i+1, err)
		}

		// After processing, verify lead status
		lead, err := leadRepo.GetLeadByID(ctx, leadID)
		if err != nil {
			t.Fatalf("Failed to get lead after attempt %d: %v", i+1, err)
		}

		t.Logf("After attempt %d, lead status: %s", i+1, lead.Status)

		if i < 4 {
			// First 4 attempts should result in FAILED status
			if lead.Status != models.LeadStatusFailed {
				t.Errorf("After attempt %d, expected lead status FAILED, got %s", i+1, lead.Status)
			}
			
			// Re-enqueue the job for the next attempt
			// Mark the job as pending again and reset next_run_at
			_, err = dbWrapper.DB.ExecContext(ctx, 
				"UPDATE background_jobs SET status = 'pending', next_run_at = NOW() - INTERVAL '1 second', attempts = 0 WHERE id = $1",
				job.ID)
			if err != nil {
				t.Fatalf("Failed to reset job for retry %d: %v", i+1, err)
			}
		} else {
			// 5th attempt should result in PERMANENTLY_FAILED status
			if lead.Status != models.LeadStatusPermanentlyFailed {
				t.Errorf("After attempt %d, expected lead status PERMANENTLY_FAILED, got %s", i+1, lead.Status)
			}
		}
	}

	// Step 3: Verify the number of API calls matches max attempts
	finalAttemptCount := atomic.LoadInt32(&attemptCount)
	t.Logf("Total API attempts: %d", finalAttemptCount)
	if finalAttemptCount != 5 {
		t.Errorf("Expected 5 delivery attempts, got %d", finalAttemptCount)
	}

	// Step 4: Verify exponential backoff timing
	// Check that delays between attempts follow the exponential backoff pattern
	timeMutex.Lock()
	defer timeMutex.Unlock()

	if len(attemptTimes) != 5 {
		t.Fatalf("Expected 5 attempt times recorded, got %d", len(attemptTimes))
	}

	// Verify delays between attempts (with some tolerance for timing variations)
	expectedDelays := shortBackoffDelays
	tolerance := 50 * time.Millisecond // Allow 50ms tolerance

	for i := 1; i < len(attemptTimes); i++ {
		actualDelay := attemptTimes[i].Sub(attemptTimes[i-1])
		expectedDelay := expectedDelays[i-1]

		t.Logf("Delay between attempt %d and %d: expected ~%v, got %v", i, i+1, expectedDelay, actualDelay)

		// Check if actual delay is within tolerance of expected delay
		// Allow for some variance due to processing time
		minDelay := expectedDelay - tolerance
		if actualDelay < minDelay {
			t.Errorf("Delay between attempt %d and %d was too short: expected >=%v, got %v",
				i, i+1, minDelay, actualDelay)
		}
		
		// Allow some extra time for processing, but not too much
		maxDelay := expectedDelay + 2*time.Second
		if actualDelay > maxDelay {
			t.Errorf("Delay between attempt %d and %d was too long: expected <=%v, got %v",
				i, i+1, maxDelay, actualDelay)
		}
	}

	// Step 5: Verify permanent failure after max retries
	lead, err := leadRepo.GetLeadByID(ctx, leadID)
	if err != nil {
		t.Fatalf("Failed to get final lead status: %v", err)
	}

	if lead.Status != models.LeadStatusPermanentlyFailed {
		t.Errorf("Expected final lead status PERMANENTLY_FAILED, got %s", lead.Status)
	}

	// Step 6: Verify all delivery attempts were recorded
	attempts, err := deliveryAttemptRepo.GetDeliveryAttemptsByLeadID(ctx, leadID)
	if err != nil {
		t.Fatalf("Failed to get delivery attempts: %v", err)
	}

	t.Logf("Total delivery attempts recorded: %d", len(attempts))
	if len(attempts) != 5 {
		t.Errorf("Expected 5 delivery attempts recorded, got %d", len(attempts))
	}

	// Verify each attempt has the correct attempt number and status code
	for i, attempt := range attempts {
		expectedAttemptNo := i + 1
		if attempt.AttemptNo != expectedAttemptNo {
			t.Errorf("Attempt %d has wrong attempt_no: expected %d, got %d",
				i, expectedAttemptNo, attempt.AttemptNo)
		}

		if attempt.ResponseStatus == nil {
			t.Errorf("Attempt %d has nil response status", i)
		} else if *attempt.ResponseStatus != http.StatusServiceUnavailable {
			t.Errorf("Attempt %d has wrong status code: expected %d, got %d",
				i, http.StatusServiceUnavailable, *attempt.ResponseStatus)
		}

		if attempt.ErrorMessage == nil {
			t.Errorf("Attempt %d has nil error message", i)
		} else {
			t.Logf("Attempt %d error message: %s", i+1, *attempt.ErrorMessage)
		}
	}
}