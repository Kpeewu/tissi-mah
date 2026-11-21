-- Drop triggers
DROP TRIGGER IF EXISTS update_vehicle_documents_updated_at ON vehicle_documents;
DROP TRIGGER IF EXISTS update_user_documents_updated_at ON user_documents;

-- Drop tables (ordre inversé pour les FK)
DROP TABLE IF EXISTS document_reviews;
DROP TABLE IF EXISTS vehicle_documents;
DROP TABLE IF EXISTS user_documents;
