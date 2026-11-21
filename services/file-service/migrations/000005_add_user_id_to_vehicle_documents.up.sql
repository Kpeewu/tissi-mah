-- Migration : ajouter user_id à vehicle_documents
--
-- Contexte : les reviews de documents véhicule ont user_document_id = NULL et
-- vehicle_document_id renseigné. La requête GetDocumentReviewsByUserID joint
-- via user_documents.user_id, donc les reviews véhicule étaient exclues.
-- Pour pouvoir récupérer les reviews véhicule par user, on dénormalise user_id
-- sur vehicle_documents (un véhicule appartient à un user dans user-service,
-- mais file-service n'a pas accès à cette table — on stocke donc user_id à
-- l'upload).

ALTER TABLE vehicle_documents
    ADD COLUMN IF NOT EXISTS user_id VARCHAR(36) NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_vehicle_documents_user_id
    ON vehicle_documents (user_id);
