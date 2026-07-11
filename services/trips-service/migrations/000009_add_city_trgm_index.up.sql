-- Index trigramme sur city sans accents pour le fuzzy matching (word_similarity)
-- Complète idx_trips_waypoints_location_name_trgm : la recherche matche désormais
-- location_name OU city.
CREATE INDEX IF NOT EXISTS idx_trips_waypoints_city_trgm
    ON trips_waypoints USING GIN (f_unaccent(city) gin_trgm_ops);
