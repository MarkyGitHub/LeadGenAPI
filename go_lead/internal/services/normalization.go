package services

import (
	"regexp"
	"strings"

	"github.com/checkfox/go_lead/internal/models"
)

// Normalizer provides data normalization functionality
type Normalizer struct {
	phonePattern *regexp.Regexp
}

// NewNormalizer creates a new Normalizer instance
func NewNormalizer() *Normalizer {
	// Pattern to extract digits from phone numbers
	phonePattern := regexp.MustCompile(`\d+`)
	
	return &Normalizer{
		phonePattern: phonePattern,
	}
}

// NormalizeLead normalizes all fields in a lead payload
// Requirements: 3.3, 3.4
func (n *Normalizer) NormalizeLead(rawPayload models.JSONB) models.JSONB {
	normalized := make(models.JSONB)
	
	// Recursively normalize all fields
	for key, value := range rawPayload {
		normalized[key] = n.normalizeValue(value)
	}
	
	return normalized
}

// normalizeValue normalizes a single value based on its type
func (n *Normalizer) normalizeValue(value interface{}) interface{} {
	if value == nil {
		return nil
	}
	
	switch v := value.(type) {
	case string:
		return n.normalizeString(v)
	case map[string]interface{}:
		// Recursively normalize nested objects
		normalized := make(map[string]interface{})
		for key, val := range v {
			normalized[key] = n.normalizeValue(val)
		}
		return normalized
	case []interface{}:
		// Recursively normalize arrays
		normalized := make([]interface{}, len(v))
		for i, val := range v {
			normalized[i] = n.normalizeValue(val)
		}
		return normalized
	default:
		// Return other types as-is (numbers, booleans, etc.)
		return value
	}
}

// normalizeString normalizes a string value
// Applies trimming and whitespace cleanup
func (n *Normalizer) normalizeString(s string) string {
	// Trim leading and trailing whitespace
	s = strings.TrimSpace(s)
	
	// Collapse multiple spaces into single space
	s = regexp.MustCompile(`\s+`).ReplaceAllString(s, " ")
	
	return s
}

// NormalizeEmail normalizes an email address
// Converts to lowercase and trims whitespace
// Requirement: 3.3
func (n *Normalizer) NormalizeEmail(email string) string {
	// Trim whitespace
	email = strings.TrimSpace(email)
	
	// Convert to lowercase
	email = strings.ToLower(email)
	
	return email
}

// NormalizePhone standardizes a phone number
// Extracts only digits and formats consistently
// Requirement: 3.3
func (n *Normalizer) NormalizePhone(phone string) string {
	// Extract all digits from the phone number
	digits := n.phonePattern.FindAllString(phone, -1)
	
	// Join all digits together
	normalized := strings.Join(digits, "")
	
	return normalized
}

// NormalizeBooleanString converts string representations of booleans to actual booleans
// Handles common string representations: "true", "false", "1", "0", "yes", "no"
// Requirement: 3.3
func (n *Normalizer) NormalizeBooleanString(value interface{}) interface{} {
	// If already a boolean, return as-is
	if b, ok := value.(bool); ok {
		return b
	}
	
	// If it's a string, try to convert
	if s, ok := value.(string); ok {
		s = strings.TrimSpace(strings.ToLower(s))
		
		switch s {
		case "true", "1", "yes", "y":
			return true
		case "false", "0", "no", "n":
			return false
		default:
			// Return original value if not recognized
			return value
		}
	}
	
	// Return original value for other types
	return value
}

// TrimString trims whitespace from a string
// Requirement: 3.3
func (n *Normalizer) TrimString(s string) string {
	return strings.TrimSpace(s)
}

// NormalizeLeadWithFieldMapping normalizes a lead with specific field handling
// Applies special normalization rules for known fields like email and phone
// Requirement: 3.3, 3.4
func (n *Normalizer) NormalizeLeadWithFieldMapping(rawPayload models.JSONB) models.JSONB {
	normalized := make(models.JSONB)
	
	for key, value := range rawPayload {
		switch key {
		case "email":
			// Special handling for email fields
			if email, ok := value.(string); ok {
				normalized[key] = n.NormalizeEmail(email)
			} else {
				normalized[key] = value
			}
			
		case "phone", "phone_number", "telephone":
			// Special handling for phone fields
			if phone, ok := value.(string); ok {
				normalized[key] = n.NormalizePhone(phone)
			} else {
				normalized[key] = value
			}
			
		default:
			// Default normalization for other fields
			normalized[key] = n.normalizeValue(value)
		}
	}
	
	return normalized
}
