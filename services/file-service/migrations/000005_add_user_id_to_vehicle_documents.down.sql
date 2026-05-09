DROP INDEX IF EXISTS idx_vehicle_documents_user_id;

ALTER TABLE vehicle_documents
    DROP COLUMN IF EXISTS user_id;
