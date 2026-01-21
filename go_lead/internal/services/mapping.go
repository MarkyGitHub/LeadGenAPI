package services

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/checkfox/go_lead/internal/config"
	"github.com/checkfox/go_lead/internal/models"
)

// MappingResult represents the outcome of mapping a lead to customer format
type MappingResult struct {
	Success           bool
	CustomerPayload   models.JSONB
	OmittedAttributes []string
	Errors            []string
}

// Mapper provides lead mapping functionality with permissive attribute handling
type Mapper struct {
	attributeMapping map[string]config.AttributeDefinition
	productName      string
}

// NewMapper creates a new Mapper instance
func NewMapper(cfg *config.Config) *Mapper {
	productName := cfg.CustomerAPI.ProductName
	if productName == "" {
		productName = "default_product"
	}
	
	return &Mapper{
		attributeMapping: cfg.AttributeMapping.Mapping,
		productName:      productName,
	}
}

// MapToCustomerFormat maps a normalized lead payload to customer format
// Requirements: 3.1, 3.2, 3.5, 3.6, 3.8
func (m *Mapper) MapToCustomerFormat(normalizedPayload models.JSONB) *MappingResult {
	result := &MappingResult{
		Success:           true,
		CustomerPayload:   make(models.JSONB),
		OmittedAttributes: []string{},
		Errors:            []string{},
	}
	
	// Validate and set required Core Customer Fields
	// Requirement 3.5: phone is required
	phone, phoneOk := normalizedPayload["phone"]
	if !phoneOk || phone == nil || phone == "" {
		result.Success = false
		result.Errors = append(result.Errors, "missing required field: phone")
		log.Printf("[MAPPING] Missing required Core Customer Field: phone")
		return result
	}
	result.CustomerPayload["phone"] = phone
	log.Printf("[MAPPING] Set required field phone: %v", phone)
	
	// Requirement 3.8: product.name is required and set from configuration
	result.CustomerPayload["product"] = map[string]interface{}{
		"name": m.productName,
	}
	log.Printf("[MAPPING] Set required field product.name: %s", m.productName)
	
	// Process all other attributes with permissive validation
	for key, value := range normalizedPayload {
		// Skip already processed core fields
		if key == "phone" || key == "product" {
			continue
		}
		
		// Check if attribute has validation rules
		attrDef, hasRules := m.attributeMapping[key]
		
		if !hasRules {
			// No validation rules defined - include as-is
			result.CustomerPayload[key] = value
			log.Printf("[MAPPING] No validation rules for '%s', including as-is", key)
			continue
		}
		
		// Validate attribute according to its type
		valid, validatedValue := m.validateAttribute(key, value, attrDef)
		
		if valid {
			result.CustomerPayload[key] = validatedValue
			log.Printf("[MAPPING] Attribute '%s' validated successfully", key)
		} else {
			// Requirement 3.6: Omit invalid optional attributes
			if !attrDef.Required {
				result.OmittedAttributes = append(result.OmittedAttributes, key)
				log.Printf("[MAPPING] Omitting invalid optional attribute: %s", key)
			} else {
				// Required attribute is invalid - this is an error
				result.Success = false
				result.Errors = append(result.Errors, fmt.Sprintf("required attribute '%s' is invalid", key))
				log.Printf("[MAPPING] Required attribute '%s' is invalid", key)
			}
		}
	}
	
	if len(result.OmittedAttributes) > 0 {
		log.Printf("[MAPPING] Omitted %d invalid optional attributes: %v", 
			len(result.OmittedAttributes), result.OmittedAttributes)
	}
	
	return result
}

// validateAttribute validates a single attribute according to its type definition
// Returns (valid, validatedValue)
func (m *Mapper) validateAttribute(key string, value interface{}, def config.AttributeDefinition) (bool, interface{}) {
	if value == nil {
		log.Printf("[MAPPING] Attribute '%s' is nil", key)
		return false, nil
	}
	
	switch def.Type {
	case "text":
		return m.validateTextAttribute(key, value, def)
	case "dropdown":
		return m.validateDropdownAttribute(key, value, def)
	case "range":
		return m.validateRangeAttribute(key, value, def)
	default:
		log.Printf("[MAPPING] Unknown attribute type '%s' for '%s'", def.Type, key)
		return false, nil
	}
}

// validateTextAttribute validates a text attribute
func (m *Mapper) validateTextAttribute(key string, value interface{}, def config.AttributeDefinition) (bool, interface{}) {
	// Text attributes should be strings
	strValue, ok := value.(string)
	if !ok {
		log.Printf("[MAPPING] Text attribute '%s' is not a string: %T", key, value)
		return false, nil
	}
	
	// Empty strings are invalid
	if strings.TrimSpace(strValue) == "" {
		log.Printf("[MAPPING] Text attribute '%s' is empty", key)
		return false, nil
	}
	
	return true, strValue
}

// validateDropdownAttribute validates a dropdown attribute
func (m *Mapper) validateDropdownAttribute(key string, value interface{}, def config.AttributeDefinition) (bool, interface{}) {
	// Dropdown values should be strings
	strValue, ok := value.(string)
	if !ok {
		log.Printf("[MAPPING] Dropdown attribute '%s' is not a string: %T", key, value)
		return false, nil
	}
	
	// Check if value is in allowed options
	if len(def.Options) == 0 {
		log.Printf("[MAPPING] Dropdown attribute '%s' has no options defined", key)
		return true, strValue // No options means any value is valid
	}
	
	for _, option := range def.Options {
		if strValue == option {
			return true, strValue
		}
	}
	
	log.Printf("[MAPPING] Dropdown attribute '%s' value '%s' not in allowed options: %v", 
		key, strValue, def.Options)
	return false, nil
}

// validateRangeAttribute validates a range attribute
func (m *Mapper) validateRangeAttribute(key string, value interface{}, def config.AttributeDefinition) (bool, interface{}) {
	// Range attributes should be numeric
	var numValue float64
	
	switch v := value.(type) {
	case float64:
		numValue = v
	case float32:
		numValue = float64(v)
	case int:
		numValue = float64(v)
	case int64:
		numValue = float64(v)
	case string:
		// Try to parse string as number
		parsed, err := strconv.ParseFloat(v, 64)
		if err != nil {
			log.Printf("[MAPPING] Range attribute '%s' string value '%s' cannot be parsed as number", key, v)
			return false, nil
		}
		numValue = parsed
	default:
		log.Printf("[MAPPING] Range attribute '%s' is not numeric: %T", key, value)
		return false, nil
	}
	
	// Check min bound
	if def.Min != nil && numValue < *def.Min {
		log.Printf("[MAPPING] Range attribute '%s' value %f is below minimum %f", key, numValue, *def.Min)
		return false, nil
	}
	
	// Check max bound
	if def.Max != nil && numValue > *def.Max {
		log.Printf("[MAPPING] Range attribute '%s' value %f is above maximum %f", key, numValue, *def.Max)
		return false, nil
	}
	
	return true, numValue
}

// ValidateRequiredFields checks if all required Core Customer Fields are present
// Requirement 3.5
func (m *Mapper) ValidateRequiredFields(payload models.JSONB) error {
	// Check phone
	phone, ok := payload["phone"]
	if !ok || phone == nil || phone == "" {
		return fmt.Errorf("missing required Core Customer Field: phone")
	}
	
	// product.name is set from configuration, so we don't check the input payload
	
	return nil
}

// BuildCustomerPayload builds a customer payload from normalized data
// This is a convenience wrapper around MapToCustomerFormat
// Requirements: 3.2, 3.7, 3.8
func (m *Mapper) BuildCustomerPayload(normalizedPayload models.JSONB) (models.JSONB, []string, error) {
	result := m.MapToCustomerFormat(normalizedPayload)
	
	if !result.Success {
		return nil, result.OmittedAttributes, fmt.Errorf("mapping failed: %v", result.Errors)
	}
	
	return result.CustomerPayload, result.OmittedAttributes, nil
}

