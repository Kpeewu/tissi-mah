-- ============================================================================
-- Migration 000001 DOWN: Drop notification service tables
-- ============================================================================

DROP TRIGGER IF EXISTS set_updated_at_notifications ON notifications;
DROP TRIGGER IF EXISTS set_updated_at_device_tokens ON user_device_tokens;
DROP TRIGGER IF EXISTS set_updated_at_preferences ON user_notification_preferences;
DROP TRIGGER IF EXISTS set_updated_at_templates ON notification_templates;
DROP TRIGGER IF EXISTS set_updated_at_routing ON notification_event_routing;

DROP TABLE IF EXISTS notification_inbox;
DROP TABLE IF EXISTS notifications;
DROP TABLE IF EXISTS user_device_tokens;
DROP TABLE IF EXISTS user_notification_preferences;
DROP TABLE IF EXISTS notification_templates;
DROP TABLE IF EXISTS notification_event_routing;
