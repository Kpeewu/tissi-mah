-- Rollback de 000008 : supprime la colonne route_polyline.
ALTER TABLE trips
  DROP COLUMN IF EXISTS route_polyline;
