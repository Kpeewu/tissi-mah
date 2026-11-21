-- Migration 000013 : timestamps corrects sur document_reviews.
--
-- Problèmes corrigés :
--   - reviewed_at était NOT NULL DEFAULT NOW() : une review jamais décidée
--     (pending, créée à l'upload) portait quand même une date de décision.
--   - created_at n'existait pas : les réponses admin renvoyaient le zéro Go
--     ("0001-01-01T00:00:00Z") et l'ordre "dernière review" reposait sur
--     reviewed_at, faussé par le défaut.

-- 1) created_at — backfill avec reviewed_at (meilleure approximation disponible)
ALTER TABLE document_reviews
    ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ;

UPDATE document_reviews SET created_at = reviewed_at WHERE created_at IS NULL;

ALTER TABLE document_reviews
    ALTER COLUMN created_at SET NOT NULL,
    ALTER COLUMN created_at SET DEFAULT NOW();

-- 2) reviewed_at devient nullable, sans défaut : seul le moment de la décision le remplit
ALTER TABLE document_reviews
    ALTER COLUMN reviewed_at DROP NOT NULL,
    ALTER COLUMN reviewed_at DROP DEFAULT;

-- 3) Les reviews non décidées ne doivent plus paraître décidées
UPDATE document_reviews
SET reviewed_at = NULL
WHERE status <> 'completed'
  AND decision = 'pending';
