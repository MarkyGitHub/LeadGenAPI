package models

import (
	"fmt"
)

// ValidationError represents an error that occurred during lead validation
type ValidationError struct {
	Field  string
	Reason RejectionReason
	Detail string
}

func (e *ValidationError) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("validation error on field '%s': %s (%s)", e.Field, e.Reason, e.Detail)
	}
	return fmt.Sprintf("validation error on field '%s': %s", e.Field, e.Reason)
}

// NewValidationError creates a new ValidationError
func NewValidationError(field string, reason RejectionReason, detail string) *ValidationError {
	return &ValidationError{
		Field:  field,
		Reason: reason,
		Detail: detail,
	}
}

// TransformationError represents an error that occurred during lead transformation
type TransformationError struct {
	Stage   string // e.g., "normalization", "mapping"
	Field   string
	Message string
	Err     error
}

func (e *TransformationError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("transformation error in %s stage for field '%s': %s (caused by: %v)", 
			e.Stage, e.Field, e.Message, e.Err)
	}
	return fmt.Sprintf("transformation error in %s stage for field '%s': %s", 
		e.Stage, e.Field, e.Message)
}

func (e *TransformationError) Unwrap() error {
	return e.Err
}

// NewTransformationError creates a new TransformationError
func NewTransformationError(stage, field, message string, err error) *TransformationError {
	return &TransformationError{
		Stage:   stage,
		Field:   field,
		Message: message,
		Err:     err,
	}
}

// DeliveryError represents an error that occurred during delivery to the Customer API
type DeliveryError struct {
	StatusCode int
	Message    string
	Retriable  bool
	Err        error
}

func (e *DeliveryError) Error() string {
	retriableStr := "non-retriable"
	if e.Retriable {
		retriableStr = "retriable"
	}
	
	if e.StatusCode > 0 {
		if e.Err != nil {
			return fmt.Sprintf("delivery error (%s): HTTP %d - %s (caused by: %v)", 
				retriableStr, e.StatusCode, e.Message, e.Err)
		}
		return fmt.Sprintf("delivery error (%s): HTTP %d - %s", 
			retriableStr, e.StatusCode, e.Message)
	}
	
	if e.Err != nil {
		return fmt.Sprintf("delivery error (%s): %s (caused by: %v)", 
			retriableStr, e.Message, e.Err)
	}
	return fmt.Sprintf("delivery error (%s): %s", retriableStr, e.Message)
}

func (e *DeliveryError) Unwrap() error {
	return e.Err
}

// IsRetriable returns true if the delivery error should trigger a retry
func (e *DeliveryError) IsRetriable() bool {
	return e.Retriable
}

// NewDeliveryError creates a new DeliveryError
func NewDeliveryError(statusCode int, message string, retriable bool, err error) *DeliveryError {
	return &DeliveryError{
		StatusCode: statusCode,
		Message:    message,
		Retriable:  retriable,
		Err:        err,
	}
}

// MissingCoreFieldError represents an error when a required core customer field is missing
type MissingCoreFieldError struct {
	Field string
}

func (e *MissingCoreFieldError) Error() string {
	return fmt.Sprintf("missing required core customer field: %s", e.Field)
}

// NewMissingCoreFieldError creates a new MissingCoreFieldError
func NewMissingCoreFieldError(field string) *MissingCoreFieldError {
	return &MissingCoreFieldError{
		Field: field,
	}
}
