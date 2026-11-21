-- Migration 000011 : unification du permis de conduire.
-- Le permis devient un document utilisateur unique 'driverLicence' :
--   - remplace driverLicenceFront/driverLicenceBack côté user_documents
--   - retiré de vehicle_documents (le permis appartient au conducteur, pas au véhicule)
-- Données de dev uniquement : les lignes aux anciens types sont supprimées sans conversion.

-- 1) Purger les reviews liées au permis (lève les FK user_document_id /
--    second_user_document_id / vehicle_document_id avant suppression des documents)
DELETE FROM document_reviews
WHERE logical_document_type = 'driverLicence'
   OR document_type IN ('driverLicence', 'driverLicenceFront', 'driverLicenceBack');

-- 2) Purger les documents aux anciens types
DELETE FROM user_documents    WHERE document_type IN ('driverLicenceFront', 'driverLicenceBack');
DELETE FROM vehicle_documents WHERE document_type = 'driverLicence';

-- 3) user_documents : remplacer Front/Back par 'driverLicence'
--    (drop par nom : chk_user_documents_metadata de 000009 mentionne aussi document_type)
ALTER TABLE user_documents DROP CONSTRAINT IF EXISTS user_documents_document_type_check;
ALTER TABLE user_documents ADD CONSTRAINT user_documents_document_type_check
    CHECK (document_type IN ('idCardFront', 'idCardBack', 'passport', 'driverLicence', 'profilePicture'));

-- 4) vehicle_documents : retirer 'driverLicence' (retour à l'état pré-000003)
ALTER TABLE vehicle_documents DROP CONSTRAINT IF EXISTS vehicle_documents_document_type_check;
ALTER TABLE vehicle_documents ADD CONSTRAINT vehicle_documents_document_type_check
    CHECK (document_type IN ('insurance', 'registrationCard'));
