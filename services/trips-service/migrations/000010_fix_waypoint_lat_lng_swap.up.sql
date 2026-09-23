-- Les colonnes location_lng / location_lat des waypoints ont été alimentées avec les
-- valeurs interverties (la longitude dans location_lat et inversement). La colonne
-- `position` (geography), elle, a toujours été construite correctement : elle sert donc
-- de référence pour réparer les deux colonnes scalaires.
UPDATE trips_waypoints
SET location_lng = ST_X(position::geometry),
    location_lat = ST_Y(position::geometry)
WHERE position IS NOT NULL
  AND (location_lng IS DISTINCT FROM ST_X(position::geometry)
       OR location_lat IS DISTINCT FROM ST_Y(position::geometry));
