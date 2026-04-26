-- Migration 000004 : ajouter 'pending' aux valeurs autorisées pour decision.
--
-- Contexte : à la création d'une inquiry, la review est insérée avec
-- decision = 'pending' (aucune décision Persona encore). La décision
-- change vers 'approved' / 'rejected' / 'resubmission' après le webhook.
--
-- On remplace la contrainte CHECK inline par une contrainte nommée
-- pour pouvoir la supprimer proprement dans le down.

ALTER TABLE document_reviews
    DROP CONSTRAINT IF EXISTS document_reviews_decision_check;

ALTER TABLE document_reviews
    ADD CONSTRAINT ck_review_decision CHECK (
        decision IN ('pending', 'approved', 'rejected', 'resubmission')
    );
