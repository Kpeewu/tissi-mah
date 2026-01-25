
CREATE TABLE IF NOT EXISTS auth (
    auth_id VARCHAR(128) PRIMARY KEY, 
    firebase_id VARCHAR(128) NOT NULL UNIQUE,
    email VARCHAR(254) UNIQUE,
    phone_number VARCHAR(20) UNIQUE,
    is_active BOOLEAN NOT NULL DEFAULT true,
    is_suspended BOOLEAN NOT NULL DEFAULT false,
    suspension_end_date TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,

    -- One contact must be available
    CONSTRAINT ck_phone_or_email_not_null CHECK(email IS NOT NULL OR phone_number IS NOT NULL)
);

CREATE INDEX IF NOT EXISTS idx_auth_firebase_id ON auth(firebase_id);
CREATE INDEX IF NOT EXISTS idx_auth_email ON auth(email) WHERE email IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_auth_phone_number ON auth(phone_number) WHERE phone_number IS NOT NULL;
CREATE INDEX IF NOT EXISTS ids_auth_deleted_at ON auth(deleted_at) WHERE deleted_at IS NOT NULL;

-- Function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger to call the function
CREATE TRIGGER update_auth_updated_at
    BEFORE UPDATE ON auth
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();