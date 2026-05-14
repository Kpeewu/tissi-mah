-- =============================================================================
-- Migration 000006 : champ message passager, détour, et index d'accès
-- =============================================================================

-- Nouvelles colonnes
ALTER TABLE bookings ADD COLUMN passenger_message    TEXT     NULL;
ALTER TABLE bookings ADD COLUMN extra_minutes_detour SMALLINT NULL;

-- Index 1 : GetDriverPendingBookings
-- Filtre sur driver_id + statut pendingApproval, trié par created_at DESC
-- Index partiel : couvre uniquement les lignes non supprimées au statut voulu
CREATE INDEX idx_bookings_driver_pending
    ON bookings (driver_id, created_at DESC)
    WHERE status = 'pendingApproval' AND deleted_at IS NULL;

-- Index 2 : GetDriverTripBookingsRaw + GetDriverTripBookingCounts
-- Accès combiné (driver_id, trip_id) pour la vue enrichie et les compteurs par statut
CREATE INDEX idx_bookings_driver_trip
    ON bookings (driver_id, trip_id)
    WHERE deleted_at IS NULL;

-- Index 3 : GetActivePassengerSummariesForTrip
-- Complémente idx_bookings_trip_status (existant) en ajoutant le filtre deleted_at
CREATE INDEX idx_bookings_trip_active
    ON bookings (trip_id, status)
    WHERE status IN ('pendingApproval', 'approved', 'inProgress') AND deleted_at IS NULL;
