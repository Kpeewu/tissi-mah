DROP INDEX IF EXISTS idx_bookings_refund_due;
ALTER TABLE bookings
    DROP COLUMN IF EXISTS refund_request_attempts,
    DROP COLUMN IF EXISTS refund_requested_at,
    DROP COLUMN IF EXISTS refund_reason;
