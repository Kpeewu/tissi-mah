-- Revert: suppression des colonnes deleted_at ajoutées par 000002

DROP INDEX IF EXISTS idx_bookings_status_history_deleted_at;
ALTER TABLE bookings_status_history DROP COLUMN IF EXISTS deleted_at;

DROP INDEX IF EXISTS idx_bookings_segments_deleted_at;
ALTER TABLE bookings_segments DROP COLUMN IF EXISTS deleted_at;

DROP INDEX IF EXISTS idx_bookings_deleted_at;
ALTER TABLE bookings DROP COLUMN IF EXISTS deleted_at;
