package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/checkfox/go_lead/internal/logger"
	"github.com/checkfox/go_lead/internal/models"
	"github.com/checkfox/go_lead/internal/queue"
	"github.com/checkfox/go_lead/internal/repository"
	"github.com/google/uuid"
)

// WebhookHandler handles webhook requests for lead reception
type WebhookHandler struct {
	leadRepo repository.LeadRepository
	queue    queue.Queue
}

// NewWebhookHandler creates a new WebhookHandler
func NewWebhookHandler(leadRepo repository.LeadRepository, q queue.Queue) *WebhookHandler {
	return &WebhookHandler{
		leadRepo: leadRepo,
		queue:    q,
	}
}

// WebhookResponse represents the response returned to webhook callers
type WebhookResponse struct {
	LeadID        int64  `json:"lead_id"`
	Status        string `json:"status"`
	CorrelationID string `json:"correlation_id"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error         string `json:"error"`
	CorrelationID string `json:"correlation_id,omitempty"`
}

// HandleLeadWebhook handles POST /webhooks/leads
func (h *WebhookHandler) HandleLeadWebhook(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	
	// Generate correlation ID for request tracing
	correlationID := uuid.New().String()
	ctx := context.WithValue(r.Context(), logger.CorrelationIDKey, correlationID)
	
	// Log incoming request
	logger.Info(ctx, "Received webhook request",
		"remote_addr", r.RemoteAddr,
		"method", r.Method,
	)
	
	// Only accept POST requests
	if r.Method != http.MethodPost {
		h.respondError(w, ctx, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		logger.LogError(ctx, "Failed to read request body", err)
		h.respondError(w, ctx, http.StatusBadRequest, "failed to read request body")
		return
	}
	defer r.Body.Close()
	
	// Validate JSON
	var rawPayload map[string]interface{}
	if err := json.Unmarshal(body, &rawPayload); err != nil {
		logger.LogError(ctx, "Malformed JSON payload", err)
		h.respondError(w, ctx, http.StatusBadRequest, "malformed JSON payload")
		return
	}
	
	// Extract headers for audit trail
	headers := make(map[string]interface{})
	for key, values := range r.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}
	
	// Create lead record
	lead := &models.InboundLead{
		ReceivedAt:    time.Now(),
		RawPayload:    rawPayload,
		SourceHeaders: headers,
		Status:        models.LeadStatusReceived,
	}
	
	// Store lead to database
	if err := h.leadRepo.CreateLead(ctx, lead); err != nil {
		logger.LogError(ctx, "Failed to create lead", err)
		h.respondError(w, ctx, http.StatusServiceUnavailable, "database error")
		return
	}
	
	// Add lead_id to context for subsequent logging
	ctx = context.WithValue(ctx, logger.LeadIDKey, lead.ID)
	
	logger.Info(ctx, "Created lead", "status", lead.Status)
	
	// Enqueue background job for processing
	jobPayload := queue.NewJobPayload(lead.ID)
	if err := h.queue.Enqueue(ctx, "process_lead", jobPayload); err != nil {
		logger.LogError(ctx, "Failed to enqueue job", err)
		h.respondError(w, ctx, http.StatusServiceUnavailable, "queue unavailable")
		return
	}
	
	logger.Info(ctx, "Enqueued processing job")
	
	// Log slow operation if needed
	duration := time.Since(startTime)
	logger.LogSlowOperation(ctx, "webhook_request", duration)
	
	// Return success response
	response := WebhookResponse{
		LeadID:        lead.ID,
		Status:        string(lead.Status),
		CorrelationID: correlationID,
	}
	
	h.respondJSON(w, ctx, http.StatusOK, response)
}

// respondJSON sends a JSON response
func (h *WebhookHandler) respondJSON(w http.ResponseWriter, ctx context.Context, statusCode int, data interface{}) {
	if correlationID, ok := ctx.Value(logger.CorrelationIDKey).(string); ok {
		w.Header().Set("X-Correlation-ID", correlationID)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	
	if err := json.NewEncoder(w).Encode(data); err != nil {
		logger.LogError(ctx, "Failed to encode response", err)
	}
}

// respondError sends an error response
func (h *WebhookHandler) respondError(w http.ResponseWriter, ctx context.Context, statusCode int, message string) {
	correlationID := ""
	if id, ok := ctx.Value(logger.CorrelationIDKey).(string); ok {
		correlationID = id
	}
	
	response := ErrorResponse{
		Error:         message,
		CorrelationID: correlationID,
	}
	h.respondJSON(w, ctx, statusCode, response)
}
