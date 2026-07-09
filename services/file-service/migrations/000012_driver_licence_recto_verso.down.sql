-- Rollback 000012 : retour au permis mono-image 'driverLicence'.
-- Les permis recto/verso créés depuis la migration sont supprimés (pas de conversion).

DELETE FROM document_reviews
WHERE logical_document_type = 'driverLicence'
   OR document_type IN ('driverLicenceFront', 'driverLicenceBack');

DELETE FROM user_documents WHERE document_type IN ('driverLicenceFront', 'driverLicenceBack');

ALTER TABLE user_documents DROP CONSTRAINT IF EXISTS user_documents_document_type_check;
ALTER TABLE user_documents ADD CONSTRAINT user_documents_document_type_check
    CHECK (document_type IN ('idCardFront', 'idCardBack', 'passport', 'driverLicence', 'profilePicture'));
