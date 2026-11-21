-- Ajouter les orders des waypoints pour la gestion des places par segment
ALTER TABLE bookings ADD COLUMN pickup_sequencer_order SMALLINT;
ALTER TABLE bookings ADD COLUMN dropoff_sequencer_order SMALLINT;
