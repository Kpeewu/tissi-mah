
-- Rollback indexes
DROP INDEX IF EXISTS idx_vehicles_user_id;
DROP INDEX IF EXISTS idx_vehicles_licence_plate;

-- Rollback triggers
DROP TRIGGER IF EXISTS update_vehicles_updated_at ON vehicles;
DROP FUNCTION IF EXISTS update_updated_at_column();

-- Rollback table
DROP TABLE IF EXISTS vehicles;
