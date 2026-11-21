DROP INDEX IF EXISTS idx_trips_waypoints_booked_seats;
ALTER TABLE trips_waypoints DROP CONSTRAINT IF EXISTS ck_trips_waypoints_booked_seats_positive;
ALTER TABLE trips_waypoints DROP COLUMN IF EXISTS booked_seats;
