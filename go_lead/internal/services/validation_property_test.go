package services

import (
	"fmt"
	"testing"

	"github.com/checkfox/go_lead/internal/models"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: lead-service-go, Property 1: Zipcode validation consistency
// Validates: Requirements 2.1
//
// Property: For any zipcode string, the validation result should be consistent:
// - Zipcodes matching ^66\d{3}$ should always pass validation
// - Zipcodes not matching the pattern should always fail validation
func TestProperty_ZipcodeValidationConsistency(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	
	properties := gopter.NewProperties(parameters)
	
	validator := NewValidator()
	
	// Property 1a: Valid zipcodes (66XXX format) should always pass
	properties.Property("valid zipcodes matching ^66\\d{3}$ always pass", prop.ForAll(
		func(suffix int) bool {
			// Generate valid zipcode: 66 + three digits (000-999)
			zipcode := fmt.Sprintf("66%03d", suffix)
			
			payload := models.JSONB{
				"zipcode": zipcode,
				"house": map[string]interface{}{
					"is_owner": true,
				},
			}
			
			result := validator.ValidateLead(payload)
			
			// Valid zipcode should pass validation
			return result.Valid
		},
		gen.IntRange(0, 999),
	))
	
	// Property 1b: Invalid zipcodes should always fail
	invalidZipcodeGen := gen.OneConstOf(
		"12345",      // Wrong prefix
		"67000",      // Wrong prefix (67 instead of 66)
		"6600",       // Too short
		"660000",     // Too long
		"66abc",      // Non-numeric
		"",           // Empty
		"abc",        // Completely invalid
		"55555",      // Different prefix
		"77777",      // Different prefix
	)
	
	properties.Property("invalid zipcodes not matching ^66\\d{3}$ always fail", prop.ForAll(
		func(zipcode string) bool {
			payload := models.JSONB{
				"zipcode": zipcode,
				"house": map[string]interface{}{
					"is_owner": true,
				},
			}
			
			result := validator.ValidateLead(payload)
			
			// Invalid zipcode should fail validation
			return !result.Valid && result.RejectionReason != nil && 
				   *result.RejectionReason == models.RejectionReasonZipNotValid
		},
		invalidZipcodeGen,
	))
	
	properties.TestingRun(t)
}

// Feature: lead-service-go, Property 2: Homeowner validation consistency
// Validates: Requirements 2.2
//
// Property: For any house.is_owner value, the validation result should be consistent:
// - When house.is_owner is exactly boolean true, validation should pass
// - When house.is_owner is any other value (false, missing, wrong type), validation should fail
func TestProperty_HomeownerValidationConsistency(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	
	properties := gopter.NewProperties(parameters)
	
	validator := NewValidator()
	
	// Property 2a: When house.is_owner is exactly true, validation should pass
	properties.Property("house.is_owner=true always passes validation", prop.ForAll(
		func(suffix int) bool {
			// Use valid zipcode to isolate homeowner validation
			validZipcode := fmt.Sprintf("66%03d", suffix)
			
			payload := models.JSONB{
				"zipcode": validZipcode,
				"house": map[string]interface{}{
					"is_owner": true, // Exactly boolean true
				},
			}
			
			result := validator.ValidateLead(payload)
			
			// Should pass validation
			return result.Valid
		},
		gen.IntRange(0, 999),
	))
	
	// Property 2b: When house.is_owner is false, validation should fail
	properties.Property("house.is_owner=false always fails validation", prop.ForAll(
		func(suffix int) bool {
			validZipcode := fmt.Sprintf("66%03d", suffix)
			
			payload := models.JSONB{
				"zipcode": validZipcode,
				"house": map[string]interface{}{
					"is_owner": false, // Boolean false
				},
			}
			
			result := validator.ValidateLead(payload)
			
			// Should fail validation with homeowner rejection reason
			return !result.Valid && result.RejectionReason != nil && 
				   *result.RejectionReason == models.RejectionReasonNotHomeowner
		},
		gen.IntRange(0, 999),
	))
	
	// Property 2c: When house.is_owner is missing, validation should fail
	properties.Property("missing house.is_owner always fails validation", prop.ForAll(
		func(suffix int) bool {
			validZipcode := fmt.Sprintf("66%03d", suffix)
			
			payload := models.JSONB{
				"zipcode": validZipcode,
				"house": map[string]interface{}{
					// is_owner field is missing
				},
			}
			
			result := validator.ValidateLead(payload)
			
			// Should fail validation
			return !result.Valid && result.RejectionReason != nil && 
				   *result.RejectionReason == models.RejectionReasonNotHomeowner
		},
		gen.IntRange(0, 999),
	))
	
	// Property 2d: When house.is_owner is a non-boolean type, validation should fail
	properties.Property("non-boolean house.is_owner always fails validation", prop.ForAll(
		func(suffix int, isOwnerValue string) bool {
			validZipcode := fmt.Sprintf("66%03d", suffix)
			
			payload := models.JSONB{
				"zipcode": validZipcode,
				"house": map[string]interface{}{
					"is_owner": isOwnerValue, // String instead of boolean
				},
			}
			
			result := validator.ValidateLead(payload)
			
			// Should fail validation
			return !result.Valid && result.RejectionReason != nil && 
				   *result.RejectionReason == models.RejectionReasonNotHomeowner
		},
		gen.IntRange(0, 999),
		gen.OneConstOf("true", "false", "yes", "no", "1", "0", ""),
	))
	
	// Property 2e: When house field is missing entirely, validation should fail
	properties.Property("missing house field always fails validation", prop.ForAll(
		func(suffix int) bool {
			validZipcode := fmt.Sprintf("66%03d", suffix)
			
			payload := models.JSONB{
				"zipcode": validZipcode,
				// house field is missing entirely
			}
			
			result := validator.ValidateLead(payload)
			
			// Should fail validation
			return !result.Valid && result.RejectionReason != nil && 
				   *result.RejectionReason == models.RejectionReasonNotHomeowner
		},
		gen.IntRange(0, 999),
	))
	
	properties.TestingRun(t)
}
