package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/checkfox/go_lead/internal/models"
	"github.com/checkfox/go_lead/internal/queue"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: lead-service-go, Property 6: Webhook responds within 500ms
// Validates: Requirements 1.1
//
// Property: For any valid JSON payload, the webhook endpoint should respond within 500ms
func TestProperty_WebhookResponseTime(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	
	properties := gopter.NewProperties(parameters)
	
	// Create mock repository and queue
	mockRepo := &MockLeadRepository{}
	mockQueue := &MockQueue{}
	
	handler := NewWebhookHandler(mockRepo, mockQueue)
	
	// Property: Webhook responds within 500ms for any valid JSON payload
	properties.Property("webhook responds within 500ms for any valid JSON", prop.ForAll(
		func(email string, phone string, zipcode string) bool {
			// Create a valid JSON payload
			payload := map[string]interface{}{
				"email":   email,
				"phone":   phone,
				"zipcode": zipcode,
				"house": map[string]interface{}{
					"is_owner": true,
				},
			}
			
			payloadBytes, err := json.Marshal(payload)
			if err != nil {
				t.Logf("Failed to marshal payload: %v", err)
				return false
			}
			
			// Create request
			req := httptest.NewRequest(http.MethodPost, "/webhooks/leads", bytes.NewReader(payloadBytes))
			req.Header.Set("Content-Type", "application/json")
			
			// Create response recorder
			rr := httptest.NewRecorder()
			
			// Measure response time
			start := time.Now()
			handler.HandleLeadWebhook(rr, req)
			elapsed := time.Since(start)
			
			// Check that response time is under 500ms
			if elapsed > 500*time.Millisecond {
				t.Logf("Response time exceeded 500ms: %v", elapsed)
				return false
			}
			
			// Also verify we got a successful response (200 OK)
			return rr.Code == http.StatusOK
		},
		gen.AlphaString(),
		gen.NumString(),
		gen.NumString(),
	))
	
	properties.TestingRun(t)
}

// MockLeadRepository is a mock implementation of LeadRepository for testing
type MockLeadRepository struct{}

func (m *MockLeadRepository) CreateLead(ctx context.Context, lead *models.InboundLead) error {
	// Simulate fast database write
	lead.ID = 12345
	return nil
}

func (m *MockLeadRepository) GetLeadByID(ctx context.Context, id int64) (*models.InboundLead, error) {
	return &models.InboundLead{ID: id}, nil
}

func (m *MockLeadRepository) UpdateLeadStatus(ctx context.Context, id int64, status models.LeadStatus) error {
	return nil
}

func (m *MockLeadRepository) UpdateLeadWithPayloads(ctx context.Context, id int64, normalizedPayload, customerPayload models.JSONB) error {
	return nil
}

func (m *MockLeadRepository) UpdateLeadRejection(ctx context.Context, id int64, reason models.RejectionReason) error {
	return nil
}

func (m *MockLeadRepository) BeginTx(ctx context.Context) (*sql.Tx, error) {
	return nil, nil
}

func (m *MockLeadRepository) UpdateLeadStatusTx(ctx context.Context, tx *sql.Tx, id int64, status models.LeadStatus) error {
	return nil
}

func (m *MockLeadRepository) GetLeadCountsByStatus(ctx context.Context) (map[string]int, error) {
	return make(map[string]int), nil
}

func (m *MockLeadRepository) GetRecentLeads(ctx context.Context, limit int) ([]*models.InboundLead, error) {
	return []*models.InboundLead{}, nil
}

// MockQueue is a mock implementation of Queue for testing
type MockQueue struct{}

func (m *MockQueue) Enqueue(ctx context.Context, jobType string, payload map[string]interface{}) error {
	// Simulate fast queue operation
	return nil
}

func (m *MockQueue) EnqueueWithDelay(ctx context.Context, jobType string, payload map[string]interface{}, delay time.Duration) error {
	return nil
}

func (m *MockQueue) Dequeue(ctx context.Context) (*queue.Job, error) {
	return nil, nil
}

func (m *MockQueue) Complete(ctx context.Context, jobID int64) error {
	return nil
}

func (m *MockQueue) Retry(ctx context.Context, jobID int64, delay time.Duration) error {
	return nil
}

func (m *MockQueue) Fail(ctx context.Context, jobID int64, errorMsg string) error {
	return nil
}

func (m *MockQueue) HealthCheck(ctx context.Context) error {
	return nil
}

func (m *MockQueue) Close() error {
	return nil
}
