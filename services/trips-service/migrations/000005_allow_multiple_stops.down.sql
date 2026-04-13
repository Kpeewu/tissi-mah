-- Rollback : restaure les contraintes UNIQUE globales

-- trips_waypoints
DROP INDEX IF EXISTS uq_trips_one_departure;
DROP INDEX IF EXISTS uq_trips_one_arrival;

ALTER TABLE trips_waypoints
    ADD CONSTRAINT uq_trips_waypoints_type_per_trip
        UNIQUE (trip_id, waypoint_type) DEFERRABLE INITIALLY DEFERRED;

-- waypoint_recurring_patterns
DROP INDEX IF EXISTS uq_recurring_one_departure;
DROP INDEX IF EXISTS uq_recurring_one_arrival;

ALTER TABLE waypoint_recurring_patterns
    ADD CONSTRAINT uq_waypoint_recurring_patterns_type_per_pattern
        UNIQUE (trip_pattern_id, waypoint_type) DEFERRABLE INITIALLY DEFERRED;
