DROP INDEX IF EXISTS idx_document_reviews_second_user_doc;
DROP INDEX IF EXISTS idx_document_reviews_user_logical_type;

ALTER TABLE document_reviews
    DROP COLUMN IF EXISTS second_user_document_id,
    DROP COLUMN IF EXISTS logical_document_type;
