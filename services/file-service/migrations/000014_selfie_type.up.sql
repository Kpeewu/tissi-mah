-- Migration 000014 : nouveau type de document utilisateur 'selfie'.
--
-- Le selfie d'identité sert de photo de profil (dès l'upload, après modération)
-- ET de pièce de vérification : le support le compare à la pièce d'identité.
-- Comme profilePicture, il ne porte aucune métadonnée légale.
-- profilePicture est conservé dans la contrainte pour l'historique (lecture),
-- mais n'est plus écrit : le selfie le remplace.

ALTER TABLE user_documents DROP CONSTRAINT IF EXISTS user_documents_document_type_check;
ALTER TABLE user_documents ADD CONSTRAINT user_documents_document_type_check
    CHECK (document_type IN ('idCardFront', 'idCardBack', 'passport', 'driverLicenceFront', 'driverLicenceBack', 'profilePicture', 'selfie'));

-- Exempter le selfie des métadonnées légales obligatoires (comme profilePicture)
ALTER TABLE user_documents DROP CONSTRAINT IF EXISTS chk_user_documents_metadata;
ALTER TABLE user_documents ADD CONSTRAINT chk_user_documents_metadata CHECK (
    document_type IN ('profilePicture', 'selfie') OR (
        document_number IS NOT NULL AND document_number != '' AND
        issued_at IS NOT NULL AND
        expire_at IS NOT NULL AND
        issuing_country IS NOT NULL AND issuing_country != ''
    )
);
