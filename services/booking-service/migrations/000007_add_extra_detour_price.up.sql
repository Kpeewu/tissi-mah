-- =============================================================================
-- Migration 000007 : surcoût détour passager
-- =============================================================================

-- Montant XOF du surcoût lié au détour, fourni par le client à la création.
-- NULL pour les réservations antérieures à cette migration (affiché comme 0 côté app).
ALTER TABLE bookings ADD COLUMN extra_detour_price INTEGER NULL;

-- Index pour GetDriverBookings (tous statuts, trié par created_at DESC)
CREATE INDEX idx_bookings_driver_all
    ON bookings (driver_id, status, created_at DESC)
    WHERE deleted_at IS NULL;
