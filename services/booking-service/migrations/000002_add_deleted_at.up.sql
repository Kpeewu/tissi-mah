-- Ajout de deleted_at sur les tables du booking-service
-- pour le soft-delete et la conformité RGPD.

ALTER TABLE bookings
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_bookings_deleted_at
    ON bookings(deleted_at)
    WHERE deleted_at IS NULL;

ALTER TABLE bookings_segments
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_bookings_segments_deleted_at
    ON bookings_segments(deleted_at)
    WHERE deleted_at IS NULL;

ALTER TABLE bookings_status_history
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_bookings_status_history_deleted_at
    ON bookings_status_history(deleted_at)
    WHERE deleted_at IS NULL;
