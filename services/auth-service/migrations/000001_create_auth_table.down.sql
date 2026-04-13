
-- Rollback indexes
DROP INDEX IF EXISTS idx_auth_firebase_id;
DROP INDEX IF EXISTS idx_auth_email;
DROP INDEX IF EXISTS idx_auth_phone_number;
DROP INDEX IF EXISTS ids_auth_deleted_at;

-- Rollback triggers
DROP TRIGGER IF EXISTS update_auth_updated_at ON auth;
DROP FUNCTION IF EXISTS update_updated_at_column();

-- Rollback table
DROP TABLE IF EXISTS auth;