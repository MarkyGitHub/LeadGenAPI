package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/checkfox/go_lead/internal/models"
)

// LeadRepository defines the interface for lead data persistence operations
type LeadRepository interface {
	// CreateLead creates a new inbound lead record
	CreateLead(ctx context.Context, lead *models.InboundLead) error
	
	// GetLeadByID retrieves a lead by its ID
	GetLeadByID(ctx context.Context, id int64) (*models.InboundLead, error)
	
	// UpdateLeadStatus updates the status of a lead atomically
	UpdateLeadStatus(ctx context.Context, id int64, status models.LeadStatus) error
	
	// UpdateLeadWithPayloads updates the lead with normalized and customer payloads
	UpdateLeadWithPayloads(ctx context.Context, id int64, normalizedPayload, customerPayload models.JSONB) error
	
	// UpdateLeadRejection marks a lead as rejected with a reason
	UpdateLeadRejection(ctx context.Context, id int64, reason models.RejectionReason) error
	
	// BeginTx starts a new database transaction
	BeginTx(ctx context.Context) (*sql.Tx, error)
	
	// UpdateLeadStatusTx updates the status of a lead within a transaction
	UpdateLeadStatusTx(ctx context.Context, tx *sql.Tx, id int64, status models.LeadStatus) error
	
	// GetLeadCountsByStatus returns counts of leads grouped by status
	GetLeadCountsByStatus(ctx context.Context) (map[string]int, error)
	
	// GetRecentLeads returns the most recent leads ordered by received_at
	GetRecentLeads(ctx context.Context, limit int) ([]*models.InboundLead, error)
}

// leadRepository is the concrete implementation of LeadRepository
type leadRepository struct {
	db *sql.DB
}

// NewLeadRepository creates a new LeadRepository instance
func NewLeadRepository(db *sql.DB) LeadRepository {
	return &leadRepository{
		db: db,
	}
}

// CreateLead creates a new inbound lead record
func (r *leadRepository) CreateLead(ctx context.Context, lead *models.InboundLead) error {
	query := `
		INSERT INTO inbound_lead (
			received_at, raw_payload, source_headers, status, 
			rejection_reason, normalized_payload, customer_payload, 
			payload_hash, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id
	`
	
	now := time.Now()
	if lead.ReceivedAt.IsZero() {
		lead.ReceivedAt = now
	}
	if lead.CreatedAt.IsZero() {
		lead.CreatedAt = now
	}
	if lead.UpdatedAt.IsZero() {
		lead.UpdatedAt = now
	}
	if lead.Status == "" {
		lead.Status = models.LeadStatusReceived
	}
	
	err := r.db.QueryRowContext(
		ctx,
		query,
		lead.ReceivedAt,
		lead.RawPayload,
		lead.SourceHeaders,
		lead.Status,
		lead.RejectionReason,
		lead.NormalizedPayload,
		lead.CustomerPayload,
		lead.PayloadHash,
		lead.CreatedAt,
		lead.UpdatedAt,
	).Scan(&lead.ID)
	
	if err != nil {
		return fmt.Errorf("failed to create lead: %w", err)
	}
	
	return nil
}

// GetLeadByID retrieves a lead by its ID
func (r *leadRepository) GetLeadByID(ctx context.Context, id int64) (*models.InboundLead, error) {
	query := `
		SELECT 
			id, received_at, raw_payload, source_headers, status,
			rejection_reason, normalized_payload, customer_payload,
			payload_hash, created_at, updated_at
		FROM inbound_lead
		WHERE id = $1
	`
	
	lead := &models.InboundLead{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&lead.ID,
		&lead.ReceivedAt,
		&lead.RawPayload,
		&lead.SourceHeaders,
		&lead.Status,
		&lead.RejectionReason,
		&lead.NormalizedPayload,
		&lead.CustomerPayload,
		&lead.PayloadHash,
		&lead.CreatedAt,
		&lead.UpdatedAt,
	)
	
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("lead not found: %d", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get lead: %w", err)
	}
	
	return lead, nil
}

// UpdateLeadStatus updates the status of a lead atomically
func (r *leadRepository) UpdateLeadStatus(ctx context.Context, id int64, status models.LeadStatus) error {
	query := `
		UPDATE inbound_lead
		SET status = $1, updated_at = $2
		WHERE id = $3
	`
	
	result, err := r.db.ExecContext(ctx, query, status, time.Now(), id)
	if err != nil {
		return fmt.Errorf("failed to update lead status: %w", err)
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	
	if rowsAffected == 0 {
		return fmt.Errorf("lead not found: %d", id)
	}
	
	return nil
}

// UpdateLeadWithPayloads updates the lead with normalized and customer payloads
func (r *leadRepository) UpdateLeadWithPayloads(ctx context.Context, id int64, normalizedPayload, customerPayload models.JSONB) error {
	query := `
		UPDATE inbound_lead
		SET normalized_payload = $1, customer_payload = $2, updated_at = $3
		WHERE id = $4
	`
	
	result, err := r.db.ExecContext(ctx, query, normalizedPayload, customerPayload, time.Now(), id)
	if err != nil {
		return fmt.Errorf("failed to update lead payloads: %w", err)
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	
	if rowsAffected == 0 {
		return fmt.Errorf("lead not found: %d", id)
	}
	
	return nil
}

// UpdateLeadRejection marks a lead as rejected with a reason
func (r *leadRepository) UpdateLeadRejection(ctx context.Context, id int64, reason models.RejectionReason) error {
	query := `
		UPDATE inbound_lead
		SET status = $1, rejection_reason = $2, updated_at = $3
		WHERE id = $4
	`
	
	reasonStr := reason.String()
	result, err := r.db.ExecContext(ctx, query, models.LeadStatusRejected, reasonStr, time.Now(), id)
	if err != nil {
		return fmt.Errorf("failed to update lead rejection: %w", err)
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	
	if rowsAffected == 0 {
		return fmt.Errorf("lead not found: %d", id)
	}
	
	return nil
}

// BeginTx starts a new database transaction
func (r *leadRepository) BeginTx(ctx context.Context) (*sql.Tx, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	return tx, nil
}

// UpdateLeadStatusTx updates the status of a lead within a transaction
func (r *leadRepository) UpdateLeadStatusTx(ctx context.Context, tx *sql.Tx, id int64, status models.LeadStatus) error {
	query := `
		UPDATE inbound_lead
		SET status = $1, updated_at = $2
		WHERE id = $3
	`
	
	result, err := tx.ExecContext(ctx, query, status, time.Now(), id)
	if err != nil {
		return fmt.Errorf("failed to update lead status in transaction: %w", err)
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	
	if rowsAffected == 0 {
		return fmt.Errorf("lead not found: %d", id)
	}
	
	return nil
}

// GetLeadCountsByStatus returns counts of leads grouped by status
// Requirements: 8.3
func (r *leadRepository) GetLeadCountsByStatus(ctx context.Context) (map[string]int, error) {
	query := `
		SELECT status, COUNT(*) as count
		FROM inbound_lead
		GROUP BY status
	`
	
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query lead counts: %w", err)
	}
	defer rows.Close()
	
	counts := make(map[string]int)
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		counts[status] = count
	}
	
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}
	
	return counts, nil
}

// GetRecentLeads returns the most recent leads ordered by received_at
// Requirements: 8.4
func (r *leadRepository) GetRecentLeads(ctx context.Context, limit int) ([]*models.InboundLead, error) {
	query := `
		SELECT 
			id, received_at, raw_payload, source_headers, status, 
			rejection_reason, normalized_payload, customer_payload, 
			created_at, updated_at
		FROM inbound_lead
		ORDER BY received_at DESC
		LIMIT $1
	`
	
	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query recent leads: %w", err)
	}
	defer rows.Close()
	
	leads := make([]*models.InboundLead, 0, limit)
	for rows.Next() {
		lead := &models.InboundLead{}
		var rejectionReason sql.NullString
		var normalizedPayload, customerPayload sql.NullString
		
		err := rows.Scan(
			&lead.ID,
			&lead.ReceivedAt,
			&lead.RawPayload,
			&lead.SourceHeaders,
			&lead.Status,
			&rejectionReason,
			&normalizedPayload,
			&customerPayload,
			&lead.CreatedAt,
			&lead.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan lead: %w", err)
		}
		
		if rejectionReason.Valid {
			lead.RejectionReason = &rejectionReason.String
		}
		
		if normalizedPayload.Valid {
			lead.NormalizedPayload = models.JSONB{}
		}
		
		if customerPayload.Valid {
			lead.CustomerPayload = models.JSONB{}
		}
		
		leads = append(leads, lead)
	}
	
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}
	
	return leads, nil
}
