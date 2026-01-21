package queue

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// DBQueue implements Queue interface using PostgreSQL
type DBQueue struct {
	db *sql.DB
}

// NewDBQueue creates a new database-backed queue
func NewDBQueue(db *sql.DB) (*DBQueue, error) {
	if db == nil {
		return nil, fmt.Errorf("database connection is required")
	}

	queue := &DBQueue{db: db}

	// Ensure the jobs table exists
	if err := queue.ensureTable(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ensure jobs table: %w", err)
	}

	return queue, nil
}

// ensureTable creates the jobs table if it doesn't exist
func (q *DBQueue) ensureTable(ctx context.Context) error {
	query := `
		CREATE TABLE IF NOT EXISTS background_jobs (
			id SERIAL PRIMARY KEY,
			job_type VARCHAR(100) NOT NULL,
			payload JSONB NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			next_run_at TIMESTAMP NOT NULL DEFAULT NOW(),
			attempts INT NOT NULL DEFAULT 0,
			status VARCHAR(20) NOT NULL DEFAULT 'pending',
			error_message TEXT,
			completed_at TIMESTAMP,
			failed_at TIMESTAMP
		);

		CREATE INDEX IF NOT EXISTS idx_background_jobs_next_run 
		ON background_jobs(next_run_at) 
		WHERE status = 'pending';

		CREATE INDEX IF NOT EXISTS idx_background_jobs_status 
		ON background_jobs(status);
	`

	_, err := q.db.ExecContext(ctx, query)
	return err
}

// Enqueue adds a new job to the queue
func (q *DBQueue) Enqueue(ctx context.Context, jobType string, payload map[string]interface{}) error {
	return q.EnqueueWithDelay(ctx, jobType, payload, 0)
}

// EnqueueWithDelay adds a job to be processed after a delay
func (q *DBQueue) EnqueueWithDelay(ctx context.Context, jobType string, payload map[string]interface{}, delay time.Duration) error {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal job payload: %w", err)
	}

	nextRunAt := time.Now().Add(delay)

	query := `
		INSERT INTO background_jobs (job_type, payload, next_run_at)
		VALUES ($1, $2, $3)
	`

	_, err = q.db.ExecContext(ctx, query, jobType, payloadJSON, nextRunAt)
	if err != nil {
		// Check if error is due to database unavailability
		if isDatabaseUnavailable(err) {
			return fmt.Errorf("%w: %v", ErrQueueUnavailable, err)
		}
		return fmt.Errorf("failed to enqueue job: %w", err)
	}

	return nil
}

// Dequeue retrieves the next available job from the queue
func (q *DBQueue) Dequeue(ctx context.Context) (*Job, error) {
	// Use SELECT FOR UPDATE SKIP LOCKED for concurrent workers
	query := `
		UPDATE background_jobs
		SET status = 'processing', attempts = attempts + 1
		WHERE id = (
			SELECT id FROM background_jobs
			WHERE status = 'pending' AND next_run_at <= NOW()
			ORDER BY next_run_at ASC
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, job_type, payload, created_at, next_run_at, attempts
	`

	var job Job
	var payloadJSON []byte

	err := q.db.QueryRowContext(ctx, query).Scan(
		&job.ID,
		&job.Type,
		&payloadJSON,
		&job.CreatedAt,
		&job.NextRunAt,
		&job.Attempts,
	)

	if err == sql.ErrNoRows {
		return nil, nil // No jobs available
	}

	if err != nil {
		return nil, fmt.Errorf("failed to dequeue job: %w", err)
	}

	// Unmarshal payload
	if err := json.Unmarshal(payloadJSON, &job.Payload); err != nil {
		return nil, fmt.Errorf("failed to unmarshal job payload: %w", err)
	}

	return &job, nil
}

// Complete marks a job as successfully completed
func (q *DBQueue) Complete(ctx context.Context, jobID int64) error {
	query := `
		UPDATE background_jobs
		SET status = 'completed', completed_at = NOW()
		WHERE id = $1
	`

	result, err := q.db.ExecContext(ctx, query, jobID)
	if err != nil {
		return fmt.Errorf("failed to complete job: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("job %d not found", jobID)
	}

	return nil
}

// Retry reschedules a job for retry with a delay
func (q *DBQueue) Retry(ctx context.Context, jobID int64, delay time.Duration) error {
	nextRunAt := time.Now().Add(delay)

	query := `
		UPDATE background_jobs
		SET status = 'pending', next_run_at = $2
		WHERE id = $1
	`

	result, err := q.db.ExecContext(ctx, query, jobID, nextRunAt)
	if err != nil {
		return fmt.Errorf("failed to retry job: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("job %d not found", jobID)
	}

	return nil
}

// Fail marks a job as permanently failed
func (q *DBQueue) Fail(ctx context.Context, jobID int64, errorMsg string) error {
	query := `
		UPDATE background_jobs
		SET status = 'failed', error_message = $2, failed_at = NOW()
		WHERE id = $1
	`

	result, err := q.db.ExecContext(ctx, query, jobID, errorMsg)
	if err != nil {
		return fmt.Errorf("failed to mark job as failed: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("job %d not found", jobID)
	}

	return nil
}

// HealthCheck verifies the queue is operational
func (q *DBQueue) HealthCheck(ctx context.Context) error {
	query := `SELECT 1`
	var result int

	err := q.db.QueryRowContext(ctx, query).Scan(&result)
	if err != nil {
		return fmt.Errorf("queue health check failed: %w", err)
	}

	return nil
}

// Close closes the queue connection
func (q *DBQueue) Close() error {
	// DBQueue doesn't own the database connection, so nothing to close
	return nil
}

// isDatabaseUnavailable checks if an error indicates database unavailability
func isDatabaseUnavailable(err error) bool {
	if err == nil {
		return false
	}
	// Check for common database unavailability errors
	errStr := err.Error()
	return sql.ErrConnDone == err ||
		sql.ErrTxDone == err ||
		containsAny(errStr, []string{
			"connection refused",
			"connection reset",
			"broken pipe",
			"no such host",
			"timeout",
			"too many connections",
		})
}

// containsAny checks if a string contains any of the given substrings
func containsAny(s string, substrs []string) bool {
	for _, substr := range substrs {
		if len(s) >= len(substr) {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
		}
	}
	return false
}
