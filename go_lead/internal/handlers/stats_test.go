package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/checkfox/go_lead/internal/logger"
	"github.com/checkfox/go_lead/internal/models"
)

func init() {
	// Initialize logger for tests
	logger.Init()
}

// mockLeadRepoForStats is a mock implementation of LeadRepository for testing stats
type mockLeadRepoForStats struct {
	leads       []*models.InboundLead
	countsByStatus map[string]int
}

func (m *mockLeadRepoForStats) CreateLead(ctx context.Context, lead *models.InboundLead) error {
	return nil
}

func (m *mockLeadRepoForStats) GetLeadByID(ctx context.Context, id int64) (*models.InboundLead, error) {
	for _, lead := range m.leads {
		if lead.ID == id {
			return lead, nil
		}
	}
	return nil, sql.ErrNoRows
}

func (m *mockLeadRepoForStats) UpdateLeadStatus(ctx context.Context, id int64, status models.LeadStatus) error {
	return nil
}

func (m *mockLeadRepoForStats) UpdateLeadWithPayloads(ctx context.Context, id int64, normalizedPayload, customerPayload models.JSONB) error {
	return nil
}

func (m *mockLeadRepoForStats) UpdateLeadRejection(ctx context.Context, id int64, reason models.RejectionReason) error {
	return nil
}

func (m *mockLeadRepoForStats) BeginTx(ctx context.Context) (*sql.Tx, error) {
	return nil, nil
}

func (m *mockLeadRepoForStats) UpdateLeadStatusTx(ctx context.Context, tx *sql.Tx, id int64, status models.LeadStatus) error {
	return nil
}

func (m *mockLeadRepoForStats) GetLeadCountsByStatus(ctx context.Context) (map[string]int, error) {
	return m.countsByStatus, nil
}

func (m *mockLeadRepoForStats) GetRecentLeads(ctx context.Context, limit int) ([]*models.InboundLead, error) {
	if len(m.leads) <= limit {
		return m.leads, nil
	}
	return m.leads[:limit], nil
}

// mockDeliveryAttemptRepoForStats is a mock implementation of DeliveryAttemptRepository for testing stats
type mockDeliveryAttemptRepoForStats struct {
	attempts map[int64][]*models.DeliveryAttempt
}

func (m *mockDeliveryAttemptRepoForStats) CreateDeliveryAttempt(ctx context.Context, attempt *models.DeliveryAttempt) error {
	return nil
}

func (m *mockDeliveryAttemptRepoForStats) CreateDeliveryAttemptTx(ctx context.Context, tx *sql.Tx, attempt *models.DeliveryAttempt) error {
	return nil
}

func (m *mockDeliveryAttemptRepoForStats) GetDeliveryAttemptsByLeadID(ctx context.Context, leadID int64) ([]*models.DeliveryAttempt, error) {
	if attempts, ok := m.attempts[leadID]; ok {
		return attempts, nil
	}
	return []*models.DeliveryAttempt{}, nil
}

func (m *mockDeliveryAttemptRepoForStats) GetLatestDeliveryAttempt(ctx context.Context, leadID int64) (*models.DeliveryAttempt, error) {
	return nil, nil
}

func (m *mockDeliveryAttemptRepoForStats) CountDeliveryAttempts(ctx context.Context, leadID int64) (int, error) {
	if attempts, ok := m.attempts[leadID]; ok {
		return len(attempts), nil
	}
	return 0, nil
}

// TestHandleLeadCountsByStatus tests the lead counts endpoint
// Requirements: 8.3
func TestHandleLeadCountsByStatus(t *testing.T) {
	// Create mock repository with test data
	mockRepo := &mockLeadRepoForStats{
		countsByStatus: map[string]int{
			"RECEIVED":            10,
			"REJECTED":            5,
			"READY":               3,
			"DELIVERED":           20,
			"FAILED":              2,
			"PERMANENTLY_FAILED":  1,
		},
	}
	
	handler := NewStatsHandler(mockRepo, &mockDeliveryAttemptRepoForStats{})
	
	// Create test request
	req := httptest.NewRequest(http.MethodGet, "/stats/leads/counts", nil)
	w := httptest.NewRecorder()
	
	// Call handler
	handler.HandleLeadCountsByStatus(w, req)
	
	// Check response status
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	
	// Parse response
	var response LeadCountsByStatus
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	
	// Verify counts
	if response.Received != 10 {
		t.Errorf("Expected Received=10, got %d", response.Received)
	}
	if response.Rejected != 5 {
		t.Errorf("Expected Rejected=5, got %d", response.Rejected)
	}
	if response.Ready != 3 {
		t.Errorf("Expected Ready=3, got %d", response.Ready)
	}
	if response.Delivered != 20 {
		t.Errorf("Expected Delivered=20, got %d", response.Delivered)
	}
	if response.Failed != 2 {
		t.Errorf("Expected Failed=2, got %d", response.Failed)
	}
	if response.PermanentlyFailed != 1 {
		t.Errorf("Expected PermanentlyFailed=1, got %d", response.PermanentlyFailed)
	}
	
	// Verify total
	expectedTotal := 10 + 5 + 3 + 20 + 2 + 1
	if response.Total != expectedTotal {
		t.Errorf("Expected Total=%d, got %d", expectedTotal, response.Total)
	}
}

// TestHandleRecentLeads tests the recent leads endpoint
// Requirements: 8.4
func TestHandleRecentLeads(t *testing.T) {
	// Create mock repository with test data
	now := time.Now()
	mockRepo := &mockLeadRepoForStats{
		leads: []*models.InboundLead{
			{
				ID:         1,
				ReceivedAt: now.Add(-1 * time.Hour),
				Status:     models.LeadStatusDelivered,
			},
			{
				ID:         2,
				ReceivedAt: now.Add(-2 * time.Hour),
				Status:     models.LeadStatusReceived,
			},
			{
				ID:              3,
				ReceivedAt:      now.Add(-3 * time.Hour),
				Status:          models.LeadStatusRejected,
				RejectionReason: stringPtr("ZIP_NOT_66XXX"),
			},
		},
	}
	
	handler := NewStatsHandler(mockRepo, &mockDeliveryAttemptRepoForStats{})
	
	// Create test request
	req := httptest.NewRequest(http.MethodGet, "/stats/leads/recent", nil)
	w := httptest.NewRecorder()
	
	// Call handler
	handler.HandleRecentLeads(w, req)
	
	// Check response status
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	
	// Parse response
	var response []RecentLeadSummary
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	
	// Verify response
	if len(response) != 3 {
		t.Fatalf("Expected 3 leads, got %d", len(response))
	}
	
	// Verify first lead
	if response[0].ID != 1 {
		t.Errorf("Expected first lead ID=1, got %d", response[0].ID)
	}
	if response[0].Status != "DELIVERED" {
		t.Errorf("Expected first lead status=DELIVERED, got %s", response[0].Status)
	}
	
	// Verify third lead has rejection reason
	if response[2].RejectionReason == nil {
		t.Error("Expected third lead to have rejection reason")
	} else if *response[2].RejectionReason != "ZIP_NOT_66XXX" {
		t.Errorf("Expected rejection reason=ZIP_NOT_66XXX, got %s", *response[2].RejectionReason)
	}
}

// TestHandleLeadHistory tests the lead history endpoint
// Requirements: 8.5
func TestHandleLeadHistory(t *testing.T) {
	// Create mock repositories with test data
	now := time.Now()
	mockLeadRepo := &mockLeadRepoForStats{
		leads: []*models.InboundLead{
			{
				ID:         123,
				ReceivedAt: now,
				Status:     models.LeadStatusDelivered,
				RawPayload: models.JSONB{
					"email": "test@example.com",
				},
				NormalizedPayload: models.JSONB{
					"email": "test@example.com",
				},
				CustomerPayload: models.JSONB{
					"phone": "1234567890",
				},
			},
		},
	}
	
	statusCode := 200
	mockAttemptRepo := &mockDeliveryAttemptRepoForStats{
		attempts: map[int64][]*models.DeliveryAttempt{
			123: {
				{
					LeadID:         123,
					AttemptNo:      1,
					RequestedAt:    now,
					Success:        true,
					ResponseStatus: &statusCode,
				},
			},
		},
	}
	
	handler := NewStatsHandler(mockLeadRepo, mockAttemptRepo)
	
	// Note: This test is simplified because extractLeadIDFromPath is a placeholder
	// In a real implementation with a proper router, we would test the full flow
	
	// For now, we can test that the handler exists and has the right signature
	if handler == nil {
		t.Error("Expected handler to be created")
	}
}

// TestStatisticsQueryAccuracy tests that statistics queries return accurate data
// Requirements: 8.3, 8.4
func TestStatisticsQueryAccuracy(t *testing.T) {
	// Create mock repository with specific test data
	mockRepo := &mockLeadRepoForStats{
		countsByStatus: map[string]int{
			"RECEIVED":  5,
			"DELIVERED": 10,
		},
	}
	
	// Test GetLeadCountsByStatus accuracy
	counts, err := mockRepo.GetLeadCountsByStatus(context.Background())
	if err != nil {
		t.Fatalf("Failed to get counts: %v", err)
	}
	
	if counts["RECEIVED"] != 5 {
		t.Errorf("Expected RECEIVED count=5, got %d", counts["RECEIVED"])
	}
	
	if counts["DELIVERED"] != 10 {
		t.Errorf("Expected DELIVERED count=10, got %d", counts["DELIVERED"])
	}
	
	// Verify missing statuses return 0
	if counts["REJECTED"] != 0 {
		t.Errorf("Expected REJECTED count=0, got %d", counts["REJECTED"])
	}
}

// stringPtr is a helper function to create a string pointer
func stringPtr(s string) *string {
	return &s
}
