ALTER TABLE trips_waypoints
    DROP COLUMN IF EXISTS cancelled_at,
    DROP COLUMN IF EXISTS cancellation_reason;
