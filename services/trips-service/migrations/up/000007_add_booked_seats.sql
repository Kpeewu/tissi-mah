-- Compteur de places réservées par leg (waypoint → waypoint suivant).
-- booked_seats sur le waypoint d'ordre N = nombre de sièges réservés sur le leg N→N+1.
ALTER TABLE trips_waypoints
    ADD COLUMN booked_seats SMALLINT NOT NULL DEFAULT 0;

ALTER TABLE trips_waypoints
    ADD CONSTRAINT ck_trips_waypoints_booked_seats_positive
    CHECK (booked_seats >= 0);

-- Index pour la sous-requête MAX(booked_seats) dans la recherche de segments
CREATE INDEX IF NOT EXISTS idx_trips_waypoints_booked_seats
    ON trips_waypoints (trip_id, sequencer_order, booked_seats)
    WHERE cancelled_at IS NULL AND deleted_at IS NULL;
