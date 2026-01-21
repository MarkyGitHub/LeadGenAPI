package logger

import (
	"context"
	"log/slog"
	"os"
	"time"
)

// ContextKey is the type for context keys used in logging
type ContextKey string

const (
	// LeadIDKey is the context key for lead_id
	LeadIDKey ContextKey = "lead_id"
	// CorrelationIDKey is the context key for correlation_id
	CorrelationIDKey ContextKey = "correlation_id"
)

var defaultLogger *slog.Logger

// Init initializes the global structured logger with JSON output
func Init() {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	defaultLogger = slog.New(handler)
	slog.SetDefault(defaultLogger)
}

// WithContext creates a logger with context values (lead_id, correlation_id)
func WithContext(ctx context.Context) *slog.Logger {
	logger := defaultLogger
	
	if leadID, ok := ctx.Value(LeadIDKey).(int64); ok {
		logger = logger.With("lead_id", leadID)
	}
	
	if correlationID, ok := ctx.Value(CorrelationIDKey).(string); ok {
		logger = logger.With("correlation_id", correlationID)
	}
	
	return logger
}

// Info logs an info message with context
func Info(ctx context.Context, msg string, args ...any) {
	WithContext(ctx).Info(msg, args...)
}

// Error logs an error message with context
func Error(ctx context.Context, msg string, args ...any) {
	WithContext(ctx).Error(msg, args...)
}

// Warn logs a warning message with context
func Warn(ctx context.Context, msg string, args ...any) {
	WithContext(ctx).Warn(msg, args...)
}

// Debug logs a debug message with context
func Debug(ctx context.Context, msg string, args ...any) {
	WithContext(ctx).Debug(msg, args...)
}

// LogStatusTransition logs a lead status transition
func LogStatusTransition(ctx context.Context, leadID int64, oldStatus, newStatus string) {
	logger := WithContext(ctx).With(
		"lead_id", leadID,
		"old_status", oldStatus,
		"new_status", newStatus,
		"timestamp", time.Now().UTC(),
	)
	logger.Info("Lead status transition")
}

// LogSlowOperation logs operations that exceed the threshold
func LogSlowOperation(ctx context.Context, operation string, duration time.Duration) {
	if duration > time.Second {
		logger := WithContext(ctx).With(
			"operation", operation,
			"duration_ms", duration.Milliseconds(),
		)
		logger.Warn("Slow operation detected")
	}
}

// LogError logs an error with stack trace information
func LogError(ctx context.Context, msg string, err error, args ...any) {
	logger := WithContext(ctx)
	allArgs := append([]any{"error", err.Error()}, args...)
	logger.Error(msg, allArgs...)
}
