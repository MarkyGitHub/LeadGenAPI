package integration

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/checkfox/go_lead/internal/client"
	"github.com/checkfox/go_lead/internal/config"
	"github.com/checkfox/go_lead/internal/handlers"
	"github.com/checkfox/go_lead/internal/models"
	"github.com/checkfox/go_lead/internal/queue"
	"github.com/checkfox/go_lead/internal/repository"
	"github.com/checkfox/go_lead/internal/services"
	"github.com/checkfox/go_lead/internal/worker"
)

// TestDatabaseUnavailability tests webhook behavior when database is unavailable
// Requirements: 9.1
func TestDatabaseUnavailability(t *testing.T) {
	_, dbWrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Initialize components
	jobQueue, err := queue.NewDBQueue(dbWrapper.DB)
	if err != nil {
		t.Fatalf("Failed to initialize queue: %v", err)
	}
	defer jobQueue.Close()

	leadRepo := repository.NewLeadRepository(dbWrapper.DB)

	// Initialize webhook handler
	webhookHandler := handlers.NewWebhookHandler(leadRepo, jobQueue)

	// Step 1: Close the database connection to simulate unavailability
	dbWrapper.Close()

	// Step 2: Send webhook request
	leadPayload := map[string]interface{}{
		"email":   "test@example.com",
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

	// Step 3: Verify webhook returns 503 Service Unavailable
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503 Service Unavailable, got %d", rr.Code)
	}

	// Step 4: Verify response contains error information
	var response map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if _, ok := response["error"]; !ok {
		t.Error("Expected error field in response")
	}

	t.Logf("Database unavailability test passed - received 503 with error: %v", response["error"])
}

// TestQueueUnavailability tests webhook behavior when queue is unavailable
// Requirements: 5.6
func TestQueueUnavailability(t *testing.T) {
	cfg, dbWrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Initialize components
	leadRepo := repository.NewLeadRepository(dbWrapper.DB)

	// Step 1: Create a queue with a closed database connection to simulate unavailability
	// First create a separate database connection for the queue
	queueDB, err := sql.Open("postgres", buildConnectionString(cfg))
	if err != nil {
		t.Fatalf("Failed to open queue database connection: %v", err)
	}

	jobQueue, err := queue.NewDBQueue(queueDB)
	if err != nil {
		t.Fatalf("Failed to initialize queue: %v", err)
	}

	// Close the queue's database connection to simulate unavailability
	queueDB.Close()

	// Initialize webhook handler with the queue that has a closed connection
	webhookHandler := handlers.NewWebhookHandler(leadRepo, jobQueue)

	// Step 2: Send webhook request
	leadPayload := map[string]interface{}{
		"email":   "test@example.com",
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

	// Step 3: Verify webhook returns 503 Service Unavailable
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503 Service Unavailable, got %d", rr.Code)
	}

	// Step 4: Verify response contains error information
	var response map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if _, ok := response["error"]; !ok {
		t.Error("Expected error field in response")
	}

	t.Logf("Queue unavailability test passed - received 503 with error: %v", response["error"])
}

// TestCustomerAPIUnreachable tests retry behavior when Customer API is unreachable
// Requirements: 9.2
func TestCustomerAPIUnreachable(t *testing.T) {
	ctx := context.Background()
	cfg, dbWrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Step 1: Configure Customer API URL to an unreachable endpoint
	// Use a non-routable IP address to simulate network unreachability
	cfg.CustomerAPI.URL = "http://192.0.2.1:9999/api/leads"

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

	// Initialize worker processor with very short timeout and backoff delays
	validator := services.NewValidator()
	normalizer := services.NewNormalizer()
	mapper := services.NewMapper(cfg)
	
	// Use a very short timeout to speed up the test
	customerAPIClient := client.NewCustomerAPIClient(
		cfg.CustomerAPI.URL,
		cfg.CustomerAPI.Token,
		1*time.Second, // Short timeout for unreachable endpoint
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

	// Step 2: Send webhook request with valid lead
	leadPayload := map[string]interface{}{
		"email":   "unreachable@example.com",
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

	// Step 3: Process the job multiple times to trigger retries
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
			// First 4 attempts should result in FAILED status (retriable network error)
			if lead.Status != models.LeadStatusFailed {
				t.Errorf("After attempt %d, expected lead status FAILED, got %s", i+1, lead.Status)
			}

			// Re-enqueue the job for the next attempt
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

	// Step 4: Verify all delivery attempts were recorded
	attempts, err := deliveryAttemptRepo.GetDeliveryAttemptsByLeadID(ctx, leadID)
	if err != nil {
		t.Fatalf("Failed to get delivery attempts: %v", err)
	}

	t.Logf("Total delivery attempts recorded: %d", len(attempts))
	if len(attempts) != 5 {
		t.Errorf("Expected 5 delivery attempts recorded, got %d", len(attempts))
	}

	// Step 5: Verify each attempt has error information
	for i, attempt := range attempts {
		if attempt.ErrorMessage == nil {
			t.Errorf("Attempt %d has nil error message", i+1)
		} else {
			t.Logf("Attempt %d error message: %s", i+1, *attempt.ErrorMessage)
		}

		// Network errors typically don't have a status code
		if attempt.ResponseStatus != nil {
			t.Logf("Attempt %d response status: %d", i+1, *attempt.ResponseStatus)
		}
	}

	// Step 6: Verify final lead status is PERMANENTLY_FAILED
	lead, err := leadRepo.GetLeadByID(ctx, leadID)
	if err != nil {
		t.Fatalf("Failed to get final lead status: %v", err)
	}

	if lead.Status != models.LeadStatusPermanentlyFailed {
		t.Errorf("Expected final lead status PERMANENTLY_FAILED, got %s", lead.Status)
	}

	t.Logf("Customer API unreachability test passed - lead marked as PERMANENTLY_FAILED after %d attempts", len(attempts))
}

// buildConnectionString builds a PostgreSQL connection string from config
func buildConnectionString(cfg *config.Config) string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.DBName,
		cfg.Database.SSLMode,
	)
}
