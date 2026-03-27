-- =============================================================================
-- Rollback : suppression des tables du payment-service
-- =============================================================================

DROP TABLE IF EXISTS payout_batches;
DROP TABLE IF EXISTS webhook_events;
DROP TABLE IF EXISTS payouts;
DROP TABLE IF EXISTS refunds;
DROP TABLE IF EXISTS payments;

DROP TYPE IF EXISTS payout_status;
DROP TYPE IF EXISTS refund_status;
DROP TYPE IF EXISTS refund_rule;
DROP TYPE IF EXISTS refund_reason;
DROP TYPE IF EXISTS payment_method;
DROP TYPE IF EXISTS payment_status;

DROP FUNCTION IF EXISTS update_updated_at_column();
