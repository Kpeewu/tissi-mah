-- Ajout de actual_arrival_datetime sur trips_waypoints pour les waypoints de type "stop".
-- Permet de distinguer l'arrivée du chauffeur au stop (actual_arrival_datetime)
-- du départ du stop avec les passagers (actual_scheduled_pickup_datetime).
ALTER TABLE trips_waypoints
    ADD COLUMN IF NOT EXISTS actual_arrival_datetime TIMESTAMPTZ;
