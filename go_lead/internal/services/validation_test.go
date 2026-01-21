package services

import (
	"testing"

	"github.com/checkfox/go_lead/internal/models"
)

// Test missing fields
func TestValidateLead_MissingZipcode(t *testing.T) {
	validator := NewValidator()
	
	payload := models.JSONB{
		// zipcode is missing
		"house": map[string]interface{}{
			"is_owner": true,
		},
	}
	
	result := validator.ValidateLead(payload)
	
	if result.Valid {
		t.Error("Expected validation to fail when zipcode is missing")
	}
	
	if result.RejectionReason == nil || *result.RejectionReason != models.RejectionReasonZipNotValid {
		t.Errorf("Expected rejection reason ZIP_NOT_66XXX, got %v", result.RejectionReason)
	}
}

func TestValidateLead_MissingHouse(t *testing.T) {
	validator := NewValidator()
	
	payload := models.JSONB{
		"zipcode": "66123",
		// house is missing
	}
	
	result := validator.ValidateLead(payload)
	
	if result.Valid {
		t.Error("Expected validation to fail when house is missing")
	}
	
	if result.RejectionReason == nil || *result.RejectionReason != models.RejectionReasonNotHomeowner {
		t.Errorf("Expected rejection reason NOT_HOMEOWNER, got %v", result.RejectionReason)
	}
}

func TestValidateLead_MissingIsOwner(t *testing.T) {
	validator := NewValidator()
	
	payload := models.JSONB{
		"zipcode": "66123",
		"house": map[string]interface{}{
			// is_owner is missing
		},
	}
	
	result := validator.ValidateLead(payload)
	
	if result.Valid {
		t.Error("Expected validation to fail when is_owner is missing")
	}
	
	if result.RejectionReason == nil || *result.RejectionReason != models.RejectionReasonNotHomeowner {
		t.Errorf("Expected rejection reason NOT_HOMEOWNER, got %v", result.RejectionReason)
	}
}

// Test null values
func TestValidateLead_NullZipcode(t *testing.T) {
	validator := NewValidator()
	
	payload := models.JSONB{
		"zipcode": nil,
		"house": map[string]interface{}{
			"is_owner": true,
		},
	}
	
	result := validator.ValidateLead(payload)
	
	if result.Valid {
		t.Error("Expected validation to fail when zipcode is null")
	}
}

func TestValidateLead_NullHouse(t *testing.T) {
	validator := NewValidator()
	
	payload := models.JSONB{
		"zipcode": "66123",
		"house":   nil,
	}
	
	result := validator.ValidateLead(payload)
	
	if result.Valid {
		t.Error("Expected validation to fail when house is null")
	}
}

func TestValidateLead_NullIsOwner(t *testing.T) {
	validator := NewValidator()
	
	payload := models.JSONB{
		"zipcode": "66123",
		"house": map[string]interface{}{
			"is_owner": nil,
		},
	}
	
	result := validator.ValidateLead(payload)
	
	if result.Valid {
		t.Error("Expected validation to fail when is_owner is null")
	}
}

// Test type mismatches
func TestValidateLead_ZipcodeNotString(t *testing.T) {
	validator := NewValidator()
	
	payload := models.JSONB{
		"zipcode": 66123, // integer instead of string
		"house": map[string]interface{}{
			"is_owner": true,
		},
	}
	
	result := validator.ValidateLead(payload)
	
	if result.Valid {
		t.Error("Expected validation to fail when zipcode is not a string")
	}
}

func TestValidateLead_HouseNotObject(t *testing.T) {
	validator := NewValidator()
	
	payload := models.JSONB{
		"zipcode": "66123",
		"house":   "not an object", // string instead of object
	}
	
	result := validator.ValidateLead(payload)
	
	if result.Valid {
		t.Error("Expected validation to fail when house is not an object")
	}
}

func TestValidateLead_IsOwnerNotBoolean(t *testing.T) {
	validator := NewValidator()
	
	payload := models.JSONB{
		"zipcode": "66123",
		"house": map[string]interface{}{
			"is_owner": "true", // string instead of boolean
		},
	}
	
	result := validator.ValidateLead(payload)
	
	if result.Valid {
		t.Error("Expected validation to fail when is_owner is not a boolean")
	}
}

func TestValidateLead_IsOwnerInteger(t *testing.T) {
	validator := NewValidator()
	
	payload := models.JSONB{
		"zipcode": "66123",
		"house": map[string]interface{}{
			"is_owner": 1, // integer instead of boolean
		},
	}
	
	result := validator.ValidateLead(payload)
	
	if result.Valid {
		t.Error("Expected validation to fail when is_owner is an integer")
	}
}

// Test boundary conditions for zipcode pattern
func TestValidateLead_ZipcodeBoundaries(t *testing.T) {
	validator := NewValidator()
	
	testCases := []struct {
		name     string
		zipcode  string
		expected bool
	}{
		{"Valid 66000", "66000", true},
		{"Valid 66999", "66999", true},
		{"Valid 66500", "66500", true},
		{"Invalid 65999", "65999", false},
		{"Invalid 67000", "67000", false},
		{"Invalid too short", "6612", false},
		{"Invalid too long", "661234", false},
		{"Invalid empty", "", false},
		{"Invalid with letters", "66a12", false},
		{"Invalid with spaces", "66 12", false},
		{"Invalid with special chars", "66-12", false},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			payload := models.JSONB{
				"zipcode": tc.zipcode,
				"house": map[string]interface{}{
					"is_owner": true,
				},
			}
			
			result := validator.ValidateLead(payload)
			
			if result.Valid != tc.expected {
				t.Errorf("For zipcode %q, expected valid=%v, got valid=%v", 
					tc.zipcode, tc.expected, result.Valid)
			}
		})
	}
}

// Test successful validation
func TestValidateLead_Success(t *testing.T) {
	validator := NewValidator()
	
	payload := models.JSONB{
		"zipcode": "66123",
		"house": map[string]interface{}{
			"is_owner": true,
		},
	}
	
	result := validator.ValidateLead(payload)
	
	if !result.Valid {
		t.Errorf("Expected validation to pass, got errors: %v", result.Errors)
	}
	
	if result.RejectionReason != nil {
		t.Errorf("Expected no rejection reason, got %v", *result.RejectionReason)
	}
}

// Test ValidateAndGetReason convenience method
func TestValidateAndGetReason_Success(t *testing.T) {
	validator := NewValidator()
	
	payload := models.JSONB{
		"zipcode": "66123",
		"house": map[string]interface{}{
			"is_owner": true,
		},
	}
	
	valid, reason, err := validator.ValidateAndGetReason(payload)
	
	if !valid {
		t.Error("Expected validation to pass")
	}
	
	if reason != nil {
		t.Errorf("Expected no rejection reason, got %v", *reason)
	}
	
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestValidateAndGetReason_Failure(t *testing.T) {
	validator := NewValidator()
	
	payload := models.JSONB{
		"zipcode": "12345",
		"house": map[string]interface{}{
			"is_owner": true,
		},
	}
	
	valid, reason, err := validator.ValidateAndGetReason(payload)
	
	if valid {
		t.Error("Expected validation to fail")
	}
	
	if reason == nil || *reason != models.RejectionReasonZipNotValid {
		t.Errorf("Expected rejection reason ZIP_NOT_66XXX, got %v", reason)
	}
	
	if err == nil {
		t.Error("Expected error to be returned")
	}
}
