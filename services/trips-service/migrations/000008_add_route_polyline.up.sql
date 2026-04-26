-- Migration 000008 : ajout du tracé polyline du trajet.
--
-- Calculé côté front via geolocation-service pendant la création (appel à
-- /api/v1/geolocation/route avec les waypoints). Le front envoie ensuite la
-- polyline encodée dans le payload CreateTrip et trips-service la stocke
-- pour que les passagers consultant le trajet voient le tracé sans relancer
-- un calcul OSRM.
--
-- Format : Google encoded polyline (string compact, ~1 KB pour un trajet
-- typique inter-villes).
--
-- NOT NULL avec DEFAULT '' pour ne pas casser les trajets existants au
-- moment de la migration. Les nouveaux trajets devront fournir le champ
-- (validation côté API).

ALTER TABLE trips
  ADD COLUMN route_polyline TEXT NOT NULL DEFAULT '';

COMMENT ON COLUMN trips.route_polyline IS
  'Google encoded polyline du tracé entre les waypoints, calculé par geolocation-service côté front pendant la création.';
