-- Rollback 000011 : re-séparation du permis (user Front/Back + vehicle driverLicence).
-- Les documents utilisateur 'driverLicence' et leurs reviews sont supprimés (dev only).

DELETE FROM document_reviews
WHERE logical_document_type = 'driverLicence'
   OR document_type = 'driverLicence';

DELETE FROM user_documents WHERE document_type = 'driverLicence';

ALTER TABLE user_documents DROP CONSTRAINT IF EXISTS user_documents_document_type_check;
ALTER TABLE user_documents ADD CONSTRAINT user_documents_document_type_check
    CHECK (document_type IN ('idCardFront', 'idCardBack', 'passport', 'driverLicenceFront', 'driverLicenceBack', 'profilePicture'));

ALTER TABLE vehicle_documents DROP CONSTRAINT IF EXISTS vehicle_documents_document_type_check;
ALTER TABLE vehicle_documents ADD CONSTRAINT vehicle_documents_document_type_check
    CHECK (document_type IN ('insurance', 'registrationCard', 'driverLicence'));
