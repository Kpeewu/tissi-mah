-- =============================================================================
-- Migration 000006 (rollback)
-- =============================================================================

DROP INDEX CONCURRENTLY IF EXISTS idx_bookings_trip_active;
DROP INDEX CONCURRENTLY IF EXISTS idx_bookings_driver_trip;
DROP INDEX CONCURRENTLY IF EXISTS idx_bookings_driver_pending;

ALTER TABLE bookings DROP COLUMN IF EXISTS extra_minutes_detour;
ALTER TABLE bookings DROP COLUMN IF EXISTS passenger_message;
