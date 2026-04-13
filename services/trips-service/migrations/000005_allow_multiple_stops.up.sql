-- Remplace les contraintes UNIQUE globales sur (trip_id, waypoint_type)
-- et (trip_pattern_id, waypoint_type) par des index partiels qui n'appliquent
-- l'unicité que sur les waypoints departure et arrival.
-- Les stops sont désormais autorisés en nombre quelconque.

-- trips_waypoints
ALTER TABLE trips_waypoints DROP CONSTRAINT uq_trips_waypoints_type_per_trip;

CREATE UNIQUE INDEX uq_trips_one_departure
    ON trips_waypoints (trip_id)
    WHERE waypoint_type = 'departure' AND cancelled_at IS NULL;

CREATE UNIQUE INDEX uq_trips_one_arrival
    ON trips_waypoints (trip_id)
    WHERE waypoint_type = 'arrival' AND cancelled_at IS NULL;

-- waypoint_recurring_patterns
ALTER TABLE waypoint_recurring_patterns DROP CONSTRAINT uq_waypoint_recurring_patterns_type_per_pattern;

CREATE UNIQUE INDEX uq_recurring_one_departure
    ON waypoint_recurring_patterns (trip_pattern_id)
    WHERE waypoint_type = 'departure';

CREATE UNIQUE INDEX uq_recurring_one_arrival
    ON waypoint_recurring_patterns (trip_pattern_id)
    WHERE waypoint_type = 'arrival';
