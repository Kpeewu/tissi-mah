CREATE TABLE IF NOT EXISTS user_violations (
    violation_id     UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id          VARCHAR     NOT NULL,
    content_type     VARCHAR     NOT NULL,
    violation_count  INT         NOT NULL,
    suspension_until TIMESTAMPTZ,
    is_permanent_ban BOOLEAN     DEFAULT FALSE,
    created_at       TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_user_violations_user_id    ON user_violations(user_id);
CREATE INDEX IF NOT EXISTS idx_user_violations_created_at ON user_violations(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_user_violations_user_type  ON user_violations(user_id, content_type);
