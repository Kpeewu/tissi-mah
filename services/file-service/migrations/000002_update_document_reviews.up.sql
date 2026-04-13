-- =============================================================================
-- Migration : mise à jour de la table document_reviews
-- Ajout des champs Persona, webhook, retry/versioning, status, timestamps
-- =============================================================================

-- Ajout des colonnes Persona
ALTER TABLE document_reviews
    ADD COLUMN IF NOT EXISTS persona_inquiry_id    VARCHAR(255),
    ADD COLUMN IF NOT EXISTS persona_template_id   VARCHAR(255),
    ADD COLUMN IF NOT EXISTS persona_session_token  VARCHAR(255),
    ADD COLUMN IF NOT EXISTS session_expires_at     TIMESTAMPTZ;

-- Ajout des colonnes webhook
ALTER TABLE document_reviews
    ADD COLUMN IF NOT EXISTS webhook_event_type   VARCHAR(100),
    ADD COLUMN IF NOT EXISTS webhook_received_at  TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS persona_raw_payload  JSONB;

-- Ajout des colonnes retry/versioning
ALTER TABLE document_reviews
    ADD COLUMN IF NOT EXISTS attempt_number     SMALLINT NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS previous_review_id VARCHAR(36) REFERENCES document_reviews(review_id);

-- Ajout du statut de la revue
ALTER TABLE document_reviews
    ADD COLUMN IF NOT EXISTS status VARCHAR(20) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'inProgress', 'submitted', 'completed', 'expired', 'failed'));

-- Renommage reviewed_by_type → review_type
ALTER TABLE document_reviews
    RENAME COLUMN reviewed_by_type TO review_type;

-- Ajout des nouveaux timestamps
ALTER TABLE document_reviews
    ADD COLUMN IF NOT EXISTS submitted_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW();

-- Mise à jour de la contrainte reason_rejection avec les valeurs enum
-- On supprime d'abord l'ancienne contrainte si elle existe, puis on ajoute la nouvelle
DO $$
BEGIN
    -- Suppression de l'ancienne contrainte si elle existe
    IF EXISTS (SELECT 1 FROM information_schema.table_constraints
               WHERE constraint_name = 'ck_reason_rejection_values'
               AND table_name = 'document_reviews') THEN
        ALTER TABLE document_reviews DROP CONSTRAINT ck_reason_rejection_values;
    END IF;
END $$;

ALTER TABLE document_reviews
    ADD CONSTRAINT ck_reason_rejection_values CHECK (
        reason_rejection IS NULL OR reason_rejection = '' OR
        reason_rejection IN (
            'document_expired',
            'document_incomplete',
            'document_illegible',
            'photo_missmatch',
            'information_missmatch',
            'wrong_document_type',
            'other'
        )
    );

-- Index : revues par persona_inquiry_id
CREATE INDEX IF NOT EXISTS idx_document_reviews_persona_inquiry
    ON document_reviews (persona_inquiry_id) WHERE persona_inquiry_id IS NOT NULL;

-- Index : revues par statut
CREATE INDEX IF NOT EXISTS idx_document_reviews_status
    ON document_reviews (status);

-- Index : chaîne de revues (previous_review_id)
CREATE INDEX IF NOT EXISTS idx_document_reviews_previous
    ON document_reviews (previous_review_id) WHERE previous_review_id IS NOT NULL;

-- Trigger : mise à jour automatique de updated_at
CREATE TRIGGER update_document_reviews_updated_at
    BEFORE UPDATE ON document_reviews
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
