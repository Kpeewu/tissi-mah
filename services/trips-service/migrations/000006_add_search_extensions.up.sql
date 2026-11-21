-- Extensions pour la recherche floue
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE EXTENSION IF NOT EXISTS unaccent;

-- Wrapper immutable requis pour les index GIN (unaccent est STABLE, pas IMMUTABLE)
CREATE OR REPLACE FUNCTION f_unaccent(text)
RETURNS text AS $$
  SELECT public.unaccent('public.unaccent', $1)
$$ LANGUAGE sql IMMUTABLE PARALLEL SAFE STRICT;

-- ============================================================
-- INDEX POUR LA RECHERCHE FLOUE
-- ============================================================

-- Index trigramme sur location_name sans accents pour similarity()
CREATE INDEX IF NOT EXISTS idx_trips_waypoints_location_name_trgm
    ON trips_waypoints USING GIN (f_unaccent(location_name) gin_trgm_ops);

-- ============================================================
-- INDEX COMPOSITES POUR LA RECHERCHE DE TRAJETS SCHEDULÉS
-- ============================================================

-- Index couvrant pour la requête principale : trajets scheduled avec places
-- Couvre le WHERE + ORDER BY sans aller dans la table
CREATE INDEX IF NOT EXISTS idx_trips_scheduled_search
    ON trips (departure_datetime ASC)
    WHERE status = 'scheduled' AND available_seats > 0 AND deleted_at IS NULL;

-- Index composite sur waypoints pour le join par type
-- Couvre : trip_id + waypoint_type + filtres actifs
CREATE INDEX IF NOT EXISTS idx_trips_waypoints_type_active
    ON trips_waypoints (trip_id, waypoint_type)
    WHERE cancelled_at IS NULL AND deleted_at IS NULL;

-- Index pour la recherche par ville
CREATE INDEX IF NOT EXISTS idx_trips_waypoints_city_active
    ON trips_waypoints (city, trip_id)
    WHERE cancelled_at IS NULL AND deleted_at IS NULL;

-- Index composite pour le self-join segments
CREATE INDEX IF NOT EXISTS idx_trips_waypoints_trip_order_active
    ON trips_waypoints (trip_id, sequencer_order)
    WHERE cancelled_at IS NULL AND deleted_at IS NULL;
