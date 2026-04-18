CREATE TABLE payout_status_history (
    history_id          VARCHAR(36)  NOT NULL PRIMARY KEY,
    payout_id           VARCHAR(36)  NOT NULL REFERENCES payouts(payout_id),
    status              VARCHAR(64)  NOT NULL,      -- scheduled | processing | failed | completed | launched_by_support
    initiated_by        VARCHAR(16)  NOT NULL DEFAULT 'system',  -- "system" | "support"
    support_user_id     VARCHAR(36),                -- NULL si initiated_by = "system"
    support_first_name  VARCHAR(128),               -- NULL si initiated_by = "system"
    support_last_name   VARCHAR(128),               -- NULL si initiated_by = "system"
    occurred_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    notes               TEXT
);

CREATE INDEX idx_payout_status_history_payout_id ON payout_status_history(payout_id);
CREATE INDEX idx_payout_status_history_initiated_by ON payout_status_history(initiated_by);
