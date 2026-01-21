package services

import (
	"reflect"
	"testing"

	"github.com/checkfox/go_lead/internal/models"
)

// Test email normalization with various inputs
// Requirement: 3.3
func TestNormalizeEmail(t *testing.T) {
	normalizer := NewNormalizer()
	
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{"lowercase already", "test@example.com", "test@example.com"},
		{"uppercase to lowercase", "TEST@EXAMPLE.COM", "test@example.com"},
		{"mixed case", "TeSt@ExAmPlE.CoM", "test@example.com"},
		{"with leading spaces", "  test@example.com", "test@example.com"},
		{"with trailing spaces", "test@example.com  ", "test@example.com"},
		{"with both spaces", "  test@example.com  ", "test@example.com"},
		{"with tabs", "\ttest@example.com\t", "test@example.com"},
		{"empty string", "", ""},
		{"only spaces", "   ", ""},
		{"complex email", "  User.Name+Tag@EXAMPLE.COM  ", "user.name+tag@example.com"},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := normalizer.NormalizeEmail(tc.input)
			if result != tc.expected {
				t.Errorf("NormalizeEmail(%q) = %q, expected %q", tc.input, result, tc.expected)
			}
		})
	}
}

// Test phone number formatting
// Requirement: 3.3
func TestNormalizePhone(t *testing.T) {
	normalizer := NewNormalizer()
	
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{"digits only", "1234567890", "1234567890"},
		{"with dashes", "123-456-7890", "1234567890"},
		{"with spaces", "123 456 7890", "1234567890"},
		{"with parentheses", "(123) 456-7890", "1234567890"},
		{"with dots", "123.456.7890", "1234567890"},
		{"with plus prefix", "+1-123-456-7890", "11234567890"},
		{"international format", "+49 123 456 7890", "491234567890"},
		{"mixed separators", "+1 (123) 456-7890", "11234567890"},
		{"with extension", "123-456-7890 ext 123", "1234567890123"},
		{"empty string", "", ""},
		{"no digits", "abc-def-ghij", ""},
		{"only spaces", "   ", ""},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := normalizer.NormalizePhone(tc.input)
			if result != tc.expected {
				t.Errorf("NormalizePhone(%q) = %q, expected %q", tc.input, result, tc.expected)
			}
		})
	}
}

// Test whitespace handling
// Requirement: 3.3
func TestTrimString(t *testing.T) {
	normalizer := NewNormalizer()
	
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{"no whitespace", "hello", "hello"},
		{"leading spaces", "  hello", "hello"},
		{"trailing spaces", "hello  ", "hello"},
		{"both sides", "  hello  ", "hello"},
		{"tabs", "\thello\t", "hello"},
		{"newlines", "\nhello\n", "hello"},
		{"mixed whitespace", " \t\nhello \t\n", "hello"},
		{"empty string", "", ""},
		{"only spaces", "   ", ""},
		{"internal spaces preserved", "  hello world  ", "hello world"},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := normalizer.TrimString(tc.input)
			if result != tc.expected {
				t.Errorf("TrimString(%q) = %q, expected %q", tc.input, result, tc.expected)
			}
		})
	}
}

// Test boolean string conversion
// Requirement: 3.3
func TestNormalizeBooleanString(t *testing.T) {
	normalizer := NewNormalizer()
	
	testCases := []struct {
		name     string
		input    interface{}
		expected interface{}
	}{
		// Already boolean
		{"boolean true", true, true},
		{"boolean false", false, false},
		
		// String representations of true
		{"string true", "true", true},
		{"string TRUE", "TRUE", true},
		{"string 1", "1", true},
		{"string yes", "yes", true},
		{"string YES", "YES", true},
		{"string y", "y", true},
		{"string Y", "Y", true},
		
		// String representations of false
		{"string false", "false", false},
		{"string FALSE", "FALSE", false},
		{"string 0", "0", false},
		{"string no", "no", false},
		{"string NO", "NO", false},
		{"string n", "n", false},
		{"string N", "N", false},
		
		// Unrecognized strings (returned as-is)
		{"unrecognized string", "maybe", "maybe"},
		{"empty string", "", ""},
		{"random string", "abc", "abc"},
		
		// Other types (returned as-is)
		{"integer", 42, 42},
		{"float", 3.14, 3.14},
		{"nil", nil, nil},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := normalizer.NormalizeBooleanString(tc.input)
			if !reflect.DeepEqual(result, tc.expected) {
				t.Errorf("NormalizeBooleanString(%v) = %v, expected %v", tc.input, result, tc.expected)
			}
		})
	}
}

// Test normalizing a complete lead payload
// Requirement: 3.3
func TestNormalizeLead(t *testing.T) {
	normalizer := NewNormalizer()
	
	input := models.JSONB{
		"name":    "  John Doe  ",
		"email":   "  JOHN@EXAMPLE.COM  ",
		"zipcode": "  66123  ",
		"nested": map[string]interface{}{
			"field1": "  value1  ",
			"field2": "  VALUE2  ",
		},
		"array": []interface{}{
			"  item1  ",
			"  item2  ",
		},
		"number": 42,
		"bool":   true,
	}
	
	result := normalizer.NormalizeLead(input)
	
	// Check string fields are trimmed
	if result["name"] != "John Doe" {
		t.Errorf("Expected name to be trimmed, got %q", result["name"])
	}
	
	if result["email"] != "JOHN@EXAMPLE.COM" {
		t.Errorf("Expected email to be trimmed (but not lowercased by NormalizeLead), got %q", result["email"])
	}
	
	if result["zipcode"] != "66123" {
		t.Errorf("Expected zipcode to be trimmed, got %q", result["zipcode"])
	}
	
	// Check nested object
	nested, ok := result["nested"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected nested to be a map")
	}
	
	if nested["field1"] != "value1" {
		t.Errorf("Expected nested field1 to be trimmed, got %q", nested["field1"])
	}
	
	if nested["field2"] != "VALUE2" {
		t.Errorf("Expected nested field2 to be trimmed, got %q", nested["field2"])
	}
	
	// Check array
	array, ok := result["array"].([]interface{})
	if !ok {
		t.Fatal("Expected array to be a slice")
	}
	
	if array[0] != "item1" {
		t.Errorf("Expected array[0] to be trimmed, got %q", array[0])
	}
	
	if array[1] != "item2" {
		t.Errorf("Expected array[1] to be trimmed, got %q", array[1])
	}
	
	// Check non-string types are preserved
	if result["number"] != 42 {
		t.Errorf("Expected number to be preserved, got %v", result["number"])
	}
	
	if result["bool"] != true {
		t.Errorf("Expected bool to be preserved, got %v", result["bool"])
	}
}

// Test normalizing with field mapping (special handling for email and phone)
// Requirement: 3.3, 3.4
func TestNormalizeLeadWithFieldMapping(t *testing.T) {
	normalizer := NewNormalizer()
	
	input := models.JSONB{
		"email":        "  TEST@EXAMPLE.COM  ",
		"phone":        "(123) 456-7890",
		"phone_number": "123-456-7890",
		"name":         "  John Doe  ",
		"other":        "  value  ",
	}
	
	result := normalizer.NormalizeLeadWithFieldMapping(input)
	
	// Email should be lowercased and trimmed
	if result["email"] != "test@example.com" {
		t.Errorf("Expected email to be normalized, got %q", result["email"])
	}
	
	// Phone should have only digits
	if result["phone"] != "1234567890" {
		t.Errorf("Expected phone to be normalized, got %q", result["phone"])
	}
	
	// Phone_number should also be normalized
	if result["phone_number"] != "1234567890" {
		t.Errorf("Expected phone_number to be normalized, got %q", result["phone_number"])
	}
	
	// Other fields should just be trimmed
	if result["name"] != "John Doe" {
		t.Errorf("Expected name to be trimmed, got %q", result["name"])
	}
	
	if result["other"] != "value" {
		t.Errorf("Expected other to be trimmed, got %q", result["other"])
	}
}

// Test normalizing strings with multiple spaces
// Requirement: 3.3
func TestNormalizeString_MultipleSpaces(t *testing.T) {
	normalizer := NewNormalizer()
	
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{"single space", "hello world", "hello world"},
		{"double space", "hello  world", "hello world"},
		{"multiple spaces", "hello    world", "hello world"},
		{"leading and internal", "  hello  world  ", "hello world"},
		{"tabs and spaces", "hello\t\tworld", "hello world"},
		{"newlines", "hello\n\nworld", "hello world"},
		{"mixed whitespace", "hello \t\n world", "hello world"},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Use normalizeString through normalizeValue
			result := normalizer.normalizeValue(tc.input)
			if result != tc.expected {
				t.Errorf("normalizeString(%q) = %q, expected %q", tc.input, result, tc.expected)
			}
		})
	}
}

// Test edge cases
func TestNormalizeLead_EdgeCases(t *testing.T) {
	normalizer := NewNormalizer()
	
	t.Run("empty payload", func(t *testing.T) {
		input := models.JSONB{}
		result := normalizer.NormalizeLead(input)
		
		if len(result) != 0 {
			t.Errorf("Expected empty result, got %v", result)
		}
	})
	
	t.Run("nil values", func(t *testing.T) {
		input := models.JSONB{
			"field1": nil,
			"field2": "value",
		}
		result := normalizer.NormalizeLead(input)
		
		if result["field1"] != nil {
			t.Errorf("Expected nil to be preserved, got %v", result["field1"])
		}
		
		if result["field2"] != "value" {
			t.Errorf("Expected field2 to be normalized, got %q", result["field2"])
		}
	})
	
	t.Run("deeply nested objects", func(t *testing.T) {
		input := models.JSONB{
			"level1": map[string]interface{}{
				"level2": map[string]interface{}{
					"level3": map[string]interface{}{
						"value": "  deep value  ",
					},
				},
			},
		}
		result := normalizer.NormalizeLead(input)
		
		level1 := result["level1"].(map[string]interface{})
		level2 := level1["level2"].(map[string]interface{})
		level3 := level2["level3"].(map[string]interface{})
		
		if level3["value"] != "deep value" {
			t.Errorf("Expected deeply nested value to be normalized, got %q", level3["value"])
		}
	})
}
