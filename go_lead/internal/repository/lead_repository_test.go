package repository

import (
	"context"
	"database/sql"
	"testing"

	"github.com/checkfox/go_lead/internal/models"
	_ "github.com/lib/pq"
)

// setupTestDB creates a test database connection
// This will skip tests if no database is available
func setupTestDB(t *testing.T) *sql.DB {
	connStr := "host=localhost port=5432 user=postgres password=postgres dbname=test_lead_gateway sslmode=disable"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Skipf("Skipping test - cannot connect to test database: %v", err)
		return nil
	}

	if err := db.Ping(); err != nil {
		t.Skipf("Skipping test - test database not available: %v", err)
		return nil
	}

	return db
}

// cleanupTestData removes test data from the database
func cleanupTestData(t *testing.T, db *sql.DB) {
	_, err := db.Exec("DELETE FROM delivery_attempt")
	if err != nil {
		t.Logf("Warning: failed to clean delivery_attempt table: %v", err)
	}
	_, err = db.Exec("DELETE FROM inbound_lead")
	if err != nil {
		t.Logf("Warning: failed to clean inbound_lead table: %v", err)
	}
}

func TestLeadRepository_CreateLead(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer db.Close()
	defer cleanupTestData(t, db)

	repo := NewLeadRepository(db)
	ctx := context.Background()

	lead := &models.InboundLead{
		RawPayload: models.JSONB{
			"email": "test@example.com",
			"address": map[string]interface{}{
				"zip": "66123",
			},
		},
		SourceHeaders: models.JSONB{
			"Content-Type": "application/json",
		},
		Status: models.LeadStatusReceived,
	}

	err := repo.CreateLead(ctx, lead)
	if err != nil {
		t.Fatalf("Failed to create lead: %v", err)
	}

	if lead.ID == 0 {
		t.Error("Expected lead ID to be set after creation")
	}

	if lead.CreatedAt.IsZero() {
		t.Error("Expected CreatedAt to be set")
	}

	if lead.UpdatedAt.IsZero() {
		t.Error("Expected UpdatedAt to be set")
	}
}

func TestLeadRepository_GetLeadByID(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer db.Close()
	defer cleanupTestData(t, db)

	repo := NewLeadRepository(db)
	ctx := context.Background()

	// Create a lead first
	lead := &models.InboundLead{
		RawPayload: models.JSONB{
			"email": "test@example.com",
		},
		Status: models.LeadStatusReceived,
	}

	err := repo.CreateLead(ctx, lead)
	if err != nil {
		t.Fatalf("Failed to create lead: %v", err)
	}

	// Retrieve the lead
	retrieved, err := repo.GetLeadByID(ctx, lead.ID)
	if err != nil {
		t.Fatalf("Failed to get lead: %v", err)
	}

	if retrieved.ID != lead.ID {
		t.Errorf("Expected ID %d, got %d", lead.ID, retrieved.ID)
	}

	if retrieved.Status != models.LeadStatusReceived {
		t.Errorf("Expected status RECEIVED, got %s", retrieved.Status)
	}

	// Test non-existent lead
	_, err = repo.GetLeadByID(ctx, 999999)
	if err == nil {
		t.Error("Expected error when getting non-existent lead")
	}
}

func TestLeadRepository_UpdateLeadStatus(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer db.Close()
	defer cleanupTestData(t, db)

	repo := NewLeadRepository(db)
	ctx := context.Background()

	// Create a lead
	lead := &models.InboundLead{
		RawPayload: models.JSONB{"email": "test@example.com"},
		Status:     models.LeadStatusReceived,
	}

	err := repo.CreateLead(ctx, lead)
	if err != nil {
		t.Fatalf("Failed to create lead: %v", err)
	}

	// Update status
	err = repo.UpdateLeadStatus(ctx, lead.ID, models.LeadStatusReady)
	if err != nil {
		t.Fatalf("Failed to update lead status: %v", err)
	}

	// Verify update
	retrieved, err := repo.GetLeadByID(ctx, lead.ID)
	if err != nil {
		t.Fatalf("Failed to get lead: %v", err)
	}

	if retrieved.Status != models.LeadStatusReady {
		t.Errorf("Expected status READY, got %s", retrieved.Status)
	}
}

func TestLeadRepository_UpdateLeadRejection(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer db.Close()
	defer cleanupTestData(t, db)

	repo := NewLeadRepository(db)
	ctx := context.Background()

	// Create a lead
	lead := &models.InboundLead{
		RawPayload: models.JSONB{"email": "test@example.com"},
		Status:     models.LeadStatusReceived,
	}

	err := repo.CreateLead(ctx, lead)
	if err != nil {
		t.Fatalf("Failed to create lead: %v", err)
	}

	// Mark as rejected
	err = repo.UpdateLeadRejection(ctx, lead.ID, models.RejectionReasonZipNotValid)
	if err != nil {
		t.Fatalf("Failed to update lead rejection: %v", err)
	}

	// Verify update
	retrieved, err := repo.GetLeadByID(ctx, lead.ID)
	if err != nil {
		t.Fatalf("Failed to get lead: %v", err)
	}

	if retrieved.Status != models.LeadStatusRejected {
		t.Errorf("Expected status REJECTED, got %s", retrieved.Status)
	}

	if retrieved.RejectionReason == nil {
		t.Error("Expected rejection reason to be set")
	} else if *retrieved.RejectionReason != "ZIP_NOT_66XXX" {
		t.Errorf("Expected rejection reason ZIP_NOT_66XXX, got %s", *retrieved.RejectionReason)
	}
}

func TestLeadRepository_Transaction(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer db.Close()
	defer cleanupTestData(t, db)

	repo := NewLeadRepository(db)
	ctx := context.Background()

	// Create a lead
	lead := &models.InboundLead{
		RawPayload: models.JSONB{"email": "test@example.com"},
		Status:     models.LeadStatusReceived,
	}

	err := repo.CreateLead(ctx, lead)
	if err != nil {
		t.Fatalf("Failed to create lead: %v", err)
	}

	// Test transaction commit
	tx, err := repo.BeginTx(ctx)
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}

	err = repo.UpdateLeadStatusTx(ctx, tx, lead.ID, models.LeadStatusReady)
	if err != nil {
		tx.Rollback()
		t.Fatalf("Failed to update lead status in transaction: %v", err)
	}

	err = tx.Commit()
	if err != nil {
		t.Fatalf("Failed to commit transaction: %v", err)
	}

	// Verify the update was committed
	retrieved, err := repo.GetLeadByID(ctx, lead.ID)
	if err != nil {
		t.Fatalf("Failed to get lead: %v", err)
	}

	if retrieved.Status != models.LeadStatusReady {
		t.Errorf("Expected status READY after commit, got %s", retrieved.Status)
	}
}

func TestLeadRepository_TransactionRollback(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer db.Close()
	defer cleanupTestData(t, db)

	repo := NewLeadRepository(db)
	ctx := context.Background()

	// Create a lead
	lead := &models.InboundLead{
		RawPayload: models.JSONB{"email": "test@example.com"},
		Status:     models.LeadStatusReceived,
	}

	err := repo.CreateLead(ctx, lead)
	if err != nil {
		t.Fatalf("Failed to create lead: %v", err)
	}

	originalStatus := lead.Status

	// Test transaction rollback
	tx, err := repo.BeginTx(ctx)
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}

	err = repo.UpdateLeadStatusTx(ctx, tx, lead.ID, models.LeadStatusReady)
	if err != nil {
		tx.Rollback()
		t.Fatalf("Failed to update lead status in transaction: %v", err)
	}

	// Rollback the transaction
	err = tx.Rollback()
	if err != nil {
		t.Fatalf("Failed to rollback transaction: %v", err)
	}

	// Verify the update was rolled back
	retrieved, err := repo.GetLeadByID(ctx, lead.ID)
	if err != nil {
		t.Fatalf("Failed to get lead: %v", err)
	}

	if retrieved.Status != originalStatus {
		t.Errorf("Expected status %s after rollback, got %s", originalStatus, retrieved.Status)
	}
}

func TestLeadRepository_UpdateLeadWithPayloads(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer db.Close()
	defer cleanupTestData(t, db)

	repo := NewLeadRepository(db)
	ctx := context.Background()

	// Create a lead
	lead := &models.InboundLead{
		RawPayload: models.JSONB{"email": "test@example.com"},
		Status:     models.LeadStatusReceived,
	}

	err := repo.CreateLead(ctx, lead)
	if err != nil {
		t.Fatalf("Failed to create lead: %v", err)
	}

	// Update with payloads
	normalizedPayload := models.JSONB{
		"email": "test@example.com",
		"phone": "1234567890",
	}
	customerPayload := models.JSONB{
		"phone": "1234567890",
		"product": map[string]interface{}{
			"name": "Test Product",
		},
	}

	err = repo.UpdateLeadWithPayloads(ctx, lead.ID, normalizedPayload, customerPayload)
	if err != nil {
		t.Fatalf("Failed to update lead with payloads: %v", err)
	}

	// Verify update
	retrieved, err := repo.GetLeadByID(ctx, lead.ID)
	if err != nil {
		t.Fatalf("Failed to get lead: %v", err)
	}

	if retrieved.NormalizedPayload == nil {
		t.Error("Expected normalized payload to be set")
	}

	if retrieved.CustomerPayload == nil {
		t.Error("Expected customer payload to be set")
	}
}
