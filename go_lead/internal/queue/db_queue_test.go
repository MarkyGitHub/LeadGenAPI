package queue

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

// setupTestDB creates a test database connection
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
	_, err := db.Exec("DELETE FROM background_jobs")
	if err != nil {
		t.Logf("Warning: failed to clean background_jobs table: %v", err)
	}
}

func TestNewDBQueue(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer db.Close()
	defer cleanupTestData(t, db)

	queue, err := NewDBQueue(db)
	if err != nil {
		t.Fatalf("Failed to create queue: %v", err)
	}

	if queue == nil {
		t.Error("Expected queue to be created")
	}

	// Test with nil database
	_, err = NewDBQueue(nil)
	if err == nil {
		t.Error("Expected error when creating queue with nil database")
	}
}

func TestDBQueue_EnqueueAndDequeue(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer db.Close()
	defer cleanupTestData(t, db)

	queue, err := NewDBQueue(db)
	if err != nil {
		t.Fatalf("Failed to create queue: %v", err)
	}

	ctx := context.Background()

	// Enqueue a job
	payload := NewJobPayload(123)
	err = queue.Enqueue(ctx, "process_lead", payload)
	if err != nil {
		t.Fatalf("Failed to enqueue job: %v", err)
	}

	// Dequeue the job
	job, err := queue.Dequeue(ctx)
	if err != nil {
		t.Fatalf("Failed to dequeue job: %v", err)
	}

	if job == nil {
		t.Fatal("Expected job to be dequeued")
	}

	if job.Type != "process_lead" {
		t.Errorf("Expected job type 'process_lead', got '%s'", job.Type)
	}

	leadID, ok := GetLeadID(job.Payload)
	if !ok {
		t.Error("Expected lead_id in payload")
	}

	if leadID != 123 {
		t.Errorf("Expected lead_id 123, got %d", leadID)
	}

	if job.Attempts != 1 {
		t.Errorf("Expected attempts to be 1, got %d", job.Attempts)
	}
}

func TestDBQueue_DequeueEmpty(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer db.Close()
	defer cleanupTestData(t, db)

	queue, err := NewDBQueue(db)
	if err != nil {
		t.Fatalf("Failed to create queue: %v", err)
	}

	ctx := context.Background()

	// Dequeue from empty queue
	job, err := queue.Dequeue(ctx)
	if err != nil {
		t.Fatalf("Expected no error when dequeuing from empty queue, got: %v", err)
	}

	if job != nil {
		t.Error("Expected nil job when queue is empty")
	}
}

func TestDBQueue_Complete(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer db.Close()
	defer cleanupTestData(t, db)

	queue, err := NewDBQueue(db)
	if err != nil {
		t.Fatalf("Failed to create queue: %v", err)
	}

	ctx := context.Background()

	// Enqueue and dequeue a job
	payload := NewJobPayload(456)
	err = queue.Enqueue(ctx, "process_lead", payload)
	if err != nil {
		t.Fatalf("Failed to enqueue job: %v", err)
	}

	job, err := queue.Dequeue(ctx)
	if err != nil {
		t.Fatalf("Failed to dequeue job: %v", err)
	}

	// Complete the job
	err = queue.Complete(ctx, job.ID)
	if err != nil {
		t.Fatalf("Failed to complete job: %v", err)
	}

	// Verify job is marked as completed
	var status string
	err = db.QueryRow("SELECT status FROM background_jobs WHERE id = $1", job.ID).Scan(&status)
	if err != nil {
		t.Fatalf("Failed to query job status: %v", err)
	}

	if status != "completed" {
		t.Errorf("Expected status 'completed', got '%s'", status)
	}

	// Try to complete non-existent job
	err = queue.Complete(ctx, 999999)
	if err == nil {
		t.Error("Expected error when completing non-existent job")
	}
}

func TestDBQueue_Retry(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer db.Close()
	defer cleanupTestData(t, db)

	queue, err := NewDBQueue(db)
	if err != nil {
		t.Fatalf("Failed to create queue: %v", err)
	}

	ctx := context.Background()

	// Enqueue and dequeue a job
	payload := NewJobPayload(789)
	err = queue.Enqueue(ctx, "process_lead", payload)
	if err != nil {
		t.Fatalf("Failed to enqueue job: %v", err)
	}

	job, err := queue.Dequeue(ctx)
	if err != nil {
		t.Fatalf("Failed to dequeue job: %v", err)
	}

	// Retry the job with delay
	delay := 30 * time.Second
	err = queue.Retry(ctx, job.ID, delay)
	if err != nil {
		t.Fatalf("Failed to retry job: %v", err)
	}

	// Verify job is back to pending status
	var status string
	var nextRunAt time.Time
	err = db.QueryRow("SELECT status, next_run_at FROM background_jobs WHERE id = $1", job.ID).Scan(&status, &nextRunAt)
	if err != nil {
		t.Fatalf("Failed to query job: %v", err)
	}

	if status != "pending" {
		t.Errorf("Expected status 'pending', got '%s'", status)
	}

	// Verify next_run_at is in the future
	if !nextRunAt.After(time.Now()) {
		t.Error("Expected next_run_at to be in the future")
	}

	// Try to retry non-existent job
	err = queue.Retry(ctx, 999999, delay)
	if err == nil {
		t.Error("Expected error when retrying non-existent job")
	}
}

func TestDBQueue_Fail(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer db.Close()
	defer cleanupTestData(t, db)

	queue, err := NewDBQueue(db)
	if err != nil {
		t.Fatalf("Failed to create queue: %v", err)
	}

	ctx := context.Background()

	// Enqueue and dequeue a job
	payload := NewJobPayload(101)
	err = queue.Enqueue(ctx, "process_lead", payload)
	if err != nil {
		t.Fatalf("Failed to enqueue job: %v", err)
	}

	job, err := queue.Dequeue(ctx)
	if err != nil {
		t.Fatalf("Failed to dequeue job: %v", err)
	}

	// Fail the job
	errorMsg := "test error message"
	err = queue.Fail(ctx, job.ID, errorMsg)
	if err != nil {
		t.Fatalf("Failed to mark job as failed: %v", err)
	}

	// Verify job is marked as failed
	var status string
	var storedError sql.NullString
	err = db.QueryRow("SELECT status, error_message FROM background_jobs WHERE id = $1", job.ID).Scan(&status, &storedError)
	if err != nil {
		t.Fatalf("Failed to query job: %v", err)
	}

	if status != "failed" {
		t.Errorf("Expected status 'failed', got '%s'", status)
	}

	if !storedError.Valid || storedError.String != errorMsg {
		t.Errorf("Expected error message '%s', got '%s'", errorMsg, storedError.String)
	}

	// Try to fail non-existent job
	err = queue.Fail(ctx, 999999, "error")
	if err == nil {
		t.Error("Expected error when failing non-existent job")
	}
}

func TestDBQueue_EnqueueWithDelay(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer db.Close()
	defer cleanupTestData(t, db)

	queue, err := NewDBQueue(db)
	if err != nil {
		t.Fatalf("Failed to create queue: %v", err)
	}

	ctx := context.Background()

	// Enqueue a job with delay
	payload := NewJobPayload(202)
	delay := 60 * time.Second
	err = queue.EnqueueWithDelay(ctx, "process_lead", payload, delay)
	if err != nil {
		t.Fatalf("Failed to enqueue job with delay: %v", err)
	}

	// Try to dequeue immediately - should get nothing
	job, err := queue.Dequeue(ctx)
	if err != nil {
		t.Fatalf("Failed to dequeue: %v", err)
	}

	if job != nil {
		t.Error("Expected no job to be available immediately when enqueued with delay")
	}

	// Verify the job exists with future next_run_at
	var nextRunAt time.Time
	err = db.QueryRow("SELECT next_run_at FROM background_jobs WHERE job_type = 'process_lead' AND status = 'pending'").Scan(&nextRunAt)
	if err != nil {
		t.Fatalf("Failed to query job: %v", err)
	}

	if !nextRunAt.After(time.Now()) {
		t.Error("Expected next_run_at to be in the future")
	}
}

func TestDBQueue_JobSerializationRoundTrip(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer db.Close()
	defer cleanupTestData(t, db)

	queue, err := NewDBQueue(db)
	if err != nil {
		t.Fatalf("Failed to create queue: %v", err)
	}

	ctx := context.Background()

	// Create a complex payload
	payload := map[string]interface{}{
		"lead_id": int64(303),
		"extra":   "data",
		"nested": map[string]interface{}{
			"key": "value",
		},
		"array": []interface{}{1, 2, 3},
	}

	// Enqueue
	err = queue.Enqueue(ctx, "process_lead", payload)
	if err != nil {
		t.Fatalf("Failed to enqueue job: %v", err)
	}

	// Dequeue
	job, err := queue.Dequeue(ctx)
	if err != nil {
		t.Fatalf("Failed to dequeue job: %v", err)
	}

	if job == nil {
		t.Fatal("Expected job to be dequeued")
	}

	// Verify payload round-trip
	leadID, ok := GetLeadID(job.Payload)
	if !ok {
		t.Error("Expected lead_id in payload")
	}

	if leadID != 303 {
		t.Errorf("Expected lead_id 303, got %d", leadID)
	}

	if job.Payload["extra"] != "data" {
		t.Errorf("Expected extra field 'data', got '%v'", job.Payload["extra"])
	}

	nested, ok := job.Payload["nested"].(map[string]interface{})
	if !ok {
		t.Error("Expected nested to be a map")
	} else if nested["key"] != "value" {
		t.Errorf("Expected nested.key 'value', got '%v'", nested["key"])
	}
}

func TestDBQueue_HealthCheck(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	queue, err := NewDBQueue(db)
	if err != nil {
		t.Fatalf("Failed to create queue: %v", err)
	}

	ctx := context.Background()

	// Health check should pass
	err = queue.HealthCheck(ctx)
	if err != nil {
		t.Errorf("Health check failed: %v", err)
	}
}

func TestDBQueue_ConcurrentDequeue(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer db.Close()
	defer cleanupTestData(t, db)

	queue, err := NewDBQueue(db)
	if err != nil {
		t.Fatalf("Failed to create queue: %v", err)
	}

	ctx := context.Background()

	// Enqueue multiple jobs
	for i := 1; i <= 5; i++ {
		payload := NewJobPayload(int64(i))
		err = queue.Enqueue(ctx, "process_lead", payload)
		if err != nil {
			t.Fatalf("Failed to enqueue job %d: %v", i, err)
		}
	}

	// Dequeue concurrently
	results := make(chan *Job, 5)
	errors := make(chan error, 5)

	for i := 0; i < 5; i++ {
		go func() {
			job, err := queue.Dequeue(ctx)
			if err != nil {
				errors <- err
				return
			}
			results <- job
		}()
	}

	// Collect results
	jobs := make([]*Job, 0, 5)
	for i := 0; i < 5; i++ {
		select {
		case job := <-results:
			if job != nil {
				jobs = append(jobs, job)
			}
		case err := <-errors:
			t.Errorf("Error during concurrent dequeue: %v", err)
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for concurrent dequeue")
		}
	}

	// Verify we got 5 unique jobs
	if len(jobs) != 5 {
		t.Errorf("Expected 5 jobs, got %d", len(jobs))
	}

	// Verify all jobs are unique
	seen := make(map[int64]bool)
	for _, job := range jobs {
		if seen[job.ID] {
			t.Errorf("Duplicate job ID: %d", job.ID)
		}
		seen[job.ID] = true
	}
}

func TestGetLeadID(t *testing.T) {
	tests := []struct {
		name      string
		payload   map[string]interface{}
		expected  int64
		shouldOK  bool
	}{
		{
			name:     "int64 lead_id",
			payload:  map[string]interface{}{"lead_id": int64(123)},
			expected: 123,
			shouldOK: true,
		},
		{
			name:     "float64 lead_id",
			payload:  map[string]interface{}{"lead_id": float64(456)},
			expected: 456,
			shouldOK: true,
		},
		{
			name:     "missing lead_id",
			payload:  map[string]interface{}{"other": "value"},
			expected: 0,
			shouldOK: false,
		},
		{
			name:     "string lead_id",
			payload:  map[string]interface{}{"lead_id": "789"},
			expected: 0,
			shouldOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			leadID, ok := GetLeadID(tt.payload)
			if ok != tt.shouldOK {
				t.Errorf("Expected ok=%v, got ok=%v", tt.shouldOK, ok)
			}
			if leadID != tt.expected {
				t.Errorf("Expected lead_id=%d, got lead_id=%d", tt.expected, leadID)
			}
		})
	}
}
