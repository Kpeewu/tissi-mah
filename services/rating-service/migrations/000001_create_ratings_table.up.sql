
CREATE TABLE IF NOT EXISTS ratings (
    rating_id VARCHAR(128) PRIMARY KEY,
    rater_id VARCHAR(128) NOT NULL,
    user_rated_id VARCHAR(128) NOT NULL,
    number_of_stars SMALLINT NOT NULL,
    comment TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Un utilisateur ne peut noter un autre qu'une seule fois
    CONSTRAINT uq_rater_user_rated UNIQUE (rater_id, user_rated_id),

    -- Le nombre d'étoiles doit être entre 1 et 5
    CONSTRAINT ck_number_of_stars CHECK (number_of_stars >= 1 AND number_of_stars <= 5),

    -- Un utilisateur ne peut pas se noter lui-même
    CONSTRAINT ck_no_self_rating CHECK (rater_id <> user_rated_id)
);

CREATE INDEX IF NOT EXISTS idx_ratings_rater_id ON ratings(rater_id);
CREATE INDEX IF NOT EXISTS idx_ratings_user_rated_id ON ratings(user_rated_id);

-- Function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_ratings_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger to call the function
CREATE TRIGGER update_ratings_updated_at
    BEFORE UPDATE ON ratings
    FOR EACH ROW
    EXECUTE FUNCTION update_ratings_updated_at_column();
