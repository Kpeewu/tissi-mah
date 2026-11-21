-- Migration 000008 : dénormaliser user_id + document_type sur document_reviews
--
-- Contexte : depuis le passage à Persona 100%, les nouvelles reviews créées
-- par kyc-service n'ont plus ni user_document_id ni vehicle_document_id (pas
-- d'upload côté app pour la pièce d'identité). Conséquences sans cette
-- migration :
--   - INSERT bloqué par ck_review_one_document (exige une FK non-null)
--   - GetByUserID (qui joint via user_documents.user_id) ne retrouve plus
--     ces reviews — ownership/status KYC cassés en cascade
--   - GetKYCStatus ne peut plus déterminer le type de document approuvé
--
-- Solution : ajouter user_id (NOT NULL) + document_type directement sur
-- document_reviews, indépendamment des FK doc. Les chemins manuels support
-- (ValidateDocument, OverrideReview) restent compatibles : les FK doc
-- existantes restent autorisées, juste plus obligatoires.

-- 1. Ajouter user_id (lien direct utilisateur, indépendant des FK doc)
ALTER TABLE document_reviews
    ADD COLUMN IF NOT EXISTS user_id VARCHAR(36);

-- 2. Ajouter document_type (pour identifier le type sans passer par user_documents)
ALTER TABLE document_reviews
    ADD COLUMN IF NOT EXISTS document_type VARCHAR(50);

-- 3. Backfill depuis les FKs existantes (reviews historiques)
UPDATE document_reviews dr
SET user_id = ud.user_id,
    document_type = ud.document_type
FROM user_documents ud
WHERE dr.user_document_id = ud.document_id
  AND dr.user_id IS NULL;

UPDATE document_reviews dr
SET user_id = vd.user_id,
    document_type = vd.document_type
FROM vehicle_documents vd
WHERE dr.vehicle_document_id = vd.document_id
  AND dr.user_id IS NULL;

-- 4. NOT NULL après backfill + index pour GetByUserID
ALTER TABLE document_reviews
    ALTER COLUMN user_id SET NOT NULL;

CREATE INDEX IF NOT EXISTS idx_document_reviews_user_id
    ON document_reviews (user_id);

-- 5. Relâcher la contrainte qui imposait au moins une FK doc
ALTER TABLE document_reviews
    DROP CONSTRAINT IF EXISTS ck_review_one_document;

-- 6. Nouvelle contrainte plus permissive : au plus un des deux non-null
--    (les deux peuvent être null pour Persona 100%)
ALTER TABLE document_reviews
    ADD CONSTRAINT ck_review_at_most_one_document CHECK (
        NOT (user_document_id IS NOT NULL AND vehicle_document_id IS NOT NULL)
    );
