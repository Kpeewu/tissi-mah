-- Suppression dans l'ordre inverse des dépendances

-- Triggers
DROP TRIGGER IF EXISTS update_waypoint_recurring_patterns_updated_at ON waypoint_recurring_patterns;
DROP TRIGGER IF EXISTS update_trips_waypoints_updated_at ON trips_waypoints;
DROP TRIGGER IF EXISTS update_trips_updated_at ON trips;
DROP TRIGGER IF EXISTS update_recurring_patterns_updated_at ON recurring_patterns;

-- Indexes
DROP INDEX IF EXISTS idx_waypoint_recurring_patterns_city_type;
DROP INDEX IF EXISTS idx_waypoint_recurring_patterns_sequence;
DROP INDEX IF EXISTS idx_trips_waypoints_scheduled_pickup;
DROP INDEX IF EXISTS idx_trips_waypoints_city_type;
DROP INDEX IF EXISTS idx_trips_waypoints_position;
DROP INDEX IF EXISTS idx_trips_waypoints_trip_sequence;
DROP INDEX IF EXISTS idx_trips_driver_status;
DROP INDEX IF EXISTS idx_trips_departure_datetime;
DROP INDEX IF EXISTS idx_trips_available_seats;
DROP INDEX IF EXISTS idx_trips_status_departure;
DROP INDEX IF EXISTS idx_trips_recurring_pattern_id;
DROP INDEX IF EXISTS idx_trips_vehicle_id;
DROP INDEX IF EXISTS idx_trips_driver_id;
DROP INDEX IF EXISTS idx_recurring_patterns_generation;
DROP INDEX IF EXISTS idx_recurring_patterns_date_range;
DROP INDEX IF EXISTS idx_recurring_patterns_recurrence_type;
DROP INDEX IF EXISTS idx_recurring_patterns_is_active;
DROP INDEX IF EXISTS idx_recurring_patterns_vehicle_id;
DROP INDEX IF EXISTS idx_recurring_patterns_driver_id;

-- Tables
DROP TABLE IF EXISTS waypoint_recurring_patterns;
DROP TABLE IF EXISTS trips_waypoints;
DROP TABLE IF EXISTS trips;
DROP TABLE IF EXISTS recurring_patterns;

-- Fonction
DROP FUNCTION IF EXISTS update_updated_at_column();

-- Énumérations
DROP TYPE IF EXISTS recurrence_type;
DROP TYPE IF EXISTS payment_method;
DROP TYPE IF EXISTS waypoint_type;
DROP TYPE IF EXISTS trip_status;

-- Extension PostGIS (commenté par défaut — partagée potentiellement avec d'autres services)
-- DROP EXTENSION IF EXISTS postgis;
