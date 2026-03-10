
CREATE TABLE IF NOT EXISTS vehicles (
    vehicle_id      TEXT PRIMARY KEY,
    user_id         TEXT NOT NULL,
    brand           TEXT NOT NULL,
    number_of_seats SMALLINT NOT NULL,
    brand_model     TEXT NOT NULL,
    color           TEXT NOT NULL,
    licence_plate   TEXT NOT NULL UNIQUE,
    is_verified     BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_vehicles_user_id ON vehicles(user_id);
CREATE INDEX IF NOT EXISTS idx_vehicles_licence_plate ON vehicles(licence_plate);

-- Fonction pour mettre à jour updated_at automatiquement
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger pour appeler la fonction
CREATE TRIGGER update_vehicles_updated_at
    BEFORE UPDATE ON vehicles
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
