package services

import (
	"testing"

	"github.com/checkfox/go_lead/internal/config"
	"github.com/checkfox/go_lead/internal/models"
)

// Test text attribute validation
func TestValidateTextAttribute(t *testing.T) {
	cfg := &config.Config{
		CustomerAPI: config.CustomerAPIConfig{
			ProductName: "test_product",
		},
		AttributeMapping: config.AttributeMappingConfig{
			Mapping: map[string]config.AttributeDefinition{
				"email": {
					Type:     "text",
					Required: false,
				},
			},
		},
	}
	
	mapper := NewMapper(cfg)
	
	tests := []struct {
		name      string
		value     interface{}
		wantValid bool
	}{
		{"valid string", "test@example.com", true},
		{"empty string", "", false},
		{"whitespace only", "   ", false},
		{"non-string", 123, false},
		{"nil value", nil, false},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, _ := mapper.validateTextAttribute("email", tt.value, cfg.AttributeMapping.Mapping["email"])
			if valid != tt.wantValid {
				t.Errorf("validateTextAttribute() = %v, want %v", valid, tt.wantValid)
			}
		})
	}
}

// Test dropdown attribute validation
func TestValidateDropdownAttribute(t *testing.T) {
	cfg := &config.Config{
		CustomerAPI: config.CustomerAPIConfig{
			ProductName: "test_product",
		},
		AttributeMapping: config.AttributeMappingConfig{
			Mapping: map[string]config.AttributeDefinition{
				"roof_type": {
					Type:     "dropdown",
					Required: false,
					Options:  []string{"flat", "pitched", "mixed"},
				},
			},
		},
	}
	
	mapper := NewMapper(cfg)
	
	tests := []struct {
		name      string
		value     interface{}
		wantValid bool
	}{
		{"valid option flat", "flat", true},
		{"valid option pitched", "pitched", true},
		{"valid option mixed", "mixed", true},
		{"invalid option", "invalid", false},
		{"non-string", 123, false},
		{"nil value", nil, false},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, _ := mapper.validateDropdownAttribute("roof_type", tt.value, cfg.AttributeMapping.Mapping["roof_type"])
			if valid != tt.wantValid {
				t.Errorf("validateDropdownAttribute() = %v, want %v", valid, tt.wantValid)
			}
		})
	}
}

// Test range attribute validation
func TestValidateRangeAttribute(t *testing.T) {
	min := 0.0
	max := 1000.0
	
	cfg := &config.Config{
		CustomerAPI: config.CustomerAPIConfig{
			ProductName: "test_product",
		},
		AttributeMapping: config.AttributeMappingConfig{
			Mapping: map[string]config.AttributeDefinition{
				"roof_area": {
					Type:     "range",
					Required: false,
					Min:      &min,
					Max:      &max,
				},
			},
		},
	}
	
	mapper := NewMapper(cfg)
	
	tests := []struct {
		name      string
		value     interface{}
		wantValid bool
	}{
		{"valid int", 500, true},
		{"valid float", 500.5, true},
		{"valid at min", 0, true},
		{"valid at max", 1000, true},
		{"below min", -1, false},
		{"above max", 1001, false},
		{"valid string number", "500", true},
		{"invalid string", "abc", false},
		{"nil value", nil, false},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, _ := mapper.validateRangeAttribute("roof_area", tt.value, cfg.AttributeMapping.Mapping["roof_area"])
			if valid != tt.wantValid {
				t.Errorf("validateRangeAttribute() = %v, want %v", valid, tt.wantValid)
			}
		})
	}
}

// Test missing required fields handling
func TestMissingRequiredFields(t *testing.T) {
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
			},
		},
	}
	
	mapper := NewMapper(cfg)
	
	tests := []struct {
		name        string
		payload     models.JSONB
		wantSuccess bool
	}{
		{
			name: "missing phone",
			payload: models.JSONB{
				"email": "test@example.com",
			},
			wantSuccess: false,
		},
		{
			name: "empty phone",
			payload: models.JSONB{
				"phone": "",
			},
			wantSuccess: false,
		},
		{
			name: "nil phone",
			payload: models.JSONB{
				"phone": nil,
			},
			wantSuccess: false,
		},
		{
			name: "valid phone",
			payload: models.JSONB{
				"phone": "1234567890",
			},
			wantSuccess: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapper.MapToCustomerFormat(tt.payload)
			if result.Success != tt.wantSuccess {
				t.Errorf("MapToCustomerFormat() success = %v, want %v", result.Success, tt.wantSuccess)
			}
		})
	}
}

// Test invalid optional attribute omission
func TestInvalidOptionalAttributeOmission(t *testing.T) {
	min := 0.0
	max := 1000.0
	
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
					Min:      &min,
					Max:      &max,
				},
			},
		},
	}
	
	mapper := NewMapper(cfg)
	
	tests := []struct {
		name              string
		payload           models.JSONB
		wantSuccess       bool
		wantOmittedCount  int
		wantOmittedAttrs  []string
	}{
		{
			name: "invalid dropdown omitted",
			payload: models.JSONB{
				"phone":     "1234567890",
				"roof_type": "invalid_type",
			},
			wantSuccess:      true,
			wantOmittedCount: 1,
			wantOmittedAttrs: []string{"roof_type"},
		},
		{
			name: "out of range value omitted",
			payload: models.JSONB{
				"phone":     "1234567890",
				"roof_area": 2000.0,
			},
			wantSuccess:      true,
			wantOmittedCount: 1,
			wantOmittedAttrs: []string{"roof_area"},
		},
		{
			name: "empty text omitted",
			payload: models.JSONB{
				"phone": "1234567890",
				"email": "",
			},
			wantSuccess:      true,
			wantOmittedCount: 1,
			wantOmittedAttrs: []string{"email"},
		},
		{
			name: "multiple invalid attributes omitted",
			payload: models.JSONB{
				"phone":     "1234567890",
				"email":     "",
				"roof_type": "invalid",
				"roof_area": 2000.0,
			},
			wantSuccess:      true,
			wantOmittedCount: 3,
		},
		{
			name: "valid attributes included",
			payload: models.JSONB{
				"phone":     "1234567890",
				"email":     "test@example.com",
				"roof_type": "flat",
				"roof_area": 500.0,
			},
			wantSuccess:      true,
			wantOmittedCount: 0,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapper.MapToCustomerFormat(tt.payload)
			
			if result.Success != tt.wantSuccess {
				t.Errorf("MapToCustomerFormat() success = %v, want %v", result.Success, tt.wantSuccess)
			}
			
			if len(result.OmittedAttributes) != tt.wantOmittedCount {
				t.Errorf("MapToCustomerFormat() omitted count = %v, want %v", len(result.OmittedAttributes), tt.wantOmittedCount)
			}
			
			if tt.wantOmittedAttrs != nil {
				for _, attr := range tt.wantOmittedAttrs {
					found := false
					for _, omitted := range result.OmittedAttributes {
						if omitted == attr {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Expected attribute '%s' to be omitted, but it wasn't", attr)
					}
				}
			}
		})
	}
}

// Test product.name is set from configuration
func TestProductNameSetFromConfig(t *testing.T) {
	cfg := &config.Config{
		CustomerAPI: config.CustomerAPIConfig{
			ProductName: "solar_panel_installation",
		},
		AttributeMapping: config.AttributeMappingConfig{
			Mapping: map[string]config.AttributeDefinition{
				"phone": {
					Type:     "text",
					Required: true,
				},
			},
		},
	}
	
	mapper := NewMapper(cfg)
	
	payload := models.JSONB{
		"phone": "1234567890",
	}
	
	result := mapper.MapToCustomerFormat(payload)
	
	if !result.Success {
		t.Fatalf("MapToCustomerFormat() failed: %v", result.Errors)
	}
	
	product, ok := result.CustomerPayload["product"].(map[string]interface{})
	if !ok {
		t.Fatal("product field is not a map")
	}
	
	productName, ok := product["name"].(string)
	if !ok {
		t.Fatal("product.name is not a string")
	}
	
	if productName != "solar_panel_installation" {
		t.Errorf("product.name = %v, want %v", productName, "solar_panel_installation")
	}
}
