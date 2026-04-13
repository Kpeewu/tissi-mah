-- Revert: suppression des colonnes deleted_at ajoutées par 000003

DROP INDEX IF EXISTS idx_waypoint_recurring_patterns_deleted_at;
ALTER TABLE waypoint_recurring_patterns DROP COLUMN IF EXISTS deleted_at;

DROP INDEX IF EXISTS idx_trips_waypoints_deleted_at;
ALTER TABLE trips_waypoints DROP COLUMN IF EXISTS deleted_at;
