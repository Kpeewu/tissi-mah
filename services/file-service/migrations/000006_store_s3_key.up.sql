ALTER TABLE user_documents    RENAME COLUMN document_url TO document_key;
ALTER TABLE vehicle_documents RENAME COLUMN document_url TO document_key;

-- Backfill : extraire la clé S3 depuis l'URL complète stockée.
-- Format MinIO : http://endpoint/bucket/<key>
-- Format S3    : https://bucket.s3.amazonaws.com/<key>
UPDATE user_documents
SET document_key = REGEXP_REPLACE(document_key, '^https?://[^/]+/[^/]+/', '')
WHERE document_key LIKE 'http%';

UPDATE vehicle_documents
SET document_key = REGEXP_REPLACE(document_key, '^https?://[^/]+/[^/]+/', '')
WHERE document_key LIKE 'http%';
