-- Rollback 000004 : revenir au CHECK sans 'pending'.
ALTER TABLE document_reviews
    DROP CONSTRAINT IF EXISTS ck_review_decision;

ALTER TABLE document_reviews
    ADD CONSTRAINT document_reviews_decision_check CHECK (
        decision IN ('approved', 'rejected', 'resubmission')
    );
