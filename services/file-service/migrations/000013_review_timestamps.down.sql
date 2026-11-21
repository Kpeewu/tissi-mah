-- Rétablir reviewed_at NOT NULL DEFAULT NOW() et supprimer created_at

UPDATE document_reviews
SET reviewed_at = COALESCE(reviewed_at, updated_at, NOW())
WHERE reviewed_at IS NULL;

ALTER TABLE document_reviews
    ALTER COLUMN reviewed_at SET NOT NULL,
    ALTER COLUMN reviewed_at SET DEFAULT NOW();

ALTER TABLE document_reviews
    DROP COLUMN IF EXISTS created_at;
