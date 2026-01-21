package repository

import (
	"context"
	"testing"

	"github.com/checkfox/go_lead/internal/models"
)

func TestDeliveryAttemptRepository_CreateDeliveryAttempt(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer db.Close()
	defer cleanupTestData(t, db)

	leadRepo := NewLeadRepository(db)
	attemptRepo := NewDeliveryAttemptRepository(db)
	ctx := context.Background()

	// Create a lead first
	lead := &models.InboundLead{
		RawPayload: models.JSONB{"email": "test@example.com"},
		Status:     models.LeadStatusReady,
	}

	err := leadRepo.CreateLead(ctx, lead)
	if err != nil {
		t.Fatalf("Failed to create lead: %v", err)
	}

	// Create a delivery attempt
	attempt := models.NewDeliveryAttempt(lead.ID, 1)
	statusCode := 200
	responseBody := "Success"
	attempt.MarkSuccess(statusCode, responseBody)

	err = attemptRepo.CreateDeliveryAttempt(ctx, attempt)
	if err != nil {
		t.Fatalf("Failed to create delivery attempt: %v", err)
	}

	if attempt.ID == 0 {
		t.Error("Expected delivery attempt ID to be set after creation")
	}

	if !attempt.Success {
		t.Error("Expected delivery attempt to be marked as success")
	}
}

func TestDeliveryAttemptRepository_GetDeliveryAttemptsByLeadID(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer db.Close()
	defer cleanupTestData(t, db)

	leadRepo := NewLeadRepository(db)
	attemptRepo := NewDeliveryAttemptRepository(db)
	ctx := context.Background()

	// Create a lead
	lead := &models.InboundLead{
		RawPayload: models.JSONB{"email": "test@example.com"},
		Status:     models.LeadStatusReady,
	}

	err := leadRepo.CreateLead(ctx, lead)
	if err != nil {
		t.Fatalf("Failed to create lead: %v", err)
	}

	// Create multiple delivery attempts
	for i := 1; i <= 3; i++ {
		attempt := models.NewDeliveryAttempt(lead.ID, i)
		if i < 3 {
			statusCode := 500
			attempt.MarkFailure(&statusCode, "Server error")
		} else {
			statusCode := 200
			attempt.MarkSuccess(statusCode, "Success")
		}

		err = attemptRepo.CreateDeliveryAttempt(ctx, attempt)
		if err != nil {
			t.Fatalf("Failed to create delivery attempt %d: %v", i, err)
		}
	}

	// Retrieve all attempts
	attempts, err := attemptRepo.GetDeliveryAttemptsByLeadID(ctx, lead.ID)
	if err != nil {
		t.Fatalf("Failed to get delivery attempts: %v", err)
	}

	if len(attempts) != 3 {
		t.Errorf("Expected 3 delivery attempts, got %d", len(attempts))
	}

	// Verify attempts are ordered by attempt_no
	for i, attempt := range attempts {
		expectedAttemptNo := i + 1
		if attempt.AttemptNo != expectedAttemptNo {
			t.Errorf("Expected attempt_no %d, got %d", expectedAttemptNo, attempt.AttemptNo)
		}
	}

	// Verify last attempt is successful
	if !attempts[2].Success {
		t.Error("Expected last attempt to be successful")
	}
}

func TestDeliveryAttemptRepository_GetLatestDeliveryAttempt(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer db.Close()
	defer cleanupTestData(t, db)

	leadRepo := NewLeadRepository(db)
	attemptRepo := NewDeliveryAttemptRepository(db)
	ctx := context.Background()

	// Create a lead
	lead := &models.InboundLead{
		RawPayload: models.JSONB{"email": "test@example.com"},
		Status:     models.LeadStatusReady,
	}

	err := leadRepo.CreateLead(ctx, lead)
	if err != nil {
		t.Fatalf("Failed to create lead: %v", err)
	}

	// Create multiple delivery attempts
	for i := 1; i <= 3; i++ {
		attempt := models.NewDeliveryAttempt(lead.ID, i)
		statusCode := 500
		attempt.MarkFailure(&statusCode, "Server error")

		err = attemptRepo.CreateDeliveryAttempt(ctx, attempt)
		if err != nil {
			t.Fatalf("Failed to create delivery attempt %d: %v", i, err)
		}
	}

	// Get latest attempt
	latest, err := attemptRepo.GetLatestDeliveryAttempt(ctx, lead.ID)
	if err != nil {
		t.Fatalf("Failed to get latest delivery attempt: %v", err)
	}

	if latest.AttemptNo != 3 {
		t.Errorf("Expected latest attempt_no to be 3, got %d", latest.AttemptNo)
	}

	// Test non-existent lead
	_, err = attemptRepo.GetLatestDeliveryAttempt(ctx, 999999)
	if err == nil {
		t.Error("Expected error when getting latest attempt for non-existent lead")
	}
}

func TestDeliveryAttemptRepository_CountDeliveryAttempts(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer db.Close()
	defer cleanupTestData(t, db)

	leadRepo := NewLeadRepository(db)
	attemptRepo := NewDeliveryAttemptRepository(db)
	ctx := context.Background()

	// Create a lead
	lead := &models.InboundLead{
		RawPayload: models.JSONB{"email": "test@example.com"},
		Status:     models.LeadStatusReady,
	}

	err := leadRepo.CreateLead(ctx, lead)
	if err != nil {
		t.Fatalf("Failed to create lead: %v", err)
	}

	// Initially should have 0 attempts
	count, err := attemptRepo.CountDeliveryAttempts(ctx, lead.ID)
	if err != nil {
		t.Fatalf("Failed to count delivery attempts: %v", err)
	}

	if count != 0 {
		t.Errorf("Expected 0 delivery attempts initially, got %d", count)
	}

	// Create delivery attempts
	for i := 1; i <= 5; i++ {
		attempt := models.NewDeliveryAttempt(lead.ID, i)
		statusCode := 500
		attempt.MarkFailure(&statusCode, "Server error")

		err = attemptRepo.CreateDeliveryAttempt(ctx, attempt)
		if err != nil {
			t.Fatalf("Failed to create delivery attempt %d: %v", i, err)
		}
	}

	// Count should now be 5
	count, err = attemptRepo.CountDeliveryAttempts(ctx, lead.ID)
	if err != nil {
		t.Fatalf("Failed to count delivery attempts: %v", err)
	}

	if count != 5 {
		t.Errorf("Expected 5 delivery attempts, got %d", count)
	}
}

func TestDeliveryAttemptRepository_CreateDeliveryAttemptTx(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer db.Close()
	defer cleanupTestData(t, db)

	leadRepo := NewLeadRepository(db)
	attemptRepo := NewDeliveryAttemptRepository(db)
	ctx := context.Background()

	// Create a lead
	lead := &models.InboundLead{
		RawPayload: models.JSONB{"email": "test@example.com"},
		Status:     models.LeadStatusReady,
	}

	err := leadRepo.CreateLead(ctx, lead)
	if err != nil {
		t.Fatalf("Failed to create lead: %v", err)
	}

	// Test transaction commit
	tx, err := leadRepo.BeginTx(ctx)
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}

	attempt := models.NewDeliveryAttempt(lead.ID, 1)
	statusCode := 200
	attempt.MarkSuccess(statusCode, "Success")

	err = attemptRepo.CreateDeliveryAttemptTx(ctx, tx, attempt)
	if err != nil {
		tx.Rollback()
		t.Fatalf("Failed to create delivery attempt in transaction: %v", err)
	}

	// Update lead status in same transaction
	err = leadRepo.UpdateLeadStatusTx(ctx, tx, lead.ID, models.LeadStatusDelivered)
	if err != nil {
		tx.Rollback()
		t.Fatalf("Failed to update lead status in transaction: %v", err)
	}

	err = tx.Commit()
	if err != nil {
		t.Fatalf("Failed to commit transaction: %v", err)
	}

	// Verify both operations were committed
	attempts, err := attemptRepo.GetDeliveryAttemptsByLeadID(ctx, lead.ID)
	if err != nil {
		t.Fatalf("Failed to get delivery attempts: %v", err)
	}

	if len(attempts) != 1 {
		t.Errorf("Expected 1 delivery attempt after commit, got %d", len(attempts))
	}

	retrievedLead, err := leadRepo.GetLeadByID(ctx, lead.ID)
	if err != nil {
		t.Fatalf("Failed to get lead: %v", err)
	}

	if retrievedLead.Status != models.LeadStatusDelivered {
		t.Errorf("Expected status DELIVERED after commit, got %s", retrievedLead.Status)
	}
}
