package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// JSONB is a custom type for PostgreSQL JSONB columns
type JSONB map[string]interface{}

// Value implements the driver.Valuer interface for JSONB
func (j JSONB) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// Scan implements the sql.Scanner interface for JSONB
func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("failed to unmarshal JSONB value: %v", value)
	}
	
	var result map[string]interface{}
	if err := json.Unmarshal(bytes, &result); err != nil {
		return err
	}
	
	*j = result
	return nil
}

// InboundLead represents a lead received via webhook
type InboundLead struct {
	ID                 int64      `json:"id" db:"id"`
	ReceivedAt         time.Time  `json:"received_at" db:"received_at"`
	RawPayload         JSONB      `json:"raw_payload" db:"raw_payload"`
	SourceHeaders      JSONB      `json:"source_headers,omitempty" db:"source_headers"`
	Status             LeadStatus `json:"status" db:"status"`
	RejectionReason    *string    `json:"rejection_reason,omitempty" db:"rejection_reason"`
	NormalizedPayload  JSONB      `json:"normalized_payload,omitempty" db:"normalized_payload"`
	CustomerPayload    JSONB      `json:"customer_payload,omitempty" db:"customer_payload"`
	PayloadHash        *string    `json:"payload_hash,omitempty" db:"payload_hash"`
	CreatedAt          time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at" db:"updated_at"`
}

// CanTransitionTo checks if the lead can transition from its current status to the target status
func (l *InboundLead) CanTransitionTo(target LeadStatus) bool {
	// Terminal states cannot transition
	if l.Status.IsTerminal() {
		return false
	}
	
	switch l.Status {
	case LeadStatusReceived:
		// RECEIVED can transition to REJECTED or READY
		return target == LeadStatusRejected || target == LeadStatusReady
		
	case LeadStatusReady:
		// READY can transition to DELIVERED, FAILED, or PERMANENTLY_FAILED
		return target == LeadStatusDelivered || target == LeadStatusFailed || target == LeadStatusPermanentlyFailed
		
	case LeadStatusFailed:
		// FAILED can transition to DELIVERED or PERMANENTLY_FAILED (after retries)
		return target == LeadStatusDelivered || target == LeadStatusPermanentlyFailed
		
	default:
		return false
	}
}

// TransitionTo attempts to transition the lead to a new status
// Returns an error if the transition is not allowed
func (l *InboundLead) TransitionTo(target LeadStatus) error {
	if !l.CanTransitionTo(target) {
		return fmt.Errorf("invalid status transition from %s to %s", l.Status, target)
	}
	
	l.Status = target
	l.UpdatedAt = time.Now()
	return nil
}

// MarkRejected marks the lead as rejected with the given reason
func (l *InboundLead) MarkRejected(reason RejectionReason) error {
	if err := l.TransitionTo(LeadStatusRejected); err != nil {
		return err
	}
	
	reasonStr := reason.String()
	l.RejectionReason = &reasonStr
	return nil
}

// MarkReady marks the lead as ready for delivery
func (l *InboundLead) MarkReady() error {
	return l.TransitionTo(LeadStatusReady)
}

// MarkDelivered marks the lead as successfully delivered
func (l *InboundLead) MarkDelivered() error {
	return l.TransitionTo(LeadStatusDelivered)
}

// MarkFailed marks the lead as failed (retriable)
func (l *InboundLead) MarkFailed() error {
	return l.TransitionTo(LeadStatusFailed)
}

// MarkPermanentlyFailed marks the lead as permanently failed
func (l *InboundLead) MarkPermanentlyFailed() error {
	return l.TransitionTo(LeadStatusPermanentlyFailed)
}

// DeliveryAttempt represents a single attempt to deliver a lead to the Customer API
type DeliveryAttempt struct {
	ID             int64      `json:"id" db:"id"`
	LeadID         int64      `json:"lead_id" db:"lead_id"`
	AttemptNo      int        `json:"attempt_no" db:"attempt_no"`
	RequestedAt    time.Time  `json:"requested_at" db:"requested_at"`
	ResponseStatus *int       `json:"response_status,omitempty" db:"response_status"`
	ResponseBody   *string    `json:"response_body,omitempty" db:"response_body"`
	ErrorMessage   *string    `json:"error_message,omitempty" db:"error_message"`
	Success        bool       `json:"success" db:"success"`
	CreatedAt      time.Time  `json:"created_at" db:"created_at"`
}

// NewDeliveryAttempt creates a new delivery attempt for a lead
func NewDeliveryAttempt(leadID int64, attemptNo int) *DeliveryAttempt {
	now := time.Now()
	return &DeliveryAttempt{
		LeadID:      leadID,
		AttemptNo:   attemptNo,
		RequestedAt: now,
		Success:     false,
		CreatedAt:   now,
	}
}

// MarkSuccess marks the delivery attempt as successful
func (d *DeliveryAttempt) MarkSuccess(statusCode int, responseBody string) {
	d.Success = true
	d.ResponseStatus = &statusCode
	d.ResponseBody = &responseBody
}

// MarkFailure marks the delivery attempt as failed
func (d *DeliveryAttempt) MarkFailure(statusCode *int, errorMessage string) {
	d.Success = false
	d.ResponseStatus = statusCode
	d.ErrorMessage = &errorMessage
}
