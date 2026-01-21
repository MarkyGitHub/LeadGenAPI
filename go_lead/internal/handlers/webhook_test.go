package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/checkfox/go_lead/internal/models"
	"github.com/checkfox/go_lead/internal/queue"
)

// Test successful lead acceptance
func TestHandleLeadWebhook_Success(t *testing.T) {
	mockRepo := &MockLeadRepository{}
	mockQueue := &MockQueue{}
	handler := NewWebhookHandler(mockRepo, mockQueue)

	payload := map[string]interface{}{
		"email":   "test@example.com",
		"phone":   "1234567890",
		"zipcode": "66001",
		"house": map[string]interface{}{
			"is_owner": true,
		},
	}

	payloadBytes, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/leads", bytes.NewReader(payloadBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "TestClient/1.0")

	rr := httptest.NewRecorder()
	handler.HandleLeadWebhook(rr, req)

	// Check status code
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	// Check response body
	var response WebhookResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.LeadID != 12345 {
		t.Errorf("Expected lead_id 12345, got %d", response.LeadID)
	}

	if response.Status != "RECEIVED" {
		t.Errorf("Expected status RECEIVED, got %s", response.Status)
	}

	if response.CorrelationID == "" {
		t.Error("Expected correlation_id to be set")
	}

	// Check correlation ID header
	if rr.Header().Get("X-Correlation-ID") == "" {
		t.Error("Expected X-Correlation-ID header to be set")
	}
}

// Test malformed JSON rejection
func TestHandleLeadWebhook_MalformedJSON(t *testing.T) {
	mockRepo := &MockLeadRepository{}
	mockQueue := &MockQueue{}
	handler := NewWebhookHandler(mockRepo, mockQueue)

	// Invalid JSON
	req := httptest.NewRequest(http.MethodPost, "/webhooks/leads", bytes.NewReader([]byte("{invalid json")))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.HandleLeadWebhook(rr, req)

	// Check status code
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rr.Code)
	}

	// Check error response
	var response ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode error response: %v", err)
	}

	if response.Error != "malformed JSON payload" {
		t.Errorf("Expected error 'malformed JSON payload', got '%s'", response.Error)
	}
}

// Test method not allowed
func TestHandleLeadWebhook_MethodNotAllowed(t *testing.T) {
	mockRepo := &MockLeadRepository{}
	mockQueue := &MockQueue{}
	handler := NewWebhookHandler(mockRepo, mockQueue)

	req := httptest.NewRequest(http.MethodGet, "/webhooks/leads", nil)
	rr := httptest.NewRecorder()
	handler.HandleLeadWebhook(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", rr.Code)
	}
}

// Test database error returns 503
func TestHandleLeadWebhook_DatabaseError(t *testing.T) {
	mockRepo := &MockLeadRepositoryWithError{
		createLeadError: errors.New("database connection failed"),
	}
	mockQueue := &MockQueue{}
	handler := NewWebhookHandler(mockRepo, mockQueue)

	payload := map[string]interface{}{
		"email": "test@example.com",
	}

	payloadBytes, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/leads", bytes.NewReader(payloadBytes))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.HandleLeadWebhook(rr, req)

	// Check status code
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503, got %d", rr.Code)
	}

	// Check error response
	var response ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode error response: %v", err)
	}

	if response.Error != "database error" {
		t.Errorf("Expected error 'database error', got '%s'", response.Error)
	}
}

// Test queue unavailability returns 503
func TestHandleLeadWebhook_QueueUnavailable(t *testing.T) {
	mockRepo := &MockLeadRepository{}
	mockQueue := &MockQueueWithError{
		enqueueError: errors.New("queue connection failed"),
	}
	handler := NewWebhookHandler(mockRepo, mockQueue)

	payload := map[string]interface{}{
		"email": "test@example.com",
	}

	payloadBytes, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/leads", bytes.NewReader(payloadBytes))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.HandleLeadWebhook(rr, req)

	// Check status code
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503, got %d", rr.Code)
	}

	// Check error response
	var response ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode error response: %v", err)
	}

	if response.Error != "queue unavailable" {
		t.Errorf("Expected error 'queue unavailable', got '%s'", response.Error)
	}
}

// MockLeadRepositoryWithError simulates repository errors
type MockLeadRepositoryWithError struct {
	createLeadError error
}

func (m *MockLeadRepositoryWithError) CreateLead(ctx context.Context, lead *models.InboundLead) error {
	if m.createLeadError != nil {
		return m.createLeadError
	}
	lead.ID = 12345
	return nil
}

func (m *MockLeadRepositoryWithError) GetLeadByID(ctx context.Context, id int64) (*models.InboundLead, error) {
	return &models.InboundLead{ID: id}, nil
}

func (m *MockLeadRepositoryWithError) UpdateLeadStatus(ctx context.Context, id int64, status models.LeadStatus) error {
	return nil
}

func (m *MockLeadRepositoryWithError) UpdateLeadWithPayloads(ctx context.Context, id int64, normalizedPayload, customerPayload models.JSONB) error {
	return nil
}

func (m *MockLeadRepositoryWithError) UpdateLeadRejection(ctx context.Context, id int64, reason models.RejectionReason) error {
	return nil
}

func (m *MockLeadRepositoryWithError) BeginTx(ctx context.Context) (*sql.Tx, error) {
	return nil, nil
}

func (m *MockLeadRepositoryWithError) UpdateLeadStatusTx(ctx context.Context, tx *sql.Tx, id int64, status models.LeadStatus) error {
	return nil
}

func (m *MockLeadRepositoryWithError) GetLeadCountsByStatus(ctx context.Context) (map[string]int, error) {
	return make(map[string]int), nil
}

func (m *MockLeadRepositoryWithError) GetRecentLeads(ctx context.Context, limit int) ([]*models.InboundLead, error) {
	return []*models.InboundLead{}, nil
}

// MockQueueWithError simulates queue errors
type MockQueueWithError struct {
	enqueueError error
}

func (m *MockQueueWithError) Enqueue(ctx context.Context, jobType string, payload map[string]interface{}) error {
	if m.enqueueError != nil {
		return m.enqueueError
	}
	return nil
}

func (m *MockQueueWithError) EnqueueWithDelay(ctx context.Context, jobType string, payload map[string]interface{}, delay time.Duration) error {
	return nil
}

func (m *MockQueueWithError) Dequeue(ctx context.Context) (*queue.Job, error) {
	return nil, nil
}

func (m *MockQueueWithError) Complete(ctx context.Context, jobID int64) error {
	return nil
}

func (m *MockQueueWithError) Retry(ctx context.Context, jobID int64, delay time.Duration) error {
	return nil
}

func (m *MockQueueWithError) Fail(ctx context.Context, jobID int64, errorMsg string) error {
	return nil
}

func (m *MockQueueWithError) HealthCheck(ctx context.Context) error {
	return nil
}

func (m *MockQueueWithError) Close() error {
	return nil
}
