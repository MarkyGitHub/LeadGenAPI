package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/checkfox/go_lead/internal/client"
	"github.com/checkfox/go_lead/internal/config"
	"github.com/checkfox/go_lead/internal/database"
	"github.com/checkfox/go_lead/internal/handlers"
	"github.com/checkfox/go_lead/internal/logger"
	"github.com/checkfox/go_lead/internal/models"
	"github.com/checkfox/go_lead/internal/queue"
	"github.com/checkfox/go_lead/internal/repository"
	"github.com/checkfox/go_lead/internal/services"
	"github.com/checkfox/go_lead/internal/worker"
)

// TestEndToEndLeadProcessing tests the full flow: webhook → validation → transformation → delivery
// Requirements: 1.1, 2.5, 3.7, 4.2, 5.2
func TestEndToEndLeadProcessing(t *testing.T) {
	// Setup test environment
	ctx := context.Background()
	cfg, dbWrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Create mock Customer API server that returns success
	mockCustomerAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST request, got %s", r.Method)
		}

		// Verify Authorization header
		expectedAuth := "Bearer " + cfg.CustomerAPI.Token
		if auth := r.Header.Get("Authorization"); auth != expectedAuth {
			t.Errorf("Expected Authorization %s, got %s", expectedAuth, auth)
		}

		// Parse request body
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("Failed to decode request body: %v", err)
		}

		// Verify required fields are present
		if _, ok := payload["phone"]; !ok {
			t.Error("Expected phone field in payload")
		}

		if product, ok := payload["product"]; !ok {
			t.Error("Expected product field in payload")
		} else {
			productMap, ok := product.(map[string]interface{})
			if !ok {
				t.Error("Expected product to be a map")
			} else if _, ok := productMap["name"]; !ok {
				t.Error("Expected product.name in payload")
			}
		}

		// Return success response
		w.WriteHeader(http.StatusOK)
		response := map[string]interface{}{
			"id":     "customer-lead-123",
			"status": "accepted",
		}
		json.NewEncoder(w).Encode(response)
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

	// Initialize worker processor
	validator := services.NewValidator()
	normalizer := services.NewNormalizer()
	mapper := services.NewMapper(cfg)
	customerAPIClient := client.NewCustomerAPIClient(
		cfg.CustomerAPI.URL,
		cfg.CustomerAPI.Token,
		30*time.Second,
	)

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
		ExponentialBackoffDelays: []time.Duration{30 * time.Second, 60 * time.Second, 120 * time.Second, 240 * time.Second, 480 * time.Second},
	})

	// Step 1: Send webhook request
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

	// Verify webhook response
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", rr.Code)
	}

	var webhookResponse handlers.WebhookResponse
	if err := json.NewDecoder(rr.Body).Decode(&webhookResponse); err != nil {
		t.Fatalf("Failed to decode webhook response: %v", err)
	}

	if webhookResponse.Status != "RECEIVED" {
		t.Errorf("Expected status RECEIVED, got %s", webhookResponse.Status)
	}

	leadID := webhookResponse.LeadID

	// Step 2: Verify lead was stored in database
	lead, err := leadRepo.GetLeadByID(ctx, leadID)
	if err != nil {
		t.Fatalf("Failed to get lead from database: %v", err)
	}

	if lead.Status != models.LeadStatusReceived {
		t.Errorf("Expected lead status RECEIVED, got %s", lead.Status)
	}

	// Step 3: Process the job (simulating worker)
	// Force the job's next_run_at to be in the past to ensure it can be dequeued
	_, err = dbWrapper.DB.ExecContext(ctx, "UPDATE background_jobs SET next_run_at = NOW() - INTERVAL '1 second' WHERE status = 'pending'")
	if err != nil {
		t.Fatalf("Failed to update job next_run_at: %v", err)
	}
	
	job, err := jobQueue.Dequeue(ctx)
	if err != nil {
		t.Fatalf("Failed to dequeue job: %v", err)
	}

	if job == nil {
		t.Fatal("Expected job to be enqueued, got nil")
	}

	// Process the lead through validation, transformation, and delivery
	err = processor.ProcessJobForTest(ctx, job)
	if err != nil {
		t.Fatalf("Failed to process job: %v", err)
	}

	// Step 4: Verify lead status transitions
	lead, err = leadRepo.GetLeadByID(ctx, leadID)
	if err != nil {
		t.Fatalf("Failed to get updated lead: %v", err)
	}

	// Lead should be DELIVERED after successful processing
	if lead.Status != models.LeadStatusDelivered {
		t.Errorf("Expected lead status DELIVERED, got %s", lead.Status)
	}

	// Step 5: Verify normalized and customer payloads are set
	if lead.NormalizedPayload == nil {
		t.Error("Expected normalized payload to be set")
	}

	if lead.CustomerPayload == nil {
		t.Error("Expected customer payload to be set")
	}

	// Step 6: Verify delivery attempt was recorded
	attempts, err := deliveryAttemptRepo.GetDeliveryAttemptsByLeadID(ctx, leadID)
	if err != nil {
		t.Fatalf("Failed to get delivery attempts: %v", err)
	}

	if len(attempts) == 0 {
		t.Fatal("Expected at least one delivery attempt to be recorded")
	}

	attempt := attempts[0]
	if attempt.ResponseStatus == nil || *attempt.ResponseStatus != http.StatusOK {
		t.Errorf("Expected delivery attempt status code 200, got %v", attempt.ResponseStatus)
	}

	if attempt.ResponseBody == nil {
		t.Error("Expected response body to be stored")
	}

	// Step 7: Verify response body contains expected data
	var responseBody map[string]interface{}
	if err := json.Unmarshal([]byte(*attempt.ResponseBody), &responseBody); err != nil {
		t.Fatalf("Failed to parse response body: %v", err)
	}

	if responseBody["id"] != "customer-lead-123" {
		t.Errorf("Expected response id 'customer-lead-123', got %v", responseBody["id"])
	}

	if responseBody["status"] != "accepted" {
		t.Errorf("Expected response status 'accepted', got %v", responseBody["status"])
	}
}

// TestEndToEndLeadRejection tests the full flow with validation failure
// Requirements: 1.1, 2.5
func TestEndToEndLeadRejection(t *testing.T) {
	// Setup test environment
	ctx := context.Background()
	cfg, dbWrapper, cleanup := setupTestEnvironment(t)
	defer cleanup()

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

	// Initialize worker processor
	validator := services.NewValidator()
	normalizer := services.NewNormalizer()
	mapper := services.NewMapper(cfg)
	customerAPIClient := client.NewCustomerAPIClient(
		cfg.CustomerAPI.URL,
		cfg.CustomerAPI.Token,
		30*time.Second,
	)

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
		ExponentialBackoffDelays: []time.Duration{30 * time.Second, 60 * time.Second, 120 * time.Second, 240 * time.Second, 480 * time.Second},
	})

	// Step 1: Send webhook request with invalid zipcode
	leadPayload := map[string]interface{}{
		"email":   "test@example.com",
		"phone":   "1234567890",
		"zipcode": "12345", // Invalid zipcode (not 66XXX)
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

	// Step 2: Process the job
	// Force the job's next_run_at to be in the past to ensure it can be dequeued
	_, err = dbWrapper.DB.ExecContext(ctx, "UPDATE background_jobs SET next_run_at = NOW() - INTERVAL '1 second' WHERE status = 'pending'")
	if err != nil {
		t.Fatalf("Failed to update job next_run_at: %v", err)
	}
	
	job, err := jobQueue.Dequeue(ctx)
	if err != nil {
		t.Fatalf("Failed to dequeue job: %v", err)
	}

	if job == nil {
		t.Fatal("Expected job to be enqueued, got nil")
	}

	err = processor.ProcessJobForTest(ctx, job)
	if err != nil {
		t.Fatalf("Failed to process job: %v", err)
	}

	// Step 3: Verify lead was rejected
	lead, err := leadRepo.GetLeadByID(ctx, leadID)
	if err != nil {
		t.Fatalf("Failed to get updated lead: %v", err)
	}

	if lead.Status != models.LeadStatusRejected {
		t.Errorf("Expected lead status REJECTED, got %s", lead.Status)
	}

	// Step 4: Verify rejection reason is set
	if lead.RejectionReason == nil {
		t.Fatal("Expected rejection reason to be set")
	}

	if *lead.RejectionReason != string(models.RejectionReasonZipNotValid) {
		t.Errorf("Expected rejection reason ZIP_NOT_66XXX, got %s", *lead.RejectionReason)
	}

	// Step 5: Verify no delivery attempts were made
	attempts, err := deliveryAttemptRepo.GetDeliveryAttemptsByLeadID(ctx, leadID)
	if err != nil {
		t.Fatalf("Failed to get delivery attempts: %v", err)
	}

	if len(attempts) != 0 {
		t.Errorf("Expected no delivery attempts for rejected lead, got %d", len(attempts))
	}
}

// setupTestEnvironment initializes test configuration and database
func setupTestEnvironment(t *testing.T) (*config.Config, *database.DB, func()) {
	// Initialize logger for tests
	logger.Init()
	
	// Load test configuration
	cfg, err := config.Load()
	if err != nil {
		t.Skipf("Skipping test - failed to load config: %v", err)
		return nil, nil, nil
	}

	// Connect to test database
	dbWrapper, err := database.InitFromConfig(cfg)
	if err != nil {
		t.Skipf("Skipping test - failed to connect to database: %v", err)
		return nil, nil, nil
	}

	// Verify database connection
	if err := dbWrapper.DB.Ping(); err != nil {
		dbWrapper.Close()
		t.Skipf("Skipping test - database not available: %v", err)
		return nil, nil, nil
	}

	// Run migrations
	if err := database.RunMigrations(dbWrapper, "../../migrations"); err != nil {
		dbWrapper.Close()
		t.Skipf("Skipping test - failed to run migrations: %v", err)
		return nil, nil, nil
	}

	cleanup := func() {
		// Clean up test data
		ctx := context.Background()
		dbWrapper.DB.ExecContext(ctx, "DELETE FROM delivery_attempts")
		dbWrapper.DB.ExecContext(ctx, "DELETE FROM inbound_leads")
		dbWrapper.DB.ExecContext(ctx, "DELETE FROM background_jobs")
		dbWrapper.Close()
	}

	return cfg, dbWrapper, cleanup
}
