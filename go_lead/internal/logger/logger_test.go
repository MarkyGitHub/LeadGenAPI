package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"
)

// TestStructuredLogOutput tests that logs are output in JSON format
// Requirements: 8.1, 8.2
func TestStructuredLogOutput(t *testing.T) {
	// Create a buffer to capture log output
	var buf bytes.Buffer
	
	// Create a JSON handler that writes to the buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	logger := slog.New(handler)
	
	// Log a message
	logger.Info("test message", "key1", "value1", "key2", 42)
	
	// Parse the JSON output
	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("Failed to parse JSON log output: %v", err)
	}
	
	// Verify the log entry contains expected fields
	if logEntry["msg"] != "test message" {
		t.Errorf("Expected msg='test message', got %v", logEntry["msg"])
	}
	
	if logEntry["key1"] != "value1" {
		t.Errorf("Expected key1='value1', got %v", logEntry["key1"])
	}
	
	if logEntry["key2"] != float64(42) {
		t.Errorf("Expected key2=42, got %v", logEntry["key2"])
	}
	
	// Verify timestamp is present
	if _, ok := logEntry["time"]; !ok {
		t.Error("Expected 'time' field in log output")
	}
	
	// Verify level is present
	if logEntry["level"] != "INFO" {
		t.Errorf("Expected level='INFO', got %v", logEntry["level"])
	}
}

// TestCorrelationIDPropagation tests that correlation_id is propagated through context
// Requirements: 8.1
func TestCorrelationIDPropagation(t *testing.T) {
	// Create a buffer to capture log output
	var buf bytes.Buffer
	
	// Create a JSON handler that writes to the buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	defaultLogger = slog.New(handler)
	
	// Create context with correlation ID
	ctx := context.WithValue(context.Background(), CorrelationIDKey, "test-correlation-id")
	
	// Log with context
	Info(ctx, "test message with correlation")
	
	// Parse the JSON output
	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("Failed to parse JSON log output: %v", err)
	}
	
	// Verify correlation_id is present
	if logEntry["correlation_id"] != "test-correlation-id" {
		t.Errorf("Expected correlation_id='test-correlation-id', got %v", logEntry["correlation_id"])
	}
}

// TestLeadIDPropagation tests that lead_id is propagated through context
// Requirements: 8.1
func TestLeadIDPropagation(t *testing.T) {
	// Create a buffer to capture log output
	var buf bytes.Buffer
	
	// Create a JSON handler that writes to the buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	defaultLogger = slog.New(handler)
	
	// Create context with lead ID
	ctx := context.WithValue(context.Background(), LeadIDKey, int64(12345))
	
	// Log with context
	Info(ctx, "test message with lead_id")
	
	// Parse the JSON output
	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("Failed to parse JSON log output: %v", err)
	}
	
	// Verify lead_id is present
	if logEntry["lead_id"] != float64(12345) {
		t.Errorf("Expected lead_id=12345, got %v", logEntry["lead_id"])
	}
}

// TestStatusTransitionLogging tests that status transitions are logged correctly
// Requirements: 8.1
func TestStatusTransitionLogging(t *testing.T) {
	// Create a buffer to capture log output
	var buf bytes.Buffer
	
	// Create a JSON handler that writes to the buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	defaultLogger = slog.New(handler)
	
	// Create context with lead ID
	ctx := context.WithValue(context.Background(), LeadIDKey, int64(12345))
	
	// Log a status transition
	LogStatusTransition(ctx, 12345, "RECEIVED", "READY")
	
	// Parse the JSON output
	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("Failed to parse JSON log output: %v", err)
	}
	
	// Verify status transition fields
	if logEntry["msg"] != "Lead status transition" {
		t.Errorf("Expected msg='Lead status transition', got %v", logEntry["msg"])
	}
	
	if logEntry["lead_id"] != float64(12345) {
		t.Errorf("Expected lead_id=12345, got %v", logEntry["lead_id"])
	}
	
	if logEntry["old_status"] != "RECEIVED" {
		t.Errorf("Expected old_status='RECEIVED', got %v", logEntry["old_status"])
	}
	
	if logEntry["new_status"] != "READY" {
		t.Errorf("Expected new_status='READY', got %v", logEntry["new_status"])
	}
	
	// Verify timestamp is present
	if _, ok := logEntry["timestamp"]; !ok {
		t.Error("Expected 'timestamp' field in status transition log")
	}
}

// TestSlowOperationLogging tests that slow operations are logged
// Requirements: 8.6
func TestSlowOperationLogging(t *testing.T) {
	// Create a buffer to capture log output
	var buf bytes.Buffer
	
	// Create a JSON handler that writes to the buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	defaultLogger = slog.New(handler)
	
	ctx := context.Background()
	
	// Test with slow operation (> 1 second)
	buf.Reset()
	LogSlowOperation(ctx, "test_operation", 1500*time.Millisecond)
	
	// Should have logged a warning
	if buf.Len() == 0 {
		t.Error("Expected slow operation to be logged")
	}
	
	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("Failed to parse JSON log output: %v", err)
	}
	
	if logEntry["msg"] != "Slow operation detected" {
		t.Errorf("Expected msg='Slow operation detected', got %v", logEntry["msg"])
	}
	
	if logEntry["operation"] != "test_operation" {
		t.Errorf("Expected operation='test_operation', got %v", logEntry["operation"])
	}
	
	if logEntry["duration_ms"] != float64(1500) {
		t.Errorf("Expected duration_ms=1500, got %v", logEntry["duration_ms"])
	}
	
	if logEntry["level"] != "WARN" {
		t.Errorf("Expected level='WARN', got %v", logEntry["level"])
	}
	
	// Test with fast operation (< 1 second)
	buf.Reset()
	LogSlowOperation(ctx, "fast_operation", 500*time.Millisecond)
	
	// Should not have logged anything
	if buf.Len() > 0 {
		t.Error("Expected fast operation not to be logged")
	}
}

// TestErrorLogging tests that errors are logged with error details
// Requirements: 8.2
func TestErrorLogging(t *testing.T) {
	// Create a buffer to capture log output
	var buf bytes.Buffer
	
	// Create a JSON handler that writes to the buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	defaultLogger = slog.New(handler)
	
	ctx := context.Background()
	
	// Log an error
	testErr := &testError{msg: "test error message"}
	LogError(ctx, "Operation failed", testErr, "additional_key", "additional_value")
	
	// Parse the JSON output
	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("Failed to parse JSON log output: %v", err)
	}
	
	// Verify error fields
	if logEntry["msg"] != "Operation failed" {
		t.Errorf("Expected msg='Operation failed', got %v", logEntry["msg"])
	}
	
	if logEntry["error"] != "test error message" {
		t.Errorf("Expected error='test error message', got %v", logEntry["error"])
	}
	
	if logEntry["additional_key"] != "additional_value" {
		t.Errorf("Expected additional_key='additional_value', got %v", logEntry["additional_key"])
	}
	
	if logEntry["level"] != "ERROR" {
		t.Errorf("Expected level='ERROR', got %v", logEntry["level"])
	}
}

// testError is a simple error type for testing
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
