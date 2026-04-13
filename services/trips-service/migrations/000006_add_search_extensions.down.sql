DROP INDEX IF EXISTS idx_trips_waypoints_trip_order_active;
DROP INDEX IF EXISTS idx_trips_waypoints_city_active;
DROP INDEX IF EXISTS idx_trips_waypoints_type_active;
DROP INDEX IF EXISTS idx_trips_scheduled_search;
DROP INDEX IF EXISTS idx_trips_waypoints_location_name_trgm;
DROP FUNCTION IF EXISTS f_unaccent(text);
DROP EXTENSION IF EXISTS pg_trgm;
DROP EXTENSION IF EXISTS unaccent;
