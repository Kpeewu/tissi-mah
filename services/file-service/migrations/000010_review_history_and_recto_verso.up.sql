-- =============================================================================
-- Ajout du type logique de document et du second document pour les recto-verso
-- =============================================================================

ALTER TABLE document_reviews
    ADD COLUMN logical_document_type    VARCHAR(50),
    ADD COLUMN second_user_document_id  VARCHAR(36) REFERENCES user_documents(document_id);

-- Backfill : dériver le type logique depuis le type physique existant
UPDATE document_reviews SET logical_document_type =
    CASE
        WHEN document_type IN ('idCardFront', 'idCardBack')              THEN 'idCard'
        WHEN document_type IN ('driverLicenceFront', 'driverLicenceBack') THEN 'driverLicence'
        ELSE document_type
    END;

-- Index : historique par user + type logique (requête GetDocumentReviewHistory)
CREATE INDEX idx_document_reviews_user_logical_type
    ON document_reviews (user_id, logical_document_type);

-- Index : second document (recto-verso)
CREATE INDEX idx_document_reviews_second_user_doc
    ON document_reviews (second_user_document_id)
    WHERE second_user_document_id IS NOT NULL;
