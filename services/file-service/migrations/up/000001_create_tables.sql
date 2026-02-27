-- =============================================================================
-- File Service - Tables de documents et revues
-- =============================================================================

-- -------------------------
-- Table : user_documents
-- -------------------------
CREATE TABLE IF NOT EXISTS user_documents (
    document_id    VARCHAR(36) PRIMARY KEY,
    user_id        VARCHAR(36) NOT NULL,
    document_name  VARCHAR(255) NOT NULL,
    document_type  VARCHAR(50) NOT NULL
        CHECK (document_type IN ('idCardFront', 'idCardBack', 'passport', 'driverLicenceFront', 'driverLicenceBack', 'profilePicture')),
    document_url   TEXT,
    file_size_bytes BIGINT,
    mime_type      VARCHAR(100),

    -- Informations légales du document
    document_number VARCHAR(100),
    issued_at       DATE,
    expire_at       DATE,
    issuing_country VARCHAR(100),

    -- Statut de vérification
    status VARCHAR(20) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'underReview', 'approved', 'rejected', 'expired')),

    -- Gestion du remplacement de documents
    is_current  BOOLEAN NOT NULL DEFAULT true,
    replaced_by VARCHAR(36) REFERENCES user_documents(document_id),

    uploaded_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index : recherche du document courant par type pour un utilisateur
CREATE INDEX IF NOT EXISTS idx_user_documents_current
    ON user_documents (user_id, document_type) WHERE is_current = true;

-- Index : file de revue (documents en attente)
CREATE INDEX IF NOT EXISTS idx_user_documents_review_queue
    ON user_documents (status) WHERE status IN ('pending', 'underReview');

-- -------------------------
-- Table : vehicle_documents
-- -------------------------
CREATE TABLE IF NOT EXISTS vehicle_documents (
    document_id    VARCHAR(36) PRIMARY KEY,
    vehicle_id     VARCHAR(36) NOT NULL,
    document_name  VARCHAR(255) NOT NULL,
    document_type  VARCHAR(50) NOT NULL
        CHECK (document_type IN ('insurance', 'registrationCard')),
    document_url   TEXT,
    file_size_bytes BIGINT,
    mime_type      VARCHAR(100),

    -- Informations légales du document
    document_number   VARCHAR(100),
    issued_at         DATE,
    expire_at         DATE,
    issuing_authority VARCHAR(100),

    -- Statut de vérification
    status VARCHAR(20) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'underReview', 'approved', 'rejected', 'expired')),

    -- Gestion du remplacement de documents
    is_current  BOOLEAN NOT NULL DEFAULT true,
    replaced_by VARCHAR(36) REFERENCES vehicle_documents(document_id),

    uploaded_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index : recherche du document courant par type pour un véhicule
CREATE INDEX IF NOT EXISTS idx_vehicle_documents_current
    ON vehicle_documents (vehicle_id, document_type) WHERE is_current = true;

-- Index : file de revue (documents en attente)
CREATE INDEX IF NOT EXISTS idx_vehicle_documents_review_queue
    ON vehicle_documents (status) WHERE status IN ('pending', 'underReview');

-- -------------------------
-- Table : document_reviews
-- -------------------------
CREATE TABLE IF NOT EXISTS document_reviews (
    review_id          VARCHAR(36) PRIMARY KEY,
    user_document_id   VARCHAR(36) REFERENCES user_documents(document_id),
    vehicle_document_id VARCHAR(36) REFERENCES vehicle_documents(document_id),

    -- Exactement un des deux doit être non-null
    CONSTRAINT ck_review_one_document CHECK (
        (user_document_id IS NOT NULL AND vehicle_document_id IS NULL) OR
        (user_document_id IS NULL AND vehicle_document_id IS NOT NULL)
    ),

    decision          VARCHAR(20) NOT NULL
        CHECK (decision IN ('approved', 'rejected', 'resubmission')),
    reason_rejection  TEXT,
    rejection_details TEXT,

    reviewed_by      VARCHAR(255) NOT NULL,
    reviewed_by_type VARCHAR(20) NOT NULL
        CHECK (reviewed_by_type IN ('manual', 'automatic')),
    reviewed_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    notes          TEXT,
    extracted_data JSONB
);

-- Index : revues par document utilisateur
CREATE INDEX IF NOT EXISTS idx_document_reviews_user_doc
    ON document_reviews (user_document_id) WHERE user_document_id IS NOT NULL;

-- Index : revues par document véhicule
CREATE INDEX IF NOT EXISTS idx_document_reviews_vehicle_doc
    ON document_reviews (vehicle_document_id) WHERE vehicle_document_id IS NOT NULL;

-- -------------------------
-- Trigger : mise à jour automatique de updated_at
-- -------------------------
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER update_user_documents_updated_at
    BEFORE UPDATE ON user_documents
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_vehicle_documents_updated_at
    BEFORE UPDATE ON vehicle_documents
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
