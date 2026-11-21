DROP INDEX IF EXISTS idx_bookings_no_show_contest_pending;
ALTER TABLE bookings
    DROP COLUMN IF EXISTS no_show_resolution_note,
    DROP COLUMN IF EXISTS no_show_resolution,
    DROP COLUMN IF EXISTS no_show_resolved_by,
    DROP COLUMN IF EXISTS no_show_resolved_at,
    DROP COLUMN IF EXISTS no_show_contest_reason,
    DROP COLUMN IF EXISTS no_show_contested_by,
    DROP COLUMN IF EXISTS no_show_contested_at;
