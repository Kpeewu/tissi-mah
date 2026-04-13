-- ============================================================================
-- Migration 000001: Create notification service tables
-- ============================================================================

-- 1. notification_event_routing
CREATE TABLE IF NOT EXISTS notification_event_routing (
    routing_id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_type         VARCHAR(50) NOT NULL UNIQUE,
    send_push          BOOLEAN NOT NULL DEFAULT false,
    send_email         BOOLEAN NOT NULL DEFAULT false,
    create_inbox_entry BOOLEAN NOT NULL DEFAULT false,
    priority           VARCHAR(20) NOT NULL DEFAULT 'standard'
                       CHECK (priority IN ('critical', 'standard', 'low')),
    is_active          BOOLEAN NOT NULL DEFAULT true,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_routing_event_type ON notification_event_routing(event_type) WHERE is_active = true;

-- Seed data
INSERT INTO notification_event_routing (event_type, send_push, send_email, create_inbox_entry, priority) VALUES
    ('BOOKING_CONFIRMED',       true,  true,  true,  'critical'),
    ('BOOKING_CANCELLED',       true,  true,  true,  'critical'),
    ('PAYMENT_RECEIVED',        true,  true,  true,  'critical'),
    ('PAYMENT_FAILED',          true,  true,  true,  'critical'),
    ('TRIP_REMINDER',           true,  false, true,  'standard'),
    ('TRIP_STARTED',            true,  false, true,  'standard'),
    ('TRIP_COMPLETED',          true,  false, true,  'standard'),
    ('TRIP_CANCELLED',          true,  true,  true,  'critical'),
    ('TRIP_MODIFIED',           true,  false, true,  'critical'),
    ('NEW_RATING',              true,  false, true,  'low'),
    ('KYC_APPROVED',            true,  true,  false, 'standard'),
    ('KYC_REJECTED',            true,  true,  false, 'standard'),
    ('NEW_MESSAGE',             true,  false, true,  'standard'),
    ('WELCOME',                 true,  true,  false, 'low'),
    ('ACCOUNT_SUSPENDED',       true,  true,  true,  'critical'),
    ('DOCUMENT_EXPIRING_SOON',  true,  true,  true,  'standard'),
    ('PAYOUT_COMPLETED',        true,  true,  true,  'standard'),
    ('PAYOUT_FAILED',           true,  true,  true,  'critical'),
    ('DRIVER_PROFILE_VERIFIED', true,  true,  false, 'standard'),
    ('NEW_BOOKING_REQUEST',     true,  false, true,  'critical'),
    ('BOOKING_REJECTED',        true,  true,  true,  'critical'),
    ('PROMOTION',               true,  true,  false, 'low'),
    ('SYSTEM_MAINTENANCE',      true,  false, false, 'low'),
    ('SECURITY_ALERT',          true,  true,  true,  'critical')
ON CONFLICT (event_type) DO NOTHING;

-- 2. notification_templates
CREATE TABLE IF NOT EXISTS notification_templates (
    template_id   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_type    VARCHAR(50) NOT NULL,
    channel       VARCHAR(10) NOT NULL CHECK (channel IN ('push', 'email')),
    language_code VARCHAR(5)  NOT NULL DEFAULT 'fr',
    title         TEXT NOT NULL DEFAULT '',
    subject       TEXT NOT NULL DEFAULT '',
    body          TEXT NOT NULL DEFAULT '',
    body_html     TEXT NOT NULL DEFAULT '',
    is_active     BOOLEAN NOT NULL DEFAULT true,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (event_type, channel, language_code)
);

CREATE INDEX idx_template_lookup ON notification_templates(event_type, channel, language_code) WHERE is_active = true;

-- 3. user_notification_preferences
CREATE TABLE IF NOT EXISTS user_notification_preferences (
    preference_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       VARCHAR(128) NOT NULL UNIQUE,
    push_enabled  BOOLEAN NOT NULL DEFAULT true,
    email_enabled BOOLEAN NOT NULL DEFAULT true,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_preferences_user ON user_notification_preferences(user_id);

-- 4. user_device_tokens
CREATE TABLE IF NOT EXISTS user_device_tokens (
    token_id    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     VARCHAR(128) NOT NULL,
    fcm_token   TEXT NOT NULL UNIQUE,
    platform    VARCHAR(10) NOT NULL CHECK (platform IN ('android', 'ios', 'web')),
    device_name VARCHAR(100) NOT NULL DEFAULT '',
    is_active   BOOLEAN NOT NULL DEFAULT true,
    last_used_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_device_tokens_user ON user_device_tokens(user_id) WHERE is_active = true;
CREATE INDEX idx_device_tokens_fcm ON user_device_tokens(fcm_token);

-- 5. notifications (journal technique d'envoi)
CREATE TABLE IF NOT EXISTS notifications (
    notification_id    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id           VARCHAR(128) NOT NULL,
    user_id            VARCHAR(128) NOT NULL,
    event_type         VARCHAR(50) NOT NULL,
    channel            VARCHAR(10) NOT NULL CHECK (channel IN ('push', 'email')),
    template_id        UUID,
    resolved_title     TEXT NOT NULL DEFAULT '',
    resolved_subject   TEXT NOT NULL DEFAULT '',
    resolved_body      TEXT NOT NULL DEFAULT '',
    recipient_address  TEXT NOT NULL DEFAULT '',
    status             VARCHAR(20) NOT NULL DEFAULT 'pending'
                       CHECK (status IN ('pending', 'processing', 'sent', 'delivered', 'failed', 'cancelled')),
    failure_reason     TEXT NOT NULL DEFAULT '',
    reference_id       VARCHAR(128) NOT NULL DEFAULT '',
    reference_type     VARCHAR(50) NOT NULL DEFAULT '',
    attempt_count      SMALLINT NOT NULL DEFAULT 0,
    max_attempts       SMALLINT NOT NULL DEFAULT 3,
    next_attempt_at    TIMESTAMPTZ,
    last_attempt_at    TIMESTAMPTZ,
    provider_name      VARCHAR(20) NOT NULL DEFAULT '',
    provider_message_id VARCHAR(256) NOT NULL DEFAULT '',
    sent_at            TIMESTAMPTZ,
    delivered_at       TIMESTAMPTZ,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Déduplication : un event_id + channel = une seule notification
CREATE UNIQUE INDEX idx_notifications_dedup ON notifications(event_id, channel);
CREATE INDEX idx_notifications_user ON notifications(user_id);
CREATE INDEX idx_notifications_status ON notifications(status) WHERE status IN ('pending', 'failed');
CREATE INDEX idx_notifications_retry ON notifications(next_attempt_at) WHERE status IN ('pending', 'failed') AND next_attempt_at IS NOT NULL;
CREATE INDEX idx_notifications_created ON notifications(created_at);

-- 6. notification_inbox
CREATE TABLE IF NOT EXISTS notification_inbox (
    inbox_id        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         VARCHAR(128) NOT NULL,
    event_type      VARCHAR(50) NOT NULL,
    title           TEXT NOT NULL DEFAULT '',
    body            TEXT NOT NULL DEFAULT '',
    action_type     VARCHAR(50) NOT NULL DEFAULT '',
    action_id       VARCHAR(128) NOT NULL DEFAULT '',
    is_read         BOOLEAN NOT NULL DEFAULT false,
    read_at         TIMESTAMPTZ,
    notification_id UUID,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_inbox_user ON notification_inbox(user_id, created_at DESC);
CREATE INDEX idx_inbox_unread ON notification_inbox(user_id) WHERE is_read = false;
CREATE INDEX idx_inbox_created ON notification_inbox(created_at);

-- Trigger updated_at automatique
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER set_updated_at_routing BEFORE UPDATE ON notification_event_routing FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER set_updated_at_templates BEFORE UPDATE ON notification_templates FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER set_updated_at_preferences BEFORE UPDATE ON user_notification_preferences FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER set_updated_at_device_tokens BEFORE UPDATE ON user_device_tokens FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER set_updated_at_notifications BEFORE UPDATE ON notifications FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
