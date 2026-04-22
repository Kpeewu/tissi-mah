-- =============================================================================
-- Rollback migration 000004
-- =============================================================================

DROP INDEX IF EXISTS idx_refunds_status_amount_to_passenger;

ALTER TABLE refunds
    DROP CONSTRAINT IF EXISTS chk_refund_retry_count,
    DROP COLUMN IF EXISTS last_retry_at,
    DROP COLUMN IF EXISTS retry_count,
    DROP COLUMN IF EXISTS failure_reason,
    DROP COLUMN IF EXISTS payment_provider_reference,
    DROP COLUMN IF EXISTS payment_provider,
    DROP COLUMN IF EXISTS payout_destination;

ALTER TABLE payments
    DROP COLUMN IF EXISTS passenger_phone_number;
