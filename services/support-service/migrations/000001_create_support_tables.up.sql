CREATE TABLE IF NOT EXISTS support_users (
    user_id              UUID PRIMARY KEY,
    email                VARCHAR(254) UNIQUE NOT NULL,
    password_hash        VARCHAR(255) NOT NULL,
    first_name           VARCHAR(100) NOT NULL,
    last_name            VARCHAR(100) NOT NULL,
    role                 VARCHAR(16)  NOT NULL CHECK (role IN ('admin','support')),
    is_active            BOOLEAN      NOT NULL DEFAULT TRUE,
    must_change_password BOOLEAN      NOT NULL DEFAULT FALSE,
    email_changed_at     TIMESTAMPTZ,
    password_changed_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    created_at           TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    deleted_at           TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_support_users_email
    ON support_users(email)
    WHERE deleted_at IS NULL;

CREATE OR REPLACE FUNCTION update_support_users_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_support_users_updated_at ON support_users;
CREATE TRIGGER trg_support_users_updated_at
BEFORE UPDATE ON support_users
FOR EACH ROW
EXECUTE FUNCTION update_support_users_updated_at();
