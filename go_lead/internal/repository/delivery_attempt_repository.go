package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/checkfox/go_lead/internal/models"
)

// DeliveryAttemptRepository defines the interface for delivery attempt data persistence operations
type DeliveryAttemptRepository interface {
	// CreateDeliveryAttempt creates a new delivery attempt record
	CreateDeliveryAttempt(ctx context.Context, attempt *models.DeliveryAttempt) error
	
	// CreateDeliveryAttemptTx creates a new delivery attempt record within a transaction
	CreateDeliveryAttemptTx(ctx context.Context, tx *sql.Tx, attempt *models.DeliveryAttempt) error
	
	// GetDeliveryAttemptsByLeadID retrieves all delivery attempts for a specific lead
	GetDeliveryAttemptsByLeadID(ctx context.Context, leadID int64) ([]*models.DeliveryAttempt, error)
	
	// GetLatestDeliveryAttempt retrieves the most recent delivery attempt for a lead
	GetLatestDeliveryAttempt(ctx context.Context, leadID int64) (*models.DeliveryAttempt, error)
	
	// CountDeliveryAttempts returns the number of delivery attempts for a lead
	CountDeliveryAttempts(ctx context.Context, leadID int64) (int, error)
}

// deliveryAttemptRepository is the concrete implementation of DeliveryAttemptRepository
type deliveryAttemptRepository struct {
	db *sql.DB
}

// NewDeliveryAttemptRepository creates a new DeliveryAttemptRepository instance
func NewDeliveryAttemptRepository(db *sql.DB) DeliveryAttemptRepository {
	return &deliveryAttemptRepository{
		db: db,
	}
}

// CreateDeliveryAttempt creates a new delivery attempt record
func (r *deliveryAttemptRepository) CreateDeliveryAttempt(ctx context.Context, attempt *models.DeliveryAttempt) error {
	query := `
		INSERT INTO delivery_attempt (
			lead_id, attempt_no, requested_at, response_status,
			response_body, error_message, success, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id
	`
	
	now := time.Now()
	if attempt.RequestedAt.IsZero() {
		attempt.RequestedAt = now
	}
	if attempt.CreatedAt.IsZero() {
		attempt.CreatedAt = now
	}
	
	err := r.db.QueryRowContext(
		ctx,
		query,
		attempt.LeadID,
		attempt.AttemptNo,
		attempt.RequestedAt,
		attempt.ResponseStatus,
		attempt.ResponseBody,
		attempt.ErrorMessage,
		attempt.Success,
		attempt.CreatedAt,
	).Scan(&attempt.ID)
	
	if err != nil {
		return fmt.Errorf("failed to create delivery attempt: %w", err)
	}
	
	return nil
}

// CreateDeliveryAttemptTx creates a new delivery attempt record within a transaction
func (r *deliveryAttemptRepository) CreateDeliveryAttemptTx(ctx context.Context, tx *sql.Tx, attempt *models.DeliveryAttempt) error {
	query := `
		INSERT INTO delivery_attempt (
			lead_id, attempt_no, requested_at, response_status,
			response_body, error_message, success, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id
	`
	
	now := time.Now()
	if attempt.RequestedAt.IsZero() {
		attempt.RequestedAt = now
	}
	if attempt.CreatedAt.IsZero() {
		attempt.CreatedAt = now
	}
	
	err := tx.QueryRowContext(
		ctx,
		query,
		attempt.LeadID,
		attempt.AttemptNo,
		attempt.RequestedAt,
		attempt.ResponseStatus,
		attempt.ResponseBody,
		attempt.ErrorMessage,
		attempt.Success,
		attempt.CreatedAt,
	).Scan(&attempt.ID)
	
	if err != nil {
		return fmt.Errorf("failed to create delivery attempt in transaction: %w", err)
	}
	
	return nil
}

// GetDeliveryAttemptsByLeadID retrieves all delivery attempts for a specific lead
func (r *deliveryAttemptRepository) GetDeliveryAttemptsByLeadID(ctx context.Context, leadID int64) ([]*models.DeliveryAttempt, error) {
	query := `
		SELECT 
			id, lead_id, attempt_no, requested_at, response_status,
			response_body, error_message, success, created_at
		FROM delivery_attempt
		WHERE lead_id = $1
		ORDER BY attempt_no ASC
	`
	
	rows, err := r.db.QueryContext(ctx, query, leadID)
	if err != nil {
		return nil, fmt.Errorf("failed to query delivery attempts: %w", err)
	}
	defer rows.Close()
	
	var attempts []*models.DeliveryAttempt
	for rows.Next() {
		attempt := &models.DeliveryAttempt{}
		err := rows.Scan(
			&attempt.ID,
			&attempt.LeadID,
			&attempt.AttemptNo,
			&attempt.RequestedAt,
			&attempt.ResponseStatus,
			&attempt.ResponseBody,
			&attempt.ErrorMessage,
			&attempt.Success,
			&attempt.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan delivery attempt: %w", err)
		}
		attempts = append(attempts, attempt)
	}
	
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating delivery attempts: %w", err)
	}
	
	return attempts, nil
}

// GetLatestDeliveryAttempt retrieves the most recent delivery attempt for a lead
func (r *deliveryAttemptRepository) GetLatestDeliveryAttempt(ctx context.Context, leadID int64) (*models.DeliveryAttempt, error) {
	query := `
		SELECT 
			id, lead_id, attempt_no, requested_at, response_status,
			response_body, error_message, success, created_at
		FROM delivery_attempt
		WHERE lead_id = $1
		ORDER BY attempt_no DESC
		LIMIT 1
	`
	
	attempt := &models.DeliveryAttempt{}
	err := r.db.QueryRowContext(ctx, query, leadID).Scan(
		&attempt.ID,
		&attempt.LeadID,
		&attempt.AttemptNo,
		&attempt.RequestedAt,
		&attempt.ResponseStatus,
		&attempt.ResponseBody,
		&attempt.ErrorMessage,
		&attempt.Success,
		&attempt.CreatedAt,
	)
	
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("no delivery attempts found for lead: %d", leadID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get latest delivery attempt: %w", err)
	}
	
	return attempt, nil
}

// CountDeliveryAttempts returns the number of delivery attempts for a lead
func (r *deliveryAttemptRepository) CountDeliveryAttempts(ctx context.Context, leadID int64) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM delivery_attempt
		WHERE lead_id = $1
	`
	
	var count int
	err := r.db.QueryRowContext(ctx, query, leadID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count delivery attempts: %w", err)
	}
	
	return count, nil
}
