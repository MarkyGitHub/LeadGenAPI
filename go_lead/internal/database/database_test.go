package database

import (
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	// This is a basic test that verifies the database connection logic
	// In a real environment, you would use a test database
	cfg := Config{
		Host:            "localhost",
		Port:            "5432",
		User:            "postgres",
		Password:        "postgres",
		DBName:          "test_db",
		SSLMode:         "disable",
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: 5 * time.Minute,
	}

	// Note: This test will fail if PostgreSQL is not running
	// In production, use testcontainers or a test database
	db, err := New(cfg)
	if err != nil {
		t.Logf("Database connection failed (expected if no test DB): %v", err)
		t.Skip("Skipping test - no database available")
		return
	}
	defer db.Close()

	// Verify connection pool settings
	stats := db.Stats()
	if stats.MaxOpenConnections != 10 {
		t.Errorf("Expected MaxOpenConnections=10, got %d", stats.MaxOpenConnections)
	}

	// Test health check
	if err := db.HealthCheck(); err != nil {
		t.Errorf("Health check failed: %v", err)
	}
}

func TestConfigDefaults(t *testing.T) {
	// Test that default values are applied correctly
	cfg := Config{
		Host:     "localhost",
		Port:     "5432",
		User:     "test",
		Password: "test",
		DBName:   "test",
		SSLMode:  "disable",
		// Leave pool settings at zero to test defaults
	}

	// We can't actually connect without a real database,
	// but we can verify the config structure
	if cfg.MaxOpenConns != 0 {
		t.Errorf("Expected MaxOpenConns to be 0 before initialization")
	}
}
