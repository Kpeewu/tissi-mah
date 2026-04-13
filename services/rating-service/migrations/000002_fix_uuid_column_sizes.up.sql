-- =============================================================================
-- Migration : alignement des colonnes ID sur VARCHAR(36) (taille standard UUID v4)
-- =============================================================================

ALTER TABLE ratings
    ALTER COLUMN rating_id TYPE VARCHAR(36),
    ALTER COLUMN rater_id TYPE VARCHAR(36),
    ALTER COLUMN user_rated_id TYPE VARCHAR(36);
