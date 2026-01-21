package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all configuration for the application
type Config struct {
	Database         DatabaseConfig
	API              APIConfig
	Worker           WorkerConfig
	Queue            QueueConfig
	CustomerAPI      CustomerAPIConfig
	Retry            RetryConfig
	Auth             AuthConfig
	Logging          LoggingConfig
	AttributeMapping AttributeMappingConfig
}

// DatabaseConfig holds database connection settings
type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// APIConfig holds API server settings
type APIConfig struct {
	Port string
	Host string
}

// WorkerConfig holds worker settings
type WorkerConfig struct {
	PollInterval time.Duration
	Concurrency  int
}

// QueueConfig holds queue settings
type QueueConfig struct {
	Type     string // "redis" or "database"
	RedisURL string
}

// CustomerAPIConfig holds Customer API client settings
type CustomerAPIConfig struct {
	URL         string
	Token       string
	Timeout     time.Duration
	ProductName string
}

// RetryConfig holds retry logic settings
type RetryConfig struct {
	MaxAttempts   int
	BackoffBase   time.Duration
}

// AuthConfig holds authentication settings
type AuthConfig struct {
	Enabled      bool
	SharedSecret string
}

// LoggingConfig holds logging settings
type LoggingConfig struct {
	Level  string
	Format string
}

// AttributeMappingConfig holds attribute mapping configuration
type AttributeMappingConfig struct {
	FilePath string
	Mapping  map[string]AttributeDefinition
}

// AttributeDefinition defines validation rules for an attribute
type AttributeDefinition struct {
	Type     string   `json:"type"`     // "text", "dropdown", "range"
	Required bool     `json:"required"` // true for core fields
	Options  []string `json:"options"`  // for dropdown type
	Min      *float64 `json:"min"`      // for range type
	Max      *float64 `json:"max"`      // for range type
}

// Load loads configuration from environment variables and files
func Load() (*Config, error) {
	// Load .env file if it exists (ignore error if not found)
	_ = godotenv.Load()

	cfg := &Config{
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5433"),
			User:     getEnv("DB_USER", "postgres"),
			Password: getEnv("DB_PASSWORD", "postgres"),
			DBName:   getEnv("DB_NAME", "lead_gateway"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		API: APIConfig{
			Port: getEnv("API_PORT", "8080"),
			Host: getEnv("API_HOST", "0.0.0.0"),
		},
		Worker: WorkerConfig{
			PollInterval: parseDuration(getEnv("WORKER_POLL_INTERVAL", "5s"), 5*time.Second),
			Concurrency:  parseInt(getEnv("WORKER_CONCURRENCY", "5"), 5),
		},
		Queue: QueueConfig{
			Type:     getEnv("QUEUE_TYPE", "redis"),
			RedisURL: getEnv("REDIS_URL", "redis://localhost:6379/0"),
		},
		CustomerAPI: CustomerAPIConfig{
			URL:         getEnv("CUSTOMER_API_URL", ""),
			Token:       getEnv("CUSTOMER_API_TOKEN", ""),
			Timeout:     parseDuration(getEnv("CUSTOMER_API_TIMEOUT", "30s"), 30*time.Second),
			ProductName: getEnv("CUSTOMER_PRODUCT_NAME", ""),
		},
		Retry: RetryConfig{
			MaxAttempts: parseInt(getEnv("MAX_RETRY_ATTEMPTS", "5"), 5),
			BackoffBase: parseDuration(getEnv("RETRY_BACKOFF_BASE", "30s"), 30*time.Second),
		},
		Auth: AuthConfig{
			Enabled:      parseBool(getEnv("ENABLE_AUTH", "false")),
			SharedSecret: getEnv("SHARED_SECRET", ""),
		},
		Logging: LoggingConfig{
			Level:  getEnv("LOG_LEVEL", "info"),
			Format: getEnv("LOG_FORMAT", "json"),
		},
		AttributeMapping: AttributeMappingConfig{
			FilePath: getEnv("ATTRIBUTE_MAPPING_FILE", "./config/customer_attribute_mapping.json"),
		},
	}

	// Validate required fields
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	// Load attribute mapping from file
	if err := cfg.LoadAttributeMapping(); err != nil {
		return nil, fmt.Errorf("failed to load attribute mapping: %w", err)
	}

	return cfg, nil
}

// Validate checks that required configuration fields are set
func (c *Config) Validate() error {
	if c.CustomerAPI.URL == "" {
		return fmt.Errorf("CUSTOMER_API_URL is required")
	}
	if c.CustomerAPI.Token == "" {
		return fmt.Errorf("CUSTOMER_API_TOKEN is required")
	}
	if c.CustomerAPI.ProductName == "" {
		return fmt.Errorf("CUSTOMER_PRODUCT_NAME is required")
	}
	if c.Auth.Enabled && c.Auth.SharedSecret == "" {
		return fmt.Errorf("SHARED_SECRET is required when ENABLE_AUTH is true")
	}
	return nil
}

// LoadAttributeMapping loads attribute definitions from JSON file
func (c *Config) LoadAttributeMapping() error {
	data, err := os.ReadFile(c.AttributeMapping.FilePath)
	if err != nil {
		return fmt.Errorf("failed to read attribute mapping file: %w", err)
	}

	// Support both current schema and legacy schema with metadata keys.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("failed to parse attribute mapping JSON: %w", err)
	}

	mapping := make(map[string]AttributeDefinition)
	for key, value := range raw {
		if strings.HasPrefix(key, "_") {
			continue
		}

		var def AttributeDefinition
		if err := json.Unmarshal(value, &def); err == nil && def.Type != "" {
			mapping[key] = def
			continue
		}

		// Legacy schema support
		var legacy struct {
			AttributeType string   `json:"attribute_type"`
			Values        []string `json:"values"`
		}
		if err := json.Unmarshal(value, &legacy); err != nil {
			return fmt.Errorf("failed to parse attribute mapping JSON for key '%s': %w", key, err)
		}
		if legacy.AttributeType == "" {
			return fmt.Errorf("invalid attribute mapping for key '%s': missing attribute_type/type", key)
		}

		mapping[key] = AttributeDefinition{
			Type:     legacy.AttributeType,
			Required: false,
			Options:  legacy.Values,
			Min:      nil,
			Max:      nil,
		}
	}

	c.AttributeMapping.Mapping = mapping
	return nil
}

// Helper functions

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func parseDuration(value string, defaultValue time.Duration) time.Duration {
	d, err := time.ParseDuration(value)
	if err != nil {
		return defaultValue
	}
	return d
}

func parseInt(value string, defaultValue int) int {
	var result int
	_, err := fmt.Sscanf(value, "%d", &result)
	if err != nil {
		return defaultValue
	}
	return result
}

func parseBool(value string) bool {
	return value == "true" || value == "1" || value == "yes"
}
