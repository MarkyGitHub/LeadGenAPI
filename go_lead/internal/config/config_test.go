package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoad_FromEnvironmentVariables(t *testing.T) {
	// Set up environment variables
	os.Setenv("DB_HOST", "testhost")
	os.Setenv("DB_PORT", "5433")
	os.Setenv("DB_USER", "testuser")
	os.Setenv("DB_PASSWORD", "testpass")
	os.Setenv("DB_NAME", "testdb")
	os.Setenv("API_PORT", "9090")
	os.Setenv("CUSTOMER_API_URL", "https://test.api.com")
	os.Setenv("CUSTOMER_API_TOKEN", "test_token")
	os.Setenv("CUSTOMER_PRODUCT_NAME", "test_product")
	os.Setenv("WORKER_POLL_INTERVAL", "10s")
	os.Setenv("MAX_RETRY_ATTEMPTS", "3")
	os.Setenv("ENABLE_AUTH", "true")
	os.Setenv("SHARED_SECRET", "test_secret")
	
	// Create temporary attribute mapping file
	tmpDir := t.TempDir()
	mappingFile := filepath.Join(tmpDir, "test_mapping.json")
	mappingContent := `{
		"phone": {"type": "text", "required": true},
		"email": {"type": "text", "required": false}
	}`
	if err := os.WriteFile(mappingFile, []byte(mappingContent), 0644); err != nil {
		t.Fatalf("Failed to create test mapping file: %v", err)
	}
	os.Setenv("ATTRIBUTE_MAPPING_FILE", mappingFile)
	
	defer func() {
		os.Unsetenv("DB_HOST")
		os.Unsetenv("DB_PORT")
		os.Unsetenv("DB_USER")
		os.Unsetenv("DB_PASSWORD")
		os.Unsetenv("DB_NAME")
		os.Unsetenv("API_PORT")
		os.Unsetenv("CUSTOMER_API_URL")
		os.Unsetenv("CUSTOMER_API_TOKEN")
		os.Unsetenv("CUSTOMER_PRODUCT_NAME")
		os.Unsetenv("WORKER_POLL_INTERVAL")
		os.Unsetenv("MAX_RETRY_ATTEMPTS")
		os.Unsetenv("ENABLE_AUTH")
		os.Unsetenv("SHARED_SECRET")
		os.Unsetenv("ATTRIBUTE_MAPPING_FILE")
	}()
	
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	
	// Verify database config
	if cfg.Database.Host != "testhost" {
		t.Errorf("Expected DB_HOST=testhost, got %s", cfg.Database.Host)
	}
	if cfg.Database.Port != "5433" {
		t.Errorf("Expected DB_PORT=5433, got %s", cfg.Database.Port)
	}
	if cfg.Database.User != "testuser" {
		t.Errorf("Expected DB_USER=testuser, got %s", cfg.Database.User)
	}
	
	// Verify API config
	if cfg.API.Port != "9090" {
		t.Errorf("Expected API_PORT=9090, got %s", cfg.API.Port)
	}
	
	// Verify Customer API config
	if cfg.CustomerAPI.URL != "https://test.api.com" {
		t.Errorf("Expected CUSTOMER_API_URL=https://test.api.com, got %s", cfg.CustomerAPI.URL)
	}
	if cfg.CustomerAPI.Token != "test_token" {
		t.Errorf("Expected CUSTOMER_API_TOKEN=test_token, got %s", cfg.CustomerAPI.Token)
	}
	
	// Verify Worker config
	if cfg.Worker.PollInterval != 10*time.Second {
		t.Errorf("Expected WORKER_POLL_INTERVAL=10s, got %v", cfg.Worker.PollInterval)
	}
	
	// Verify Retry config
	if cfg.Retry.MaxAttempts != 3 {
		t.Errorf("Expected MAX_RETRY_ATTEMPTS=3, got %d", cfg.Retry.MaxAttempts)
	}
	
	// Verify Auth config
	if !cfg.Auth.Enabled {
		t.Error("Expected ENABLE_AUTH=true")
	}
	if cfg.Auth.SharedSecret != "test_secret" {
		t.Errorf("Expected SHARED_SECRET=test_secret, got %s", cfg.Auth.SharedSecret)
	}
}

func TestLoad_DefaultValues(t *testing.T) {
	// Clear relevant environment variables
	os.Unsetenv("DB_HOST")
	os.Unsetenv("API_PORT")
	os.Unsetenv("WORKER_POLL_INTERVAL")
	os.Unsetenv("ENABLE_AUTH")
	
	// Set required fields
	os.Setenv("CUSTOMER_API_URL", "https://required.api.com")
	os.Setenv("CUSTOMER_API_TOKEN", "required_token")
	os.Setenv("CUSTOMER_PRODUCT_NAME", "required_product")
	
	// Create temporary attribute mapping file
	tmpDir := t.TempDir()
	mappingFile := filepath.Join(tmpDir, "test_mapping.json")
	mappingContent := `{"phone": {"type": "text", "required": true}}`
	if err := os.WriteFile(mappingFile, []byte(mappingContent), 0644); err != nil {
		t.Fatalf("Failed to create test mapping file: %v", err)
	}
	os.Setenv("ATTRIBUTE_MAPPING_FILE", mappingFile)
	
	defer func() {
		os.Unsetenv("CUSTOMER_API_URL")
		os.Unsetenv("CUSTOMER_API_TOKEN")
		os.Unsetenv("CUSTOMER_PRODUCT_NAME")
		os.Unsetenv("ATTRIBUTE_MAPPING_FILE")
	}()
	
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	
	// Verify default values
	if cfg.Database.Host != "localhost" {
		t.Errorf("Expected default DB_HOST=localhost, got %s", cfg.Database.Host)
	}
	if cfg.API.Port != "8080" {
		t.Errorf("Expected default API_PORT=8080, got %s", cfg.API.Port)
	}
	if cfg.Worker.PollInterval != 5*time.Second {
		t.Errorf("Expected default WORKER_POLL_INTERVAL=5s, got %v", cfg.Worker.PollInterval)
	}
	if cfg.Auth.Enabled {
		t.Error("Expected default ENABLE_AUTH=false")
	}
}

func TestValidate_MissingCustomerAPIURL(t *testing.T) {
	cfg := &Config{
		CustomerAPI: CustomerAPIConfig{
			URL:   "", // Missing required field
			Token: "test_token",
		},
	}
	
	err := cfg.Validate()
	if err == nil {
		t.Error("Expected validation error for missing CUSTOMER_API_URL")
	}
	if err != nil && err.Error() != "CUSTOMER_API_URL is required" {
		t.Errorf("Expected error message 'CUSTOMER_API_URL is required', got %v", err)
	}
}

func TestValidate_MissingCustomerAPIToken(t *testing.T) {
	cfg := &Config{
		CustomerAPI: CustomerAPIConfig{
			URL:   "https://test.api.com",
			Token: "", // Missing required field
		},
	}
	
	err := cfg.Validate()
	if err == nil {
		t.Error("Expected validation error for missing CUSTOMER_API_TOKEN")
	}
	if err != nil && err.Error() != "CUSTOMER_API_TOKEN is required" {
		t.Errorf("Expected error message 'CUSTOMER_API_TOKEN is required', got %v", err)
	}
}

func TestValidate_MissingSharedSecretWhenAuthEnabled(t *testing.T) {
	cfg := &Config{
		CustomerAPI: CustomerAPIConfig{
			URL:         "https://test.api.com",
			Token:       "test_token",
			ProductName: "test_product",
		},
		Auth: AuthConfig{
			Enabled:      true,
			SharedSecret: "", // Missing when auth enabled
		},
	}
	
	err := cfg.Validate()
	if err == nil {
		t.Error("Expected validation error for missing SHARED_SECRET when auth enabled")
	}
	if err != nil && err.Error() != "SHARED_SECRET is required when ENABLE_AUTH is true" {
		t.Errorf("Expected error message about SHARED_SECRET, got %v", err)
	}
}

func TestValidate_Success(t *testing.T) {
	cfg := &Config{
		CustomerAPI: CustomerAPIConfig{
			URL:         "https://test.api.com",
			Token:       "test_token",
			ProductName: "test_product",
		},
		Auth: AuthConfig{
			Enabled:      false,
			SharedSecret: "",
		},
	}
	
	err := cfg.Validate()
	if err != nil {
		t.Errorf("Expected validation to pass, got error: %v", err)
	}
}

func TestLoadAttributeMapping_ValidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	mappingFile := filepath.Join(tmpDir, "test_mapping.json")
	mappingContent := `{
		"phone": {
			"type": "text",
			"required": true
		},
		"email": {
			"type": "text",
			"required": false
		},
		"house.roof_type": {
			"type": "dropdown",
			"required": false,
			"options": ["flat", "pitched"]
		},
		"house.roof_area": {
			"type": "range",
			"required": false,
			"min": 0,
			"max": 1000
		}
	}`
	
	if err := os.WriteFile(mappingFile, []byte(mappingContent), 0644); err != nil {
		t.Fatalf("Failed to create test mapping file: %v", err)
	}
	
	cfg := &Config{
		AttributeMapping: AttributeMappingConfig{
			FilePath: mappingFile,
		},
	}
	
	err := cfg.LoadAttributeMapping()
	if err != nil {
		t.Fatalf("LoadAttributeMapping() failed: %v", err)
	}
	
	// Verify phone attribute
	phone, ok := cfg.AttributeMapping.Mapping["phone"]
	if !ok {
		t.Error("Expected 'phone' attribute in mapping")
	}
	if phone.Type != "text" {
		t.Errorf("Expected phone type=text, got %s", phone.Type)
	}
	if !phone.Required {
		t.Error("Expected phone required=true")
	}
	
	// Verify email attribute
	email, ok := cfg.AttributeMapping.Mapping["email"]
	if !ok {
		t.Error("Expected 'email' attribute in mapping")
	}
	if email.Required {
		t.Error("Expected email required=false")
	}
	
	// Verify dropdown attribute
	roofType, ok := cfg.AttributeMapping.Mapping["house.roof_type"]
	if !ok {
		t.Error("Expected 'house.roof_type' attribute in mapping")
	}
	if roofType.Type != "dropdown" {
		t.Errorf("Expected roof_type type=dropdown, got %s", roofType.Type)
	}
	if len(roofType.Options) != 2 {
		t.Errorf("Expected 2 options, got %d", len(roofType.Options))
	}
	
	// Verify range attribute
	roofArea, ok := cfg.AttributeMapping.Mapping["house.roof_area"]
	if !ok {
		t.Error("Expected 'house.roof_area' attribute in mapping")
	}
	if roofArea.Type != "range" {
		t.Errorf("Expected roof_area type=range, got %s", roofArea.Type)
	}
	if roofArea.Min == nil || *roofArea.Min != 0 {
		t.Error("Expected roof_area min=0")
	}
	if roofArea.Max == nil || *roofArea.Max != 1000 {
		t.Error("Expected roof_area max=1000")
	}
}

func TestLoadAttributeMapping_FileNotFound(t *testing.T) {
	cfg := &Config{
		AttributeMapping: AttributeMappingConfig{
			FilePath: "/nonexistent/path/mapping.json",
		},
	}
	
	err := cfg.LoadAttributeMapping()
	if err == nil {
		t.Error("Expected error when mapping file does not exist")
	}
}

func TestLoadAttributeMapping_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	mappingFile := filepath.Join(tmpDir, "invalid_mapping.json")
	invalidContent := `{"phone": {invalid json}}`
	
	if err := os.WriteFile(mappingFile, []byte(invalidContent), 0644); err != nil {
		t.Fatalf("Failed to create test mapping file: %v", err)
	}
	
	cfg := &Config{
		AttributeMapping: AttributeMappingConfig{
			FilePath: mappingFile,
		},
	}
	
	err := cfg.LoadAttributeMapping()
	if err == nil {
		t.Error("Expected error when parsing invalid JSON")
	}
}

func TestParseBool(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"true", true},
		{"1", true},
		{"yes", true},
		{"false", false},
		{"0", false},
		{"no", false},
		{"", false},
		{"invalid", false},
	}
	
	for _, tt := range tests {
		result := parseBool(tt.input)
		if result != tt.expected {
			t.Errorf("parseBool(%q) = %v, expected %v", tt.input, result, tt.expected)
		}
	}
}

func TestParseInt(t *testing.T) {
	tests := []struct {
		input        string
		defaultValue int
		expected     int
	}{
		{"42", 10, 42},
		{"0", 10, 0},
		{"-5", 10, -5},
		{"invalid", 10, 10},
		{"", 10, 10},
		{"3.14", 10, 3}, // fmt.Sscanf parses the integer part
	}
	
	for _, tt := range tests {
		result := parseInt(tt.input, tt.defaultValue)
		if result != tt.expected {
			t.Errorf("parseInt(%q, %d) = %d, expected %d", tt.input, tt.defaultValue, result, tt.expected)
		}
	}
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		input        string
		defaultValue time.Duration
		expected     time.Duration
	}{
		{"5s", 10 * time.Second, 5 * time.Second},
		{"1m", 10 * time.Second, 1 * time.Minute},
		{"100ms", 10 * time.Second, 100 * time.Millisecond},
		{"invalid", 10 * time.Second, 10 * time.Second},
		{"", 10 * time.Second, 10 * time.Second},
	}
	
	for _, tt := range tests {
		result := parseDuration(tt.input, tt.defaultValue)
		if result != tt.expected {
			t.Errorf("parseDuration(%q, %v) = %v, expected %v", tt.input, tt.defaultValue, result, tt.expected)
		}
	}
}
