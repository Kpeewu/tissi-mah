-- =============================================================================
-- Rollback migration 000009
-- =============================================================================
DROP INDEX IF EXISTS uq_refunds_payment_id;
CREATE INDEX IF NOT EXISTS idx_refunds_payment_id ON refunds(payment_id);
