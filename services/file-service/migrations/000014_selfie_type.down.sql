-- Purge des selfies avant restauration des contraintes (clean break, préprod)
DELETE FROM document_reviews
WHERE document_type = 'selfie' OR logical_document_type = 'selfie';
DELETE FROM user_documents WHERE document_type = 'selfie';

ALTER TABLE user_documents DROP CONSTRAINT IF EXISTS user_documents_document_type_check;
ALTER TABLE user_documents ADD CONSTRAINT user_documents_document_type_check
    CHECK (document_type IN ('idCardFront', 'idCardBack', 'passport', 'driverLicenceFront', 'driverLicenceBack', 'profilePicture'));

ALTER TABLE user_documents DROP CONSTRAINT IF EXISTS chk_user_documents_metadata;
ALTER TABLE user_documents ADD CONSTRAINT chk_user_documents_metadata CHECK (
    document_type = 'profilePicture' OR (
        document_number IS NOT NULL AND document_number != '' AND
        issued_at IS NOT NULL AND
        expire_at IS NOT NULL AND
        issuing_country IS NOT NULL AND issuing_country != ''
    )
);
