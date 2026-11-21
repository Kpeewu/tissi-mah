DROP INDEX IF EXISTS idx_bookings_payment_release_pending;
ALTER TABLE bookings DROP COLUMN IF EXISTS payment_released_at;
