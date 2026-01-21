package services

import (
	"testing"

	"github.com/checkfox/go_lead/internal/config"
	"github.com/checkfox/go_lead/internal/models"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: lead-service-go, Property 4: Missing core fields cause failure
// Validates: Requirements 3.5
//
// Property: For any normalized payload, if a required Core Customer Field (phone, product.name) is missing,
// the mapping should fail. Conversely, if all required fields are present, mapping should succeed.
func TestProperty_MissingCoreFieldsCauseFailure(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	
	properties := gopter.NewProperties(parameters)
	
	// Create a test configuration with minimal attribute mapping
	cfg := &config.Config{
		CustomerAPI: config.CustomerAPIConfig{
			ProductName: "test_product",
		},
		AttributeMapping: config.AttributeMappingConfig{
			Mapping: map[string]config.AttributeDefinition{
				"phone": {
					Type:     "text",
					Required: true,
				},
				"email": {
					Type:     "text",
					Required: false,
				},
			},
		},
	}
	
	mapper := NewMapper(cfg)
	
	// Property 4a: Missing phone field should cause failure
	properties.Property("missing phone field always causes mapping failure", prop.ForAll(
		func(email string) bool {
			payload := models.JSONB{
				"email": email,
				// phone is missing
			}
			
			result := mapper.MapToCustomerFormat(payload)
			
			// Should fail because phone is missing
			return !result.Success && len(result.Errors) > 0
		},
		gen.AlphaString(),
	))
	
	// Property 4b: Empty phone field should cause failure
	properties.Property("empty phone field always causes mapping failure", prop.ForAll(
		func(email string) bool {
			payload := models.JSONB{
				"phone": "", // Empty phone
				"email": email,
			}
			
			result := mapper.MapToCustomerFormat(payload)
			
			// Should fail because phone is empty
			return !result.Success && len(result.Errors) > 0
		},
		gen.AlphaString(),
	))
	
	// Property 4c: Nil phone field should cause failure
	properties.Property("nil phone field always causes mapping failure", prop.ForAll(
		func(email string) bool {
			payload := models.JSONB{
				"phone": nil, // Nil phone
				"email": email,
			}
			
			result := mapper.MapToCustomerFormat(payload)
			
			// Should fail because phone is nil
			return !result.Success && len(result.Errors) > 0
		},
		gen.AlphaString(),
	))
	
	// Property 4d: Valid phone field should succeed (product.name is set from config)
	properties.Property("valid phone field with product.name from config always succeeds", prop.ForAll(
		func(phone string, email string) bool {
			// Ensure phone is non-empty
			if phone == "" {
				phone = "1234567890"
			}
			
			payload := models.JSONB{
				"phone": phone,
				"email": email,
			}
			
			result := mapper.MapToCustomerFormat(payload)
			
			// Should succeed because phone is present and product.name is set from config
			if !result.Success {
				return false
			}
			
			// Verify phone is in customer payload
			if result.CustomerPayload["phone"] != phone {
				return false
			}
			
			// Verify product.name is set from configuration
			product, ok := result.CustomerPayload["product"].(map[string]interface{})
			if !ok {
				return false
			}
			
			productName, ok := product["name"].(string)
			if !ok || productName != "test_product" {
				return false
			}
			
			return true
		},
		gen.AlphaString().SuchThat(func(s string) bool { return s != "" }),
		gen.AlphaString(),
	))
	
	properties.TestingRun(t)
}

// Feature: lead-service-go, Property 5: Invalid optional attributes are omitted
// Validates: Requirements 3.6
//
// Property: For any normalized payload with invalid optional attributes, the mapping should succeed
// but omit the invalid attributes. The omitted attributes should be tracked for diagnostics.
func TestProperty_InvalidOptionalAttributesAreOmitted(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	
	properties := gopter.NewProperties(parameters)
	
	// Create a test configuration with various attribute types
	cfg := &config.Config{
		CustomerAPI: config.CustomerAPIConfig{
			ProductName: "test_product",
		},
		AttributeMapping: config.AttributeMappingConfig{
			Mapping: map[string]config.AttributeDefinition{
				"phone": {
					Type:     "text",
					Required: true,
				},
				"email": {
					Type:     "text",
					Required: false,
				},
				"roof_type": {
					Type:     "dropdown",
					Required: false,
					Options:  []string{"flat", "pitched", "mixed"},
				},
				"roof_area": {
					Type:     "range",
					Required: false,
					Min:      floatPtr(0),
					Max:      floatPtr(1000),
				},
			},
		},
	}
	
	mapper := NewMapper(cfg)
	
	// Property 5a: Invalid dropdown value should be omitted
	properties.Property("invalid dropdown value is omitted but mapping succeeds", prop.ForAll(
		func(phone string, invalidRoofType string) bool {
			// Ensure phone is valid
			if phone == "" {
				phone = "1234567890"
			}
			
			// Ensure roof_type is invalid (not in allowed options)
			if invalidRoofType == "flat" || invalidRoofType == "pitched" || invalidRoofType == "mixed" {
				invalidRoofType = "invalid_type"
			}
			
			payload := models.JSONB{
				"phone":     phone,
				"roof_type": invalidRoofType,
			}
			
			result := mapper.MapToCustomerFormat(payload)
			
			// Should succeed because roof_type is optional
			if !result.Success {
				return false
			}
			
			// roof_type should be omitted
			if _, exists := result.CustomerPayload["roof_type"]; exists {
				return false
			}
			
			// roof_type should be in omitted attributes list
			found := false
			for _, attr := range result.OmittedAttributes {
				if attr == "roof_type" {
					found = true
					break
				}
			}
			
			return found
		},
		gen.AlphaString().SuchThat(func(s string) bool { return s != "" }),
		gen.AlphaString(),
	))
	
	// Property 5b: Out-of-range value should be omitted
	properties.Property("out-of-range value is omitted but mapping succeeds", prop.ForAll(
		func(phone string, roofArea float64) bool {
			// Ensure phone is valid
			if phone == "" {
				phone = "1234567890"
			}
			
			// Ensure roof_area is out of range (not 0-1000)
			if roofArea >= 0 && roofArea <= 1000 {
				roofArea = 1500 // Out of range
			}
			
			payload := models.JSONB{
				"phone":     phone,
				"roof_area": roofArea,
			}
			
			result := mapper.MapToCustomerFormat(payload)
			
			// Should succeed because roof_area is optional
			if !result.Success {
				return false
			}
			
			// roof_area should be omitted
			if _, exists := result.CustomerPayload["roof_area"]; exists {
				return false
			}
			
			// roof_area should be in omitted attributes list
			found := false
			for _, attr := range result.OmittedAttributes {
				if attr == "roof_area" {
					found = true
					break
				}
			}
			
			return found
		},
		gen.AlphaString().SuchThat(func(s string) bool { return s != "" }),
		gen.Float64Range(-1000, 2000),
	))
	
	// Property 5c: Valid optional attributes should be included
	properties.Property("valid optional attributes are included in customer payload", prop.ForAll(
		func(phone string, email string, roofType string) bool {
			// Ensure phone is valid
			if phone == "" {
				phone = "1234567890"
			}
			
			// Ensure email is non-empty (valid text attribute)
			if email == "" {
				email = "test@example.com"
			}
			
			// Ensure roof_type is valid
			validRoofTypes := []string{"flat", "pitched", "mixed"}
			roofType = validRoofTypes[0] // Use first valid option
			
			payload := models.JSONB{
				"phone":     phone,
				"email":     email,
				"roof_type": roofType,
			}
			
			result := mapper.MapToCustomerFormat(payload)
			
			// Should succeed
			if !result.Success {
				return false
			}
			
			// Valid optional attributes should be included
			if result.CustomerPayload["email"] != email {
				return false
			}
			
			if result.CustomerPayload["roof_type"] != roofType {
				return false
			}
			
			// No attributes should be omitted
			return len(result.OmittedAttributes) == 0
		},
		gen.AlphaString().SuchThat(func(s string) bool { return s != "" }),
		gen.AlphaString().SuchThat(func(s string) bool { return s != "" }),
		gen.OneConstOf("flat", "pitched", "mixed"),
	))
	
	// Property 5d: Multiple invalid optional attributes should all be omitted
	properties.Property("multiple invalid optional attributes are all omitted", prop.ForAll(
		func(phone string) bool {
			// Ensure phone is valid
			if phone == "" {
				phone = "1234567890"
			}
			
			payload := models.JSONB{
				"phone":     phone,
				"roof_type": "invalid_type",  // Invalid dropdown
				"roof_area": 2000.0,          // Out of range
			}
			
			result := mapper.MapToCustomerFormat(payload)
			
			// Should succeed because both are optional
			if !result.Success {
				return false
			}
			
			// Both should be omitted
			if _, exists := result.CustomerPayload["roof_type"]; exists {
				return false
			}
			if _, exists := result.CustomerPayload["roof_area"]; exists {
				return false
			}
			
			// Both should be in omitted attributes list
			return len(result.OmittedAttributes) == 2
		},
		gen.AlphaString().SuchThat(func(s string) bool { return s != "" }),
	))
	
	properties.TestingRun(t)
}

// Helper function to create float64 pointer
func floatPtr(f float64) *float64 {
	return &f
}
