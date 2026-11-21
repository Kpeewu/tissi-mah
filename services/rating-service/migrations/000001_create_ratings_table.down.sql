
-- Rollback indexes
DROP INDEX IF EXISTS idx_ratings_rater_id;
DROP INDEX IF EXISTS idx_ratings_user_rated_id;

-- Rollback triggers
DROP TRIGGER IF EXISTS update_ratings_updated_at ON ratings;
DROP FUNCTION IF EXISTS update_ratings_updated_at_column();

-- Rollback table
DROP TABLE IF EXISTS ratings;
