-- Migration 000017 : abandon de Persona — validation 100 % manuelle par le support.
-- Suppression des colonnes Persona/webhook de document_reviews.

DROP INDEX IF EXISTS idx_document_reviews_persona_inquiry;

ALTER TABLE document_reviews
    DROP COLUMN IF EXISTS persona_inquiry_id,
    DROP COLUMN IF EXISTS persona_template_id,
    DROP COLUMN IF EXISTS persona_session_token,
    DROP COLUMN IF EXISTS session_expires_at,
    DROP COLUMN IF EXISTS webhook_event_type,
    DROP COLUMN IF EXISTS webhook_received_at,
    DROP COLUMN IF EXISTS persona_raw_payload;

-- review_type : plus que 'manual' en écriture ; 'automatic' reste toléré
-- pour les lignes historiques (contrainte inchangée).

-- Les statuts Persona (inProgress, submitted) n'ont plus d'écrivain ;
-- les reviews actives restantes dans ces états sont ramenées à 'pending'
-- pour rester visibles dans la file support.
UPDATE document_reviews
SET status = 'pending'
WHERE status IN ('inProgress', 'submitted');
