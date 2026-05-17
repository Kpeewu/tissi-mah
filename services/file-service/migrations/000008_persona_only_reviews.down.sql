-- Rollback migration 000008
--
-- Note : restaurer ck_review_one_document n'est possible que si toutes les
-- reviews ont au moins une FK doc non-null. Les reviews créées par Persona
-- 100% (les deux FK NULL) bloqueront ce rollback — c'est intentionnel : il
-- faut les supprimer manuellement avant le down si besoin.

ALTER TABLE document_reviews
    DROP CONSTRAINT IF EXISTS ck_review_at_most_one_document;

ALTER TABLE document_reviews
    ADD CONSTRAINT ck_review_one_document CHECK (
        (user_document_id IS NOT NULL AND vehicle_document_id IS NULL) OR
        (user_document_id IS NULL AND vehicle_document_id IS NOT NULL)
    );

DROP INDEX IF EXISTS idx_document_reviews_user_id;

ALTER TABLE document_reviews
    DROP COLUMN IF EXISTS document_type;

ALTER TABLE document_reviews
    DROP COLUMN IF EXISTS user_id;
