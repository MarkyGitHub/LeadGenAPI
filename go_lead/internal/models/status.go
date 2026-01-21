package models

// LeadStatus represents the current state of a lead in the processing pipeline
type LeadStatus string

const (
	// LeadStatusReceived indicates the lead has been accepted via webhook and queued for processing
	LeadStatusReceived LeadStatus = "RECEIVED"
	
	// LeadStatusRejected indicates the lead failed business validation rules
	LeadStatusRejected LeadStatus = "REJECTED"
	
	// LeadStatusReady indicates the lead passed validation and transformation and is ready for delivery
	LeadStatusReady LeadStatus = "READY"
	
	// LeadStatusDelivered indicates the lead was successfully sent to the Customer API
	LeadStatusDelivered LeadStatus = "DELIVERED"
	
	// LeadStatusFailed indicates delivery attempt failed but may be retried
	LeadStatusFailed LeadStatus = "FAILED"
	
	// LeadStatusPermanentlyFailed indicates maximum retry attempts exhausted or non-retriable error occurred
	LeadStatusPermanentlyFailed LeadStatus = "PERMANENTLY_FAILED"
)

// IsValid checks if the status is a valid LeadStatus value
func (s LeadStatus) IsValid() bool {
	switch s {
	case LeadStatusReceived, LeadStatusRejected, LeadStatusReady, 
		LeadStatusDelivered, LeadStatusFailed, LeadStatusPermanentlyFailed:
		return true
	default:
		return false
	}
}

// IsTerminal returns true if the status represents a terminal state
func (s LeadStatus) IsTerminal() bool {
	return s == LeadStatusRejected || s == LeadStatusDelivered || s == LeadStatusPermanentlyFailed
}

// RejectionReason represents specific reasons why a lead was rejected during validation
type RejectionReason string

const (
	// RejectionReasonZipNotValid indicates the zipcode does not match the required pattern ^66\d{3}$
	RejectionReasonZipNotValid RejectionReason = "ZIP_NOT_66XXX"
	
	// RejectionReasonNotHomeowner indicates house.is_owner is not exactly true
	RejectionReasonNotHomeowner RejectionReason = "NOT_HOMEOWNER"
	
	// RejectionReasonMissingRequiredField indicates a required field is missing from the payload
	RejectionReasonMissingRequiredField RejectionReason = "MISSING_REQUIRED_FIELD"
)

// String returns the string representation of the rejection reason
func (r RejectionReason) String() string {
	return string(r)
}
