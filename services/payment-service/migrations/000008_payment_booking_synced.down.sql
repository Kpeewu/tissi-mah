-- =============================================================================
-- Rollback migration 000008
-- =============================================================================
DROP INDEX IF EXISTS idx_payments_held_unsynced;
ALTER TABLE payments DROP COLUMN IF EXISTS booking_sync_attempts;
ALTER TABLE payments DROP COLUMN IF EXISTS booking_synced_at;
