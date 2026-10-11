-- =============================================================================
-- Rollback migration 000005
-- =============================================================================
-- Note : la valeur 'seatsUnavailable' de l'enum refund_reason n'est pas retirée
-- (PostgreSQL ne supporte pas DROP VALUE sur un enum).

DROP INDEX IF EXISTS idx_payments_pending_expires_at;

ALTER TABLE payments
    DROP COLUMN IF EXISTS expires_at,
    DROP COLUMN IF EXISTS provider_status_updated_at,
    DROP COLUMN IF EXISTS provider_status,
    DROP COLUMN IF EXISTS payment_url;
