-- Recréation des colonnes Persona (vides — les données sont perdues).
ALTER TABLE document_reviews
    ADD COLUMN IF NOT EXISTS persona_inquiry_id    VARCHAR(255),
    ADD COLUMN IF NOT EXISTS persona_template_id   VARCHAR(255),
    ADD COLUMN IF NOT EXISTS persona_session_token VARCHAR(1024),
    ADD COLUMN IF NOT EXISTS session_expires_at    TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS webhook_event_type    VARCHAR(100),
    ADD COLUMN IF NOT EXISTS webhook_received_at   TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS persona_raw_payload   JSONB;

CREATE INDEX IF NOT EXISTS idx_document_reviews_persona_inquiry
    ON document_reviews (persona_inquiry_id) WHERE persona_inquiry_id IS NOT NULL;
