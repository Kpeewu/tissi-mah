-- =============================================================================
-- Rollback : retour à l'ancien schéma document_reviews
-- =============================================================================

DROP TRIGGER IF EXISTS update_document_reviews_updated_at ON document_reviews;

DROP INDEX IF EXISTS idx_document_reviews_previous;
DROP INDEX IF EXISTS idx_document_reviews_status;
DROP INDEX IF EXISTS idx_document_reviews_persona_inquiry;

ALTER TABLE document_reviews DROP CONSTRAINT IF EXISTS ck_reason_rejection_values;

ALTER TABLE document_reviews
    DROP COLUMN IF EXISTS updated_at,
    DROP COLUMN IF EXISTS submitted_at,
    DROP COLUMN IF EXISTS status,
    DROP COLUMN IF EXISTS previous_review_id,
    DROP COLUMN IF EXISTS attempt_number,
    DROP COLUMN IF EXISTS persona_raw_payload,
    DROP COLUMN IF EXISTS webhook_received_at,
    DROP COLUMN IF EXISTS webhook_event_type,
    DROP COLUMN IF EXISTS session_expires_at,
    DROP COLUMN IF EXISTS persona_session_token,
    DROP COLUMN IF EXISTS persona_template_id,
    DROP COLUMN IF EXISTS persona_inquiry_id;

ALTER TABLE document_reviews
    RENAME COLUMN review_type TO reviewed_by_type;
