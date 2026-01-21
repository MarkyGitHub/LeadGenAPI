-- Migration: Create inbound_lead table
-- This table stores all incoming leads with their processing status and payloads

CREATE TABLE IF NOT EXISTS inbound_lead (
    id SERIAL PRIMARY KEY,
    received_at TIMESTAMP NOT NULL DEFAULT NOW(),
    raw_payload JSONB NOT NULL,
    source_headers JSONB,
    status VARCHAR(20) NOT NULL DEFAULT 'RECEIVED',
    rejection_reason VARCHAR(100),
    normalized_payload JSONB,
    customer_payload JSONB,
    payload_hash VARCHAR(64) UNIQUE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Add constraint to ensure status is one of the valid values
ALTER TABLE inbound_lead ADD CONSTRAINT check_status 
    CHECK (status IN ('RECEIVED', 'REJECTED', 'READY', 'DELIVERED', 'FAILED', 'PERMANENTLY_FAILED'));

-- Create indexes for common query patterns
CREATE INDEX idx_inbound_lead_status ON inbound_lead(status);
CREATE INDEX idx_inbound_lead_received_at ON inbound_lead(received_at);
CREATE INDEX idx_inbound_lead_payload_hash ON inbound_lead(payload_hash);

-- Add comment for documentation
COMMENT ON TABLE inbound_lead IS 'Stores all incoming leads with their processing status and audit trail';
COMMENT ON COLUMN inbound_lead.status IS 'Current processing status: RECEIVED, REJECTED, READY, DELIVERED, FAILED, PERMANENTLY_FAILED';
COMMENT ON COLUMN inbound_lead.rejection_reason IS 'Reason code if lead was rejected during validation (e.g., ZIP_NOT_66XXX, NOT_HOMEOWNER)';
COMMENT ON COLUMN inbound_lead.payload_hash IS 'SHA-256 hash of normalized payload for deduplication (optional enhancement)';
