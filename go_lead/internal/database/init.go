package database

import (
	"fmt"

	"github.com/checkfox/go_lead/internal/config"
)

// InitFromConfig initializes a database connection from application config
func InitFromConfig(cfg *config.Config) (*DB, error) {
	dbConfig := Config{
		Host:     cfg.Database.Host,
		Port:     cfg.Database.Port,
		User:     cfg.Database.User,
		Password: cfg.Database.Password,
		DBName:   cfg.Database.DBName,
		SSLMode:  cfg.Database.SSLMode,
	}

	db, err := New(dbConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	return db, nil
}

// RunMigrations runs all pending database migrations
func RunMigrations(db *DB, migrationsPath string) error {
	runner := NewMigrationRunner(db, migrationsPath)
	if err := runner.Run(); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}
	return nil
}

// MigrationStatus prints the current migration status
func MigrationStatus(db *DB, migrationsPath string) error {
	runner := NewMigrationRunner(db, migrationsPath)
	return runner.Status()
}
