CREATE TABLE IF NOT EXISTS moderation_logs (
    log_id       UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    content_id   VARCHAR     NOT NULL,
    content_type VARCHAR     NOT NULL,
    author_id    VARCHAR     NOT NULL,
    decision     VARCHAR     NOT NULL,
    category     VARCHAR,
    score        FLOAT,
    reason       TEXT,
    used_fallback BOOLEAN    DEFAULT FALSE,
    created_at   TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_moderation_logs_author_id  ON moderation_logs(author_id);
CREATE INDEX IF NOT EXISTS idx_moderation_logs_decision   ON moderation_logs(decision);
CREATE INDEX IF NOT EXISTS idx_moderation_logs_content_id ON moderation_logs(content_id);
CREATE INDEX IF NOT EXISTS idx_moderation_logs_created_at ON moderation_logs(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_moderation_logs_author_dec ON moderation_logs(author_id, decision);
