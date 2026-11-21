-- Migration 000009 : contraintes CHECK sur les métadonnées légales des documents
-- Les colonnes existent depuis 000001 mais étaient nullables sans contrainte.
-- On backfille les lignes existantes avec des valeurs placeholder avant d'ajouter les contraintes.

-- Backfill user_documents (hors profilePicture qui n'a pas de métadonnées légales)
UPDATE user_documents
SET
    document_number = COALESCE(NULLIF(document_number, ''), 'LEGACY-' || document_id),
    issued_at       = COALESCE(issued_at, NOW() - INTERVAL '1 year'),
    expire_at       = COALESCE(expire_at, NOW() + INTERVAL '5 years'),
    issuing_country = COALESCE(NULLIF(issuing_country, ''), 'N/A')
WHERE document_type != 'profilePicture';

-- Règle user_documents : hors profilePicture, les 4 champs sont obligatoires
ALTER TABLE user_documents
ADD CONSTRAINT chk_user_documents_metadata CHECK (
    document_type = 'profilePicture' OR (
        document_number IS NOT NULL AND document_number != '' AND
        issued_at IS NOT NULL AND
        expire_at IS NOT NULL AND
        issuing_country IS NOT NULL AND issuing_country != ''
    )
);

-- Backfill vehicle_documents
UPDATE vehicle_documents
SET
    document_number   = COALESCE(NULLIF(document_number, ''), 'LEGACY-' || document_id),
    issued_at         = COALESCE(issued_at, NOW() - INTERVAL '1 year'),
    expire_at         = CASE
                            WHEN expire_at IS NOT NULL THEN expire_at
                            WHEN document_type != 'registrationCard' THEN NOW() + INTERVAL '5 years'
                            ELSE NULL
                        END,
    issuing_authority = COALESCE(NULLIF(issuing_authority, ''), 'N/A')
WHERE document_number IS NULL OR document_number = ''
   OR issued_at IS NULL
   OR issuing_authority IS NULL OR issuing_authority = '';

-- Règle vehicle_documents : document_number, issued_at, issuing_authority toujours obligatoires.
-- expire_at obligatoire pour driverLicence et insurance, optionnel pour registrationCard.
ALTER TABLE vehicle_documents
ADD CONSTRAINT chk_vehicle_documents_metadata CHECK (
    document_number IS NOT NULL AND document_number != '' AND
    issued_at IS NOT NULL AND
    issuing_authority IS NOT NULL AND issuing_authority != '' AND
    (document_type = 'registrationCard' OR expire_at IS NOT NULL)
);
