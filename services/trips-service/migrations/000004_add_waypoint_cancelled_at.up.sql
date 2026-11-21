-- Ajout de cancelled_at et cancellation_reason sur trips_waypoints
-- pour permettre l'annulation soft d'un waypoint de type "stop" par le conducteur.

ALTER TABLE trips_waypoints
    ADD COLUMN IF NOT EXISTS cancelled_at        TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS cancellation_reason TEXT;
