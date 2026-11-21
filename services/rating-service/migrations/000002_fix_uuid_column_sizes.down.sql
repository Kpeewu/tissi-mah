-- =============================================================================
-- Rollback : retour à VARCHAR(128) pour les colonnes ID
-- =============================================================================

ALTER TABLE ratings
    ALTER COLUMN rating_id TYPE VARCHAR(128),
    ALTER COLUMN rater_id TYPE VARCHAR(128),
    ALTER COLUMN user_rated_id TYPE VARCHAR(128);
