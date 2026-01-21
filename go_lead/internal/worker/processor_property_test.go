package worker

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/checkfox/go_lead/internal/client"
	"github.com/checkfox/go_lead/internal/models"
	"github.com/checkfox/go_lead/internal/repository"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	_ "github.com/lib/pq"
)

// Feature: lead-service-go, Property 7: Max retries leads to permanent failure
// Validates: Requirements 4.6, 5.4
//
// Property: For any lead that encounters retriable errors, after max retry attempts (5),
// the lead should be marked as PERMANENTLY_FAILED
func TestProperty_MaxRetriesLeadsToPermanentFailure(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: Max retries with retriable errors leads to PERMANENTLY_FAILED status
	properties.Property("max retries with retriable errors leads to PERMANENTLY_FAILED", prop.ForAll(
		func(statusCode int) bool {
			// Setup test database
			db := setupTestDB(t)
			if db == nil {
				return true // Skip if database not available
			}
			defer db.Close()
			defer cleanupTestData(t, db)

			// Create a mock Customer API that always returns retriable errors
			mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(statusCode)
				w.Write([]byte(`{"error": "server error"}`))
			}))
			defer mockServer.Close()

			// Create repositories
			leadRepo := repository.NewLeadRepository(db)
			deliveryAttemptRepo := repository.NewDeliveryAttemptRepository(db)

			// Create a test lead in READY status
			lead := &models.InboundLead{
				ReceivedAt: time.Now(),
				RawPayload: models.JSONB{
					"zipcode": "66123",
					"house": map[string]interface{}{
						"is_owner": true,
					},
					"phone": "1234567890",
				},
				Status: models.LeadStatusReady,
				CustomerPayload: models.JSONB{
					"phone": "1234567890",
					"product": map[string]interface{}{
						"name": "test-product",
					},
				},
			}

			ctx := context.Background()
			if err := leadRepo.CreateLead(ctx, lead); err != nil {
				t.Logf("Failed to create lead: %v", err)
				return false
			}

			// Create Customer API client
			customerAPIClient := client.NewCustomerAPIClient(mockServer.URL, "test-token", 5*time.Second)

			// Create processor with max 5 attempts and no delays for testing
			processor := NewProcessor(ProcessorConfig{
				Queue:                    nil, // Not needed for this test
				LeadRepo:                 leadRepo,
				DeliveryAttemptRepo:      deliveryAttemptRepo,
				Validator:                nil,
				Normalizer:               nil,
				Mapper:                   nil,
				CustomerAPIClient:        customerAPIClient,
				MaxDeliveryAttempts:      5,
				ExponentialBackoffDelays: []time.Duration{0, 0, 0, 0, 0}, // No delays for testing
			})

			// Execute delivery stage 5 times (max attempts)
			for i := 0; i < 5; i++ {
				// Reload lead to get current status
				lead, err := leadRepo.GetLeadByID(ctx, lead.ID)
				if err != nil {
					t.Logf("Failed to reload lead: %v", err)
					return false
				}

				// Execute delivery stage
				if err := processor.executeDeliveryStage(ctx, lead); err != nil {
					t.Logf("Delivery stage failed: %v", err)
					return false
				}
			}

			// Reload lead to check final status
			finalLead, err := leadRepo.GetLeadByID(ctx, lead.ID)
			if err != nil {
				t.Logf("Failed to reload final lead: %v", err)
				return false
			}

			// Verify lead is marked as PERMANENTLY_FAILED
			if finalLead.Status != models.LeadStatusPermanentlyFailed {
				t.Logf("Expected status PERMANENTLY_FAILED, got %s", finalLead.Status)
				return false
			}

			// Verify exactly 5 delivery attempts were created
			attempts, err := deliveryAttemptRepo.GetDeliveryAttemptsByLeadID(ctx, lead.ID)
			if err != nil {
				t.Logf("Failed to get delivery attempts: %v", err)
				return false
			}

			if len(attempts) != 5 {
				t.Logf("Expected 5 delivery attempts, got %d", len(attempts))
				return false
			}

			// Verify all attempts failed
			for _, attempt := range attempts {
				if attempt.Success {
					t.Logf("Expected all attempts to fail, but attempt %d succeeded", attempt.AttemptNo)
					return false
				}
			}

			return true
		},
		gen.OneConstOf(500, 502, 503, 504), // Retriable 5xx status codes
	))

	properties.TestingRun(t)
}

// Feature: lead-service-go, Property 8: 4xx errors cause permanent failure without retry
// Validates: Requirements 4.3
//
// Property: For any lead that encounters a non-retriable 4xx error (except 429),
// the lead should be marked as PERMANENTLY_FAILED immediately without retry
func TestProperty_NonRetriableErrorsCausePermanentFailure(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: 4xx errors (except 429) cause immediate PERMANENTLY_FAILED status
	properties.Property("4xx errors cause immediate PERMANENTLY_FAILED without retry", prop.ForAll(
		func(statusCode int) bool {
			// Setup test database
			db := setupTestDB(t)
			if db == nil {
				return true // Skip if database not available
			}
			defer db.Close()
			defer cleanupTestData(t, db)

			// Create a mock Customer API that returns a 4xx error
			mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(statusCode)
				w.Write([]byte(`{"error": "client error"}`))
			}))
			defer mockServer.Close()

			// Create repositories
			leadRepo := repository.NewLeadRepository(db)
			deliveryAttemptRepo := repository.NewDeliveryAttemptRepository(db)

			// Create a test lead in READY status
			lead := &models.InboundLead{
				ReceivedAt: time.Now(),
				RawPayload: models.JSONB{
					"zipcode": "66123",
					"house": map[string]interface{}{
						"is_owner": true,
					},
					"phone": "1234567890",
				},
				Status: models.LeadStatusReady,
				CustomerPayload: models.JSONB{
					"phone": "1234567890",
					"product": map[string]interface{}{
						"name": "test-product",
					},
				},
			}

			ctx := context.Background()
			if err := leadRepo.CreateLead(ctx, lead); err != nil {
				t.Logf("Failed to create lead: %v", err)
				return false
			}

			// Create Customer API client
			customerAPIClient := client.NewCustomerAPIClient(mockServer.URL, "test-token", 5*time.Second)

			// Create processor
			processor := NewProcessor(ProcessorConfig{
				Queue:                    nil,
				LeadRepo:                 leadRepo,
				DeliveryAttemptRepo:      deliveryAttemptRepo,
				Validator:                nil,
				Normalizer:               nil,
				Mapper:                   nil,
				CustomerAPIClient:        customerAPIClient,
				MaxDeliveryAttempts:      5,
				ExponentialBackoffDelays: []time.Duration{0, 0, 0, 0, 0},
			})

			// Execute delivery stage once
			if err := processor.executeDeliveryStage(ctx, lead); err != nil {
				t.Logf("Delivery stage failed: %v", err)
				return false
			}

			// Reload lead to check status
			finalLead, err := leadRepo.GetLeadByID(ctx, lead.ID)
			if err != nil {
				t.Logf("Failed to reload lead: %v", err)
				return false
			}

			// Verify lead is marked as PERMANENTLY_FAILED immediately
			if finalLead.Status != models.LeadStatusPermanentlyFailed {
				t.Logf("Expected status PERMANENTLY_FAILED, got %s", finalLead.Status)
				return false
			}

			// Verify exactly 1 delivery attempt was created (no retries)
			attempts, err := deliveryAttemptRepo.GetDeliveryAttemptsByLeadID(ctx, lead.ID)
			if err != nil {
				t.Logf("Failed to get delivery attempts: %v", err)
				return false
			}

			if len(attempts) != 1 {
				t.Logf("Expected 1 delivery attempt (no retries), got %d", len(attempts))
				return false
			}

			// Verify the attempt failed
			if attempts[0].Success {
				t.Logf("Expected attempt to fail")
				return false
			}

			return true
		},
		gen.OneConstOf(400, 401, 403, 404, 422), // Non-retriable 4xx status codes (excluding 429)
	))

	properties.TestingRun(t)
}

// setupTestDB creates a test database connection
// This will skip tests if no database is available
func setupTestDB(t *testing.T) *sql.DB {
	connStr := "host=localhost port=5432 user=postgres password=postgres dbname=test_lead_gateway sslmode=disable"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Skipf("Skipping test - cannot connect to test database: %v", err)
		return nil
	}

	if err := db.Ping(); err != nil {
		t.Skipf("Skipping test - test database not available: %v", err)
		return nil
	}

	return db
}

// cleanupTestData removes test data from the database
func cleanupTestData(t *testing.T, db *sql.DB) {
	_, err := db.Exec("DELETE FROM delivery_attempt")
	if err != nil {
		t.Logf("Warning: failed to clean delivery_attempt table: %v", err)
	}
	_, err = db.Exec("DELETE FROM inbound_lead")
	if err != nil {
		t.Logf("Warning: failed to clean inbound_lead table: %v", err)
	}
}
