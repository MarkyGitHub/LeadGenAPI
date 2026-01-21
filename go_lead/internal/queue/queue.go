package queue

import (
	"context"
	"encoding/json"
	"time"
)

// Job represents a background job to be processed
type Job struct {
	ID        int64                  `json:"id"`
	Type      string                 `json:"type"`
	Payload   map[string]interface{} `json:"payload"`
	CreatedAt time.Time              `json:"created_at"`
	NextRunAt time.Time              `json:"next_run_at"`
	Attempts  int                    `json:"attempts"`
}

// Queue defines the interface for job queue operations
type Queue interface {
	// Enqueue adds a new job to the queue
	Enqueue(ctx context.Context, jobType string, payload map[string]interface{}) error

	// EnqueueWithDelay adds a job to be processed after a delay
	EnqueueWithDelay(ctx context.Context, jobType string, payload map[string]interface{}, delay time.Duration) error

	// Dequeue retrieves the next available job from the queue
	// Returns nil if no jobs are available
	Dequeue(ctx context.Context) (*Job, error)

	// Complete marks a job as successfully completed and removes it from the queue
	Complete(ctx context.Context, jobID int64) error

	// Retry reschedules a job for retry with a delay
	Retry(ctx context.Context, jobID int64, delay time.Duration) error

	// Fail marks a job as permanently failed
	Fail(ctx context.Context, jobID int64, errorMsg string) error

	// HealthCheck verifies the queue is operational
	HealthCheck(ctx context.Context) error

	// Close closes the queue connection
	Close() error
}

// JobPayload is a helper to create job payloads
func NewJobPayload(leadID int64) map[string]interface{} {
	return map[string]interface{}{
		"lead_id": leadID,
	}
}

// GetLeadID extracts lead_id from job payload
func GetLeadID(payload map[string]interface{}) (int64, bool) {
	leadID, ok := payload["lead_id"]
	if !ok {
		return 0, false
	}

	// Handle different numeric types
	switch v := leadID.(type) {
	case int64:
		return v, true
	case float64:
		return int64(v), true
	case json.Number:
		if i, err := v.Int64(); err == nil {
			return i, true
		}
	}

	return 0, false
}
