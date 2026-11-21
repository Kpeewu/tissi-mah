DROP INDEX IF EXISTS idx_refunds_driver_payout_processing;
DROP INDEX IF EXISTS idx_refunds_driver_payout_pending;

ALTER TABLE refunds
    DROP CONSTRAINT IF EXISTS chk_driver_payout_retry_count,
    DROP COLUMN IF EXISTS driver_payout_completed_at,
    DROP COLUMN IF EXISTS driver_payout_last_retry_at,
    DROP COLUMN IF EXISTS driver_payout_retry_count,
    DROP COLUMN IF EXISTS driver_payout_provider_ref,
    DROP COLUMN IF EXISTS driver_payout_status;
