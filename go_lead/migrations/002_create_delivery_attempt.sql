-- Migration: Create delivery_attempt table
-- This table stores all attempts to deliver leads to the Customer API

CREATE TABLE IF NOT EXISTS delivery_attempt (
    id SERIAL PRIMARY KEY,
    lead_id INTEGER NOT NULL REFERENCES inbound_lead(id) ON DELETE CASCADE,
    attempt_no INTEGER NOT NULL,
    requested_at TIMESTAMP NOT NULL DEFAULT NOW(),
    response_status INTEGER,
    response_body TEXT,
    error_message TEXT,
    success BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Add constraint to ensure attempt_no is positive
ALTER TABLE delivery_attempt ADD CONSTRAINT check_attempt_no 
    CHECK (attempt_no > 0);

-- Add constraint to ensure response_status is valid HTTP status code if present
ALTER TABLE delivery_attempt ADD CONSTRAINT check_response_status 
    CHECK (response_status IS NULL OR (response_status >= 100 AND response_status < 600));

-- Create indexes for common query patterns
CREATE INDEX idx_delivery_attempt_lead_id ON delivery_attempt(lead_id);
CREATE INDEX idx_delivery_attempt_success ON delivery_attempt(success);
CREATE INDEX idx_delivery_attempt_requested_at ON delivery_attempt(requested_at);

-- Add unique constraint to prevent duplicate attempt numbers for the same lead
CREATE UNIQUE INDEX idx_delivery_attempt_lead_attempt ON delivery_attempt(lead_id, attempt_no);

-- Add comment for documentation
COMMENT ON TABLE delivery_attempt IS 'Audit trail of all delivery attempts to the Customer API';
COMMENT ON COLUMN delivery_attempt.attempt_no IS 'Sequential attempt number (1-based) for this lead';
COMMENT ON COLUMN delivery_attempt.response_status IS 'HTTP status code returned by Customer API (null if network error)';
COMMENT ON COLUMN delivery_attempt.error_message IS 'Error details if delivery failed (network errors, timeouts, etc.)';
