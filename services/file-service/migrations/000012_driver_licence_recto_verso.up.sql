-- Migration 000012 : le permis de conduire redevient recto-verso.
-- Le permis reste un document utilisateur (jamais véhicule) mais exige désormais
-- deux faces : driverLicenceFront / driverLicenceBack, appariées via
-- logical_document_type = 'driverLicence' (même mécanique que la carte d'identité).
-- Clean break : les permis mono-image existants et leurs reviews sont supprimés,
-- les utilisateurs concernés devront re-uploader recto + verso.

-- 1) Purger les reviews liées au permis (lève les FK user_document_id /
--    second_user_document_id avant suppression des documents)
DELETE FROM document_reviews
WHERE logical_document_type = 'driverLicence'
   OR document_type IN ('driverLicence', 'driverLicenceFront', 'driverLicenceBack');

-- 2) Purger les permis mono-image
DELETE FROM user_documents WHERE document_type = 'driverLicence';

-- 3) user_documents : remplacer 'driverLicence' par Front/Back
ALTER TABLE user_documents DROP CONSTRAINT IF EXISTS user_documents_document_type_check;
ALTER TABLE user_documents ADD CONSTRAINT user_documents_document_type_check
    CHECK (document_type IN ('idCardFront', 'idCardBack', 'passport', 'driverLicenceFront', 'driverLicenceBack', 'profilePicture'));
