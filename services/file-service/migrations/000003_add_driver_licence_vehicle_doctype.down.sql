-- =============================================================================
-- Rollback : retirer 'driverLicence' du CHECK document_type sur vehicle_documents
-- =============================================================================

DO $$
DECLARE
    constraint_name_var TEXT;
BEGIN
    SELECT conname INTO constraint_name_var
    FROM pg_constraint
    WHERE conrelid = 'vehicle_documents'::regclass
      AND contype  = 'c'
      AND pg_get_constraintdef(oid) LIKE '%document_type%';

    IF constraint_name_var IS NOT NULL THEN
        EXECUTE format('ALTER TABLE vehicle_documents DROP CONSTRAINT %I', constraint_name_var);
    END IF;
END $$;

ALTER TABLE vehicle_documents
    ADD CONSTRAINT vehicle_documents_document_type_check
    CHECK (document_type IN ('insurance', 'registrationCard'));
