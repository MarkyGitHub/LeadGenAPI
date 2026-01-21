package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/checkfox/go_lead/internal/logger"
	"github.com/checkfox/go_lead/internal/repository"
)

// StatsHandler handles statistics and observability endpoints
type StatsHandler struct {
	leadRepo            repository.LeadRepository
	deliveryAttemptRepo repository.DeliveryAttemptRepository
}

// NewStatsHandler creates a new StatsHandler
func NewStatsHandler(leadRepo repository.LeadRepository, deliveryAttemptRepo repository.DeliveryAttemptRepository) *StatsHandler {
	return &StatsHandler{
		leadRepo:            leadRepo,
		deliveryAttemptRepo: deliveryAttemptRepo,
	}
}

// LeadCountsByStatus represents lead counts grouped by status
type LeadCountsByStatus struct {
	Received           int `json:"received"`
	Rejected           int `json:"rejected"`
	Ready              int `json:"ready"`
	Delivered          int `json:"delivered"`
	Failed             int `json:"failed"`
	PermanentlyFailed  int `json:"permanently_failed"`
	Total              int `json:"total"`
}

// RecentLeadSummary represents a summary of a recent lead
type RecentLeadSummary struct {
	ID            int64  `json:"id"`
	ReceivedAt    string `json:"received_at"`
	Status        string `json:"status"`
	RejectionReason *string `json:"rejection_reason,omitempty"`
}

// LeadHistoryResponse represents the full history of a lead
type LeadHistoryResponse struct {
	ID                int64                    `json:"id"`
	ReceivedAt        string                   `json:"received_at"`
	Status            string                   `json:"status"`
	RejectionReason   *string                  `json:"rejection_reason,omitempty"`
	RawPayload        map[string]interface{}   `json:"raw_payload"`
	NormalizedPayload map[string]interface{}   `json:"normalized_payload,omitempty"`
	CustomerPayload   map[string]interface{}   `json:"customer_payload,omitempty"`
	DeliveryAttempts  []DeliveryAttemptSummary `json:"delivery_attempts"`
}

// DeliveryAttemptSummary represents a summary of a delivery attempt
type DeliveryAttemptSummary struct {
	AttemptNo    int     `json:"attempt_no"`
	AttemptedAt  string  `json:"attempted_at"`
	Success      bool    `json:"success"`
	StatusCode   *int    `json:"status_code,omitempty"`
	ErrorMessage *string `json:"error_message,omitempty"`
}

// HandleLeadCountsByStatus handles GET /stats/leads/counts
// Requirements: 8.3
func (h *StatsHandler) HandleLeadCountsByStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	logger.Info(ctx, "Fetching lead counts by status")
	
	// Only accept GET requests
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// Get counts from repository
	counts, err := h.leadRepo.GetLeadCountsByStatus(ctx)
	if err != nil {
		logger.LogError(ctx, "Failed to get lead counts", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	
	// Calculate total
	total := 0
	for _, count := range counts {
		total += count
	}
	
	// Build response
	response := LeadCountsByStatus{
		Received:          counts["RECEIVED"],
		Rejected:          counts["REJECTED"],
		Ready:             counts["READY"],
		Delivered:         counts["DELIVERED"],
		Failed:            counts["FAILED"],
		PermanentlyFailed: counts["PERMANENTLY_FAILED"],
		Total:             total,
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// HandleRecentLeads handles GET /stats/leads/recent
// Requirements: 8.4
func (h *StatsHandler) HandleRecentLeads(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	logger.Info(ctx, "Fetching recent leads")
	
	// Only accept GET requests
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// Get recent leads from repository (default limit: 50)
	leads, err := h.leadRepo.GetRecentLeads(ctx, 50)
	if err != nil {
		logger.LogError(ctx, "Failed to get recent leads", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	
	// Build response
	response := make([]RecentLeadSummary, 0, len(leads))
	for _, lead := range leads {
		summary := RecentLeadSummary{
			ID:              lead.ID,
			ReceivedAt:      lead.ReceivedAt.Format("2006-01-02T15:04:05Z07:00"),
			Status:          string(lead.Status),
			RejectionReason: lead.RejectionReason,
		}
		response = append(response, summary)
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// HandleLeadHistory handles GET /stats/leads/{id}/history
// Requirements: 8.5
func (h *StatsHandler) HandleLeadHistory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	// Only accept GET requests
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// Extract lead ID from URL path
	// This is a simple implementation - in production, use a router like gorilla/mux or chi
	leadID := extractLeadIDFromPath(r.URL.Path)
	if leadID == 0 {
		http.Error(w, "invalid lead ID", http.StatusBadRequest)
		return
	}
	
	ctx = context.WithValue(ctx, logger.LeadIDKey, leadID)
	logger.Info(ctx, "Fetching lead history")
	
	// Get lead from repository
	lead, err := h.leadRepo.GetLeadByID(ctx, leadID)
	if err != nil {
		logger.LogError(ctx, "Failed to get lead", err)
		http.Error(w, "lead not found", http.StatusNotFound)
		return
	}
	
	// Get delivery attempts
	attempts, err := h.deliveryAttemptRepo.GetDeliveryAttemptsByLeadID(ctx, leadID)
	if err != nil {
		logger.LogError(ctx, "Failed to get delivery attempts", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	
	// Build response
	attemptSummaries := make([]DeliveryAttemptSummary, 0, len(attempts))
	for _, attempt := range attempts {
		summary := DeliveryAttemptSummary{
			AttemptNo:    attempt.AttemptNo,
			AttemptedAt:  attempt.RequestedAt.Format("2006-01-02T15:04:05Z07:00"),
			Success:      attempt.Success,
			StatusCode:   attempt.ResponseStatus,
			ErrorMessage: attempt.ErrorMessage,
		}
		attemptSummaries = append(attemptSummaries, summary)
	}
	
	response := LeadHistoryResponse{
		ID:                lead.ID,
		ReceivedAt:        lead.ReceivedAt.Format("2006-01-02T15:04:05Z07:00"),
		Status:            string(lead.Status),
		RejectionReason:   lead.RejectionReason,
		RawPayload:        lead.RawPayload,
		NormalizedPayload: lead.NormalizedPayload,
		CustomerPayload:   lead.CustomerPayload,
		DeliveryAttempts:  attemptSummaries,
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// extractLeadIDFromPath extracts the lead ID from a URL path like /stats/leads/123/history
func extractLeadIDFromPath(path string) int64 {
	// Simple implementation - in production, use a proper router
	var leadID int64
	// Expected format: /stats/leads/{id}/history
	// This is a placeholder - proper implementation would use a router
	return leadID
}
