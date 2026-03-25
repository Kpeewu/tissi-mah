-- Ajout de deleted_at sur trips_waypoints et waypoint_recurring_patterns
-- pour le soft-delete (cohérence avec recurring_patterns et trips).

ALTER TABLE trips_waypoints
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_trips_waypoints_deleted_at
    ON trips_waypoints(deleted_at)
    WHERE deleted_at IS NULL;

ALTER TABLE waypoint_recurring_patterns
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_waypoint_recurring_patterns_deleted_at
    ON waypoint_recurring_patterns(deleted_at)
    WHERE deleted_at IS NULL;
