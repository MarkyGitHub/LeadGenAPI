package worker

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/checkfox/go_lead/internal/client"
	"github.com/checkfox/go_lead/internal/config"
	"github.com/checkfox/go_lead/internal/database"
	"github.com/checkfox/go_lead/internal/models"
	"github.com/checkfox/go_lead/internal/queue"
	"github.com/checkfox/go_lead/internal/repository"
	"github.com/checkfox/go_lead/internal/services"
)

// setupTestProcessor creates a processor with test dependencies
func setupTestProcessor(t *testing.T) (*Processor, func()) {
	ensureTestDBEnv()

	// Load test configuration
	cfg, err := config.Load()
	if err != nil {
		t.Skipf("Skipping test - failed to load config: %v", err)
		return nil, nil
	}

	// Connect to test database
	dbWrapper, err := database.InitFromConfig(cfg)
	if err != nil {
		t.Skipf("Skipping test - failed to connect to database: %v", err)
		return nil, nil
	}
	db := dbWrapper.DB

	// Verify database connection
	if err := db.Ping(); err != nil {
		dbWrapper.Close()
		t.Skipf("Skipping test - database not available: %v", err)
		return nil, nil
	}

	// Initialize queue
	jobQueue, err := queue.NewDBQueue(db)
	if err != nil {
		t.Fatalf("Failed to initialize queue: %v", err)
	}

	// Initialize repositories
	leadRepo := repository.NewLeadRepository(db)
	deliveryAttemptRepo := repository.NewDeliveryAttemptRepository(db)

	// Initialize services
	validator := services.NewValidator()
	normalizer := services.NewNormalizer()
	mapper := services.NewMapper(cfg)

	// Initialize Customer API client (will not be used in these tests)
	customerAPIClient := client.NewCustomerAPIClient(
		cfg.CustomerAPI.URL,
		cfg.CustomerAPI.Token,
		30*time.Second,
	)

	// Create processor
	processor := NewProcessor(ProcessorConfig{
		Queue:               jobQueue,
		LeadRepo:            leadRepo,
		DeliveryAttemptRepo: deliveryAttemptRepo,
		Validator:           validator,
		Normalizer:          normalizer,
		Mapper:              mapper,
		CustomerAPIClient:   customerAPIClient,
		PollInterval:        100 * time.Millisecond,
	})

	cleanup := func() {
		jobQueue.Close()
		dbWrapper.Close()
	}

	return processor, cleanup
}

func ensureTestDBEnv() {
	if _, ok := os.LookupEnv("DB_PORT"); ok {
		return
	}
	if testPort, ok := os.LookupEnv("TEST_DB_PORT"); ok {
		_ = os.Setenv("DB_PORT", testPort)
	}
}

// TestProcessLead_SuccessfulProcessing tests the successful processing flow
// Requirements: 5.2, 5.5
func TestProcessLead_SuccessfulProcessing(t *testing.T) {
	processor, cleanup := setupTestProcessor(t)
	defer cleanup()

	ctx := context.Background()

	// Create a valid lead
	lead := &models.InboundLead{
		RawPayload: models.JSONB{
			"email":   "test@example.com",
			"phone":   "1234567890",
			"zipcode": "66123",
			"house": map[string]interface{}{
				"is_owner": true,
			},
		},
		SourceHeaders: models.JSONB{},
		Status:        models.LeadStatusReceived,
	}

	// Insert lead into database
	err := processor.leadRepo.CreateLead(ctx, lead)
	if err != nil {
		t.Fatalf("Failed to create lead: %v", err)
	}

	// Create a job for the lead
	job := &queue.Job{
		ID:   1,
		Type: "process_lead",
		Payload: map[string]interface{}{
			"lead_id": float64(lead.ID),
		},
	}

	// Process the lead
	err = processor.processLead(ctx, job)
	if err != nil {
		t.Fatalf("Failed to process lead: %v", err)
	}

	// Verify lead status is READY (validation and transformation passed)
	updatedLead, err := processor.leadRepo.GetLeadByID(ctx, lead.ID)
	if err != nil {
		t.Fatalf("Failed to get updated lead: %v", err)
	}

	if updatedLead.Status != models.LeadStatusReady {
		t.Errorf("Expected lead status to be READY, got %s", updatedLead.Status)
	}

	// Verify normalized and customer payloads are set
	if updatedLead.NormalizedPayload == nil {
		t.Error("Expected normalized payload to be set")
	}

	if updatedLead.CustomerPayload == nil {
		t.Error("Expected customer payload to be set")
	}

	// Verify customer payload has required fields
	if phone, ok := updatedLead.CustomerPayload["phone"]; !ok || phone == nil {
		t.Error("Expected customer payload to have phone field")
	}

	if product, ok := updatedLead.CustomerPayload["product"]; !ok || product == nil {
		t.Error("Expected customer payload to have product field")
	}
}

// TestProcessLead_ValidationFailure tests the validation failure path
// Requirements: 5.2, 5.5
func TestProcessLead_ValidationFailure(t *testing.T) {
	processor, cleanup := setupTestProcessor(t)
	defer cleanup()

	ctx := context.Background()

	// Create a lead with invalid zipcode
	lead := &models.InboundLead{
		RawPayload: models.JSONB{
			"email":   "test@example.com",
			"phone":   "1234567890",
			"zipcode": "12345", // Invalid zipcode (not 66XXX)
			"house": map[string]interface{}{
				"is_owner": true,
			},
		},
		SourceHeaders: models.JSONB{},
		Status:        models.LeadStatusReceived,
	}

	// Insert lead into database
	err := processor.leadRepo.CreateLead(ctx, lead)
	if err != nil {
		t.Fatalf("Failed to create lead: %v", err)
	}

	// Create a job for the lead
	job := &queue.Job{
		ID:   1,
		Type: "process_lead",
		Payload: map[string]interface{}{
			"lead_id": float64(lead.ID),
		},
	}

	// Process the lead
	err = processor.processLead(ctx, job)
	if err != nil {
		t.Fatalf("Failed to process lead: %v", err)
	}

	// Verify lead status is REJECTED
	updatedLead, err := processor.leadRepo.GetLeadByID(ctx, lead.ID)
	if err != nil {
		t.Fatalf("Failed to get updated lead: %v", err)
	}

	if updatedLead.Status != models.LeadStatusRejected {
		t.Errorf("Expected lead status to be REJECTED, got %s", updatedLead.Status)
	}

	// Verify rejection reason is set
	if updatedLead.RejectionReason == nil {
		t.Error("Expected rejection reason to be set")
	} else if *updatedLead.RejectionReason != string(models.RejectionReasonZipNotValid) {
		t.Errorf("Expected rejection reason to be ZIP_NOT_66XXX, got %s", *updatedLead.RejectionReason)
	}
}

// TestProcessLead_TransformationFailure tests the transformation failure path
// Requirements: 5.2, 5.5
func TestProcessLead_TransformationFailure(t *testing.T) {
	processor, cleanup := setupTestProcessor(t)
	defer cleanup()

	ctx := context.Background()

	// Create a lead with missing required field (phone)
	lead := &models.InboundLead{
		RawPayload: models.JSONB{
			"email":   "test@example.com",
			// Missing phone field
			"zipcode": "66123",
			"house": map[string]interface{}{
				"is_owner": true,
			},
		},
		SourceHeaders: models.JSONB{},
		Status:        models.LeadStatusReceived,
	}

	// Insert lead into database
	err := processor.leadRepo.CreateLead(ctx, lead)
	if err != nil {
		t.Fatalf("Failed to create lead: %v", err)
	}

	// Create a job for the lead
	job := &queue.Job{
		ID:   1,
		Type: "process_lead",
		Payload: map[string]interface{}{
			"lead_id": float64(lead.ID),
		},
	}

	// Process the lead
	err = processor.processLead(ctx, job)
	if err != nil {
		t.Fatalf("Failed to process lead: %v", err)
	}

	// Verify lead status is PERMANENTLY_FAILED (missing core field)
	updatedLead, err := processor.leadRepo.GetLeadByID(ctx, lead.ID)
	if err != nil {
		t.Fatalf("Failed to get updated lead: %v", err)
	}

	if updatedLead.Status != models.LeadStatusPermanentlyFailed {
		t.Errorf("Expected lead status to be PERMANENTLY_FAILED, got %s", updatedLead.Status)
	}
}

// TestProcessLead_StatusTransitions tests status transitions during processing
// Requirements: 5.2, 5.5
func TestProcessLead_StatusTransitions(t *testing.T) {
	processor, cleanup := setupTestProcessor(t)
	defer cleanup()

	ctx := context.Background()

	// Create a valid lead
	lead := &models.InboundLead{
		RawPayload: models.JSONB{
			"email":   "test@example.com",
			"phone":   "1234567890",
			"zipcode": "66123",
			"house": map[string]interface{}{
				"is_owner": true,
			},
		},
		SourceHeaders: models.JSONB{},
		Status:        models.LeadStatusReceived,
	}

	// Insert lead into database
	err := processor.leadRepo.CreateLead(ctx, lead)
	if err != nil {
		t.Fatalf("Failed to create lead: %v", err)
	}

	initialStatus := lead.Status

	// Create a job for the lead
	job := &queue.Job{
		ID:   1,
		Type: "process_lead",
		Payload: map[string]interface{}{
			"lead_id": float64(lead.ID),
		},
	}

	// Process the lead
	err = processor.processLead(ctx, job)
	if err != nil {
		t.Fatalf("Failed to process lead: %v", err)
	}

	// Verify status transitioned from RECEIVED to READY
	updatedLead, err := processor.leadRepo.GetLeadByID(ctx, lead.ID)
	if err != nil {
		t.Fatalf("Failed to get updated lead: %v", err)
	}

	if initialStatus != models.LeadStatusReceived {
		t.Errorf("Expected initial status to be RECEIVED, got %s", initialStatus)
	}

	if updatedLead.Status != models.LeadStatusReady {
		t.Errorf("Expected final status to be READY, got %s", updatedLead.Status)
	}

	// Verify updated_at timestamp changed
	if !updatedLead.UpdatedAt.After(updatedLead.CreatedAt) {
		t.Error("Expected updated_at to be after created_at")
	}
}

// TestExecuteValidationStage_ValidLead tests validation stage with valid lead
func TestExecuteValidationStage_ValidLead(t *testing.T) {
	processor, cleanup := setupTestProcessor(t)
	defer cleanup()

	ctx := context.Background()

	// Create a valid lead
	lead := &models.InboundLead{
		RawPayload: models.JSONB{
			"zipcode": "66123",
			"house": map[string]interface{}{
				"is_owner": true,
			},
		},
		Status: models.LeadStatusReceived,
	}

	err := processor.leadRepo.CreateLead(ctx, lead)
	if err != nil {
		t.Fatalf("Failed to create lead: %v", err)
	}

	// Execute validation stage
	err = processor.executeValidationStage(ctx, lead)
	if err != nil {
		t.Fatalf("Validation stage failed: %v", err)
	}

	// Verify status is READY
	if lead.Status != models.LeadStatusReady {
		t.Errorf("Expected status to be READY, got %s", lead.Status)
	}

	// Verify no rejection reason
	if lead.RejectionReason != nil {
		t.Errorf("Expected no rejection reason, got %s", *lead.RejectionReason)
	}
}

// TestExecuteValidationStage_InvalidZipcode tests validation stage with invalid zipcode
func TestExecuteValidationStage_InvalidZipcode(t *testing.T) {
	processor, cleanup := setupTestProcessor(t)
	defer cleanup()

	ctx := context.Background()

	// Create a lead with invalid zipcode
	lead := &models.InboundLead{
		RawPayload: models.JSONB{
			"zipcode": "12345",
			"house": map[string]interface{}{
				"is_owner": true,
			},
		},
		Status: models.LeadStatusReceived,
	}

	err := processor.leadRepo.CreateLead(ctx, lead)
	if err != nil {
		t.Fatalf("Failed to create lead: %v", err)
	}

	// Execute validation stage
	err = processor.executeValidationStage(ctx, lead)
	if err != nil {
		t.Fatalf("Validation stage failed: %v", err)
	}

	// Verify status is REJECTED
	if lead.Status != models.LeadStatusRejected {
		t.Errorf("Expected status to be REJECTED, got %s", lead.Status)
	}

	// Verify rejection reason
	if lead.RejectionReason == nil {
		t.Error("Expected rejection reason to be set")
	} else if *lead.RejectionReason != string(models.RejectionReasonZipNotValid) {
		t.Errorf("Expected rejection reason ZIP_NOT_66XXX, got %s", *lead.RejectionReason)
	}
}

// TestExecuteTransformationStage_Success tests transformation stage with valid data
func TestExecuteTransformationStage_Success(t *testing.T) {
	processor, cleanup := setupTestProcessor(t)
	defer cleanup()

	ctx := context.Background()

	// Create a lead with valid data
	lead := &models.InboundLead{
		RawPayload: models.JSONB{
			"email": "TEST@EXAMPLE.COM",
			"phone": "123-456-7890",
		},
		Status: models.LeadStatusReady,
	}

	err := processor.leadRepo.CreateLead(ctx, lead)
	if err != nil {
		t.Fatalf("Failed to create lead: %v", err)
	}

	// Execute transformation stage
	err = processor.executeTransformationStage(ctx, lead)
	if err != nil {
		t.Fatalf("Transformation stage failed: %v", err)
	}

	// Verify normalized payload is set
	if lead.NormalizedPayload == nil {
		t.Fatal("Expected normalized payload to be set")
	}

	// Verify email is normalized (lowercase)
	if email, ok := lead.NormalizedPayload["email"]; !ok {
		t.Error("Expected email in normalized payload")
	} else if email != "test@example.com" {
		t.Errorf("Expected normalized email to be lowercase, got %v", email)
	}

	// Verify phone is normalized (digits only)
	if phone, ok := lead.NormalizedPayload["phone"]; !ok {
		t.Error("Expected phone in normalized payload")
	} else if phone != "1234567890" {
		t.Errorf("Expected normalized phone to be digits only, got %v", phone)
	}

	// Verify customer payload is set
	if lead.CustomerPayload == nil {
		t.Fatal("Expected customer payload to be set")
	}

	// Verify required fields in customer payload
	if _, ok := lead.CustomerPayload["phone"]; !ok {
		t.Error("Expected phone in customer payload")
	}

	if product, ok := lead.CustomerPayload["product"]; !ok {
		t.Error("Expected product in customer payload")
	} else {
		productMap, ok := product.(map[string]interface{})
		if !ok {
			t.Error("Expected product to be a map")
		} else if _, ok := productMap["name"]; !ok {
			t.Error("Expected product.name in customer payload")
		}
	}
}

// TestExecuteTransformationStage_MissingCoreField tests transformation with missing core field
func TestExecuteTransformationStage_MissingCoreField(t *testing.T) {
	processor, cleanup := setupTestProcessor(t)
	defer cleanup()

	ctx := context.Background()

	// Create a lead without phone (required core field)
	lead := &models.InboundLead{
		RawPayload: models.JSONB{
			"email": "test@example.com",
			// Missing phone
		},
		Status: models.LeadStatusReady,
	}

	err := processor.leadRepo.CreateLead(ctx, lead)
	if err != nil {
		t.Fatalf("Failed to create lead: %v", err)
	}

	// Execute transformation stage
	err = processor.executeTransformationStage(ctx, lead)
	if err != nil {
		t.Fatalf("Transformation stage failed: %v", err)
	}

	// Verify status is PERMANENTLY_FAILED
	if lead.Status != models.LeadStatusPermanentlyFailed {
		t.Errorf("Expected status to be PERMANENTLY_FAILED, got %s", lead.Status)
	}
}

// TestExecuteDeliveryStage_SuccessfulDelivery tests successful delivery (2xx response)
// Requirements: 4.2, 4.5, 4.7
func TestExecuteDeliveryStage_SuccessfulDelivery(t *testing.T) {
	processor, cleanup := setupTestProcessor(t)
	if processor == nil {
		return // Test was skipped
	}
	defer cleanup()

	ctx := context.Background()

	// Create a lead in READY status with customer payload
	lead := &models.InboundLead{
		RawPayload: models.JSONB{
			"email":   "test@example.com",
			"phone":   "1234567890",
			"zipcode": "66123",
		},
		Status: models.LeadStatusReady,
		CustomerPayload: models.JSONB{
			"phone": "1234567890",
			"product": map[string]interface{}{
				"name": "test-product",
			},
		},
	}

	err := processor.leadRepo.CreateLead(ctx, lead)
	if err != nil {
		t.Fatalf("Failed to create lead: %v", err)
	}

	// Note: This test requires a mock Customer API server
	// For now, we'll skip the actual delivery test
	t.Skip("Skipping delivery test - requires mock Customer API server")
}

// TestExecuteDeliveryStage_RetryOn5xx tests retry behavior on 5xx errors
// Requirements: 4.3, 4.4, 5.3
func TestExecuteDeliveryStage_RetryOn5xx(t *testing.T) {
	processor, cleanup := setupTestProcessor(t)
	if processor == nil {
		return // Test was skipped
	}
	defer cleanup()

	ctx := context.Background()

	// Create a lead in READY status
	lead := &models.InboundLead{
		RawPayload: models.JSONB{
			"email":   "test@example.com",
			"phone":   "1234567890",
			"zipcode": "66123",
		},
		Status: models.LeadStatusReady,
		CustomerPayload: models.JSONB{
			"phone": "1234567890",
			"product": map[string]interface{}{
				"name": "test-product",
			},
		},
	}

	err := processor.leadRepo.CreateLead(ctx, lead)
	if err != nil {
		t.Fatalf("Failed to create lead: %v", err)
	}

	// Note: This test requires a mock Customer API server that returns 5xx
	t.Skip("Skipping 5xx retry test - requires mock Customer API server")
}

// TestExecuteDeliveryStage_NoRetryOn4xx tests no retry on 4xx errors
// Requirements: 4.3
func TestExecuteDeliveryStage_NoRetryOn4xx(t *testing.T) {
	processor, cleanup := setupTestProcessor(t)
	if processor == nil {
		return // Test was skipped
	}
	defer cleanup()

	ctx := context.Background()

	// Create a lead in READY status
	lead := &models.InboundLead{
		RawPayload: models.JSONB{
			"email":   "test@example.com",
			"phone":   "1234567890",
			"zipcode": "66123",
		},
		Status: models.LeadStatusReady,
		CustomerPayload: models.JSONB{
			"phone": "1234567890",
			"product": map[string]interface{}{
				"name": "test-product",
			},
		},
	}

	err := processor.leadRepo.CreateLead(ctx, lead)
	if err != nil {
		t.Fatalf("Failed to create lead: %v", err)
	}

	// Note: This test requires a mock Customer API server that returns 4xx
	t.Skip("Skipping 4xx no-retry test - requires mock Customer API server")
}

// TestExecuteDeliveryStage_RetryExhaustion tests permanent failure after max retries
// Requirements: 4.6, 5.4
func TestExecuteDeliveryStage_RetryExhaustion(t *testing.T) {
	processor, cleanup := setupTestProcessor(t)
	if processor == nil {
		return // Test was skipped
	}
	defer cleanup()

	ctx := context.Background()

	// Create a lead in READY status
	lead := &models.InboundLead{
		RawPayload: models.JSONB{
			"email":   "test@example.com",
			"phone":   "1234567890",
			"zipcode": "66123",
		},
		Status: models.LeadStatusReady,
		CustomerPayload: models.JSONB{
			"phone": "1234567890",
			"product": map[string]interface{}{
				"name": "test-product",
			},
		},
	}

	err := processor.leadRepo.CreateLead(ctx, lead)
	if err != nil {
		t.Fatalf("Failed to create lead: %v", err)
	}

	// Note: This test requires a mock Customer API server that always fails
	t.Skip("Skipping retry exhaustion test - requires mock Customer API server")
}

// TestExecuteDeliveryStage_ExponentialBackoffTiming tests exponential backoff delays
// Requirements: 5.3, 5.4
func TestExecuteDeliveryStage_ExponentialBackoffTiming(t *testing.T) {
	// This test would verify that the backoff delays are applied correctly
	// between retry attempts. It requires time measurement and mock server.
	t.Skip("Skipping exponential backoff timing test - requires mock Customer API server and time measurement")
}

// TestExecuteDeliveryStage_AtomicStatusUpdate tests atomic status updates with delivery attempts
// Requirements: 5.5, 6.4
func TestExecuteDeliveryStage_AtomicStatusUpdate(t *testing.T) {
	processor, cleanup := setupTestProcessor(t)
	if processor == nil {
		return // Test was skipped
	}
	defer cleanup()

	ctx := context.Background()

	// Create a lead in READY status
	lead := &models.InboundLead{
		RawPayload: models.JSONB{
			"email":   "test@example.com",
			"phone":   "1234567890",
			"zipcode": "66123",
		},
		Status: models.LeadStatusReady,
		CustomerPayload: models.JSONB{
			"phone": "1234567890",
			"product": map[string]interface{}{
				"name": "test-product",
			},
		},
	}

	err := processor.leadRepo.CreateLead(ctx, lead)
	if err != nil {
		t.Fatalf("Failed to create lead: %v", err)
	}

	// Note: This test requires a mock Customer API server to test transaction atomicity
	t.Skip("Skipping atomic status update test - requires mock Customer API server")
}
