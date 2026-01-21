package services

import (
	"reflect"
	"testing"

	"github.com/checkfox/go_lead/internal/models"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: lead-service-go, Property 3: Normalization idempotence
// Validates: Requirements 3.3
//
// Property: For any lead payload, normalizing it once and normalizing it again
// should produce the same result. That is, normalize(normalize(x)) == normalize(x)
// This ensures normalization is idempotent and can be safely applied multiple times.
func TestProperty_NormalizationIdempotence(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	
	properties := gopter.NewProperties(parameters)
	
	normalizer := NewNormalizer()
	
	// Property 3a: Normalizing a payload twice produces the same result as normalizing once
	properties.Property("normalize(normalize(payload)) == normalize(payload)", prop.ForAll(
		func(email string, phone string, name string, zipcode string) bool {
			// Create a test payload with various fields
			payload := models.JSONB{
				"email":   email,
				"phone":   phone,
				"name":    name,
				"zipcode": zipcode,
				"nested": map[string]interface{}{
					"field1": "  value with spaces  ",
					"field2": "UPPERCASE",
				},
			}
			
			// Normalize once
			normalized1 := normalizer.NormalizeLead(payload)
			
			// Normalize again
			normalized2 := normalizer.NormalizeLead(normalized1)
			
			// They should be equal (idempotent)
			return reflect.DeepEqual(normalized1, normalized2)
		},
		gen.AnyString(),
		gen.AnyString(),
		gen.AnyString(),
		gen.AnyString(),
	))
	
	// Property 3b: Email normalization is idempotent
	properties.Property("normalizing email twice produces same result", prop.ForAll(
		func(email string) bool {
			// Normalize once
			normalized1 := normalizer.NormalizeEmail(email)
			
			// Normalize again
			normalized2 := normalizer.NormalizeEmail(normalized1)
			
			// They should be equal
			return normalized1 == normalized2
		},
		gen.AnyString(),
	))
	
	// Property 3c: Phone normalization is idempotent
	properties.Property("normalizing phone twice produces same result", prop.ForAll(
		func(phone string) bool {
			// Normalize once
			normalized1 := normalizer.NormalizePhone(phone)
			
			// Normalize again
			normalized2 := normalizer.NormalizePhone(normalized1)
			
			// They should be equal
			return normalized1 == normalized2
		},
		gen.AnyString(),
	))
	
	// Property 3d: String trimming is idempotent
	properties.Property("trimming string twice produces same result", prop.ForAll(
		func(s string) bool {
			// Trim once
			trimmed1 := normalizer.TrimString(s)
			
			// Trim again
			trimmed2 := normalizer.TrimString(trimmed1)
			
			// They should be equal
			return trimmed1 == trimmed2
		},
		gen.AnyString(),
	))
	
	// Property 3e: Normalization with field mapping is idempotent
	properties.Property("normalize with field mapping is idempotent", prop.ForAll(
		func(email string, phone string) bool {
			payload := models.JSONB{
				"email": email,
				"phone": phone,
				"data": map[string]interface{}{
					"nested": "  value  ",
				},
			}
			
			// Normalize once
			normalized1 := normalizer.NormalizeLeadWithFieldMapping(payload)
			
			// Normalize again
			normalized2 := normalizer.NormalizeLeadWithFieldMapping(normalized1)
			
			// They should be equal
			return reflect.DeepEqual(normalized1, normalized2)
		},
		gen.AnyString(),
		gen.AnyString(),
	))
	
	properties.TestingRun(t)
}
