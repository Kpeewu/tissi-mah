DROP INDEX IF EXISTS idx_bookings_driver_all;
ALTER TABLE bookings DROP COLUMN IF EXISTS extra_detour_price;
