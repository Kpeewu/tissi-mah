ALTER TABLE auth ADD COLUMN IF NOT EXISTS is_banned BOOLEAN NOT NULL DEFAULT false;

CREATE INDEX IF NOT EXISTS idx_auth_is_banned ON auth(is_banned) WHERE is_banned = true;
