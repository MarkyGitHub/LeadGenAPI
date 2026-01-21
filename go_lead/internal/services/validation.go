package services

import (
	"fmt"
	"log"
	"regexp"

	"github.com/checkfox/go_lead/internal/models"
)

// ValidationResult represents the outcome of validating a lead
type ValidationResult struct {
	Valid           bool
	RejectionReason *models.RejectionReason
	Errors          []string
}

// Validator provides lead validation functionality
type Validator struct {
	zipcodePattern *regexp.Regexp
}

// NewValidator creates a new Validator instance
func NewValidator() *Validator {
	// Compile the zipcode pattern: ^66\d{3}$
	pattern := regexp.MustCompile(`^66\d{3}$`)
	
	return &Validator{
		zipcodePattern: pattern,
	}
}

// ValidateLead validates a lead against all business rules
// Requirements: 2.1, 2.2, 2.3, 2.4, 2.6
func (v *Validator) ValidateLead(rawPayload models.JSONB) *ValidationResult {
	result := &ValidationResult{
		Valid:  true,
		Errors: []string{},
	}
	
	// Rule 1: Validate zipcode (Requirement 2.1)
	if !v.validateZipcode(rawPayload) {
		log.Printf("[VALIDATION] Zipcode validation failed for payload")
		result.Valid = false
		reason := models.RejectionReasonZipNotValid
		result.RejectionReason = &reason
		result.Errors = append(result.Errors, "zipcode must match pattern ^66\\d{3}$")
		return result // Return immediately on first failure
	}
	log.Printf("[VALIDATION] Zipcode validation passed")
	
	// Rule 2: Validate homeowner status (Requirement 2.2)
	if !v.validateHomeowner(rawPayload) {
		log.Printf("[VALIDATION] Homeowner validation failed for payload")
		result.Valid = false
		reason := models.RejectionReasonNotHomeowner
		result.RejectionReason = &reason
		result.Errors = append(result.Errors, "house.is_owner must be exactly true")
		return result // Return immediately on first failure
	}
	log.Printf("[VALIDATION] Homeowner validation passed")
	
	log.Printf("[VALIDATION] All validation rules passed")
	return result
}

// validateZipcode checks if the zipcode matches the required pattern ^66\d{3}$
// Requirement 2.1
func (v *Validator) validateZipcode(payload models.JSONB) bool {
	// Extract zipcode from payload
	zipcode, ok := payload["zipcode"]
	if !ok {
		log.Printf("[VALIDATION] Zipcode field missing from payload")
		return false
	}
	
	// Convert to string
	zipcodeStr, ok := zipcode.(string)
	if !ok {
		log.Printf("[VALIDATION] Zipcode field is not a string: %T", zipcode)
		return false
	}
	
	// Check pattern
	matches := v.zipcodePattern.MatchString(zipcodeStr)
	log.Printf("[VALIDATION] Zipcode '%s' pattern match result: %v", zipcodeStr, matches)
	
	return matches
}

// validateHomeowner checks if house.is_owner is exactly true
// Requirement 2.2
func (v *Validator) validateHomeowner(payload models.JSONB) bool {
	// Extract house object
	house, ok := payload["house"]
	if !ok {
		log.Printf("[VALIDATION] House field missing from payload")
		return false
	}
	
	// Convert to map
	houseMap, ok := house.(map[string]interface{})
	if !ok {
		log.Printf("[VALIDATION] House field is not an object: %T", house)
		return false
	}
	
	// Extract is_owner field
	isOwner, ok := houseMap["is_owner"]
	if !ok {
		log.Printf("[VALIDATION] house.is_owner field missing from payload")
		return false
	}
	
	// Check if it's exactly true (boolean true)
	isOwnerBool, ok := isOwner.(bool)
	if !ok {
		log.Printf("[VALIDATION] house.is_owner is not a boolean: %T", isOwner)
		return false
	}
	
	log.Printf("[VALIDATION] house.is_owner value: %v", isOwnerBool)
	
	return isOwnerBool == true
}

// ValidateAndGetReason is a convenience method that validates and returns the rejection reason
func (v *Validator) ValidateAndGetReason(rawPayload models.JSONB) (bool, *models.RejectionReason, error) {
	result := v.ValidateLead(rawPayload)
	
	if !result.Valid {
		if result.RejectionReason != nil {
			return false, result.RejectionReason, fmt.Errorf("validation failed: %v", result.Errors)
		}
		return false, nil, fmt.Errorf("validation failed: %v", result.Errors)
	}
	
	return true, nil, nil
}
