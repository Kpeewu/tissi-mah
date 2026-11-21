package fixtures

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Kpeewu/tissi-mah/services/notification-service/internal/domain"
)

// =============================================================================
// EventRouting fixtures
// =============================================================================

type RoutingOption func(*domain.EventRouting)

func WithRoutingEventType(et string) RoutingOption {
	return func(r *domain.EventRouting) { r.EventType = et }
}

func WithRoutingPush(v bool) RoutingOption {
	return func(r *domain.EventRouting) { r.SendPush = v }
}

func WithRoutingEmail(v bool) RoutingOption {
	return func(r *domain.EventRouting) { r.SendEmail = v }
}

func WithRoutingInbox(v bool) RoutingOption {
	return func(r *domain.EventRouting) { r.CreateInboxEntry = v }
}

func WithRoutingPriority(p string) RoutingOption {
	return func(r *domain.EventRouting) { r.Priority = p }
}

func NewTestRouting(opts ...RoutingOption) *domain.EventRouting {
	now := time.Now().UTC()
	r := &domain.EventRouting{
		RoutingID:        uuid.New().String(),
		EventType:        "TEST_EVENT_" + uuid.New().String()[:8],
		SendPush:         true,
		SendEmail:        true,
		CreateInboxEntry: true,
		Priority:         "standard",
		IsActive:         true,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

func InsertRouting(ctx context.Context, pool *pgxpool.Pool, r *domain.EventRouting) error {
	_, err := pool.Exec(ctx, `
		INSERT INTO notification_event_routing
			(routing_id, event_type, send_push, send_email, create_inbox_entry, priority, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (event_type) DO UPDATE SET
			send_push = EXCLUDED.send_push,
			send_email = EXCLUDED.send_email,
			create_inbox_entry = EXCLUDED.create_inbox_entry,
			priority = EXCLUDED.priority`,
		r.RoutingID, r.EventType, r.SendPush, r.SendEmail, r.CreateInboxEntry,
		r.Priority, r.IsActive, r.CreatedAt, r.UpdatedAt,
	)
	return err
}

// =============================================================================
// Template fixtures
// =============================================================================

type TemplateOption func(*domain.Template)

func WithTemplateEventType(et string) TemplateOption {
	return func(t *domain.Template) { t.EventType = et }
}

func WithTemplateChannel(c string) TemplateOption {
	return func(t *domain.Template) { t.Channel = c }
}

func WithTemplateLang(lang string) TemplateOption {
	return func(t *domain.Template) { t.LanguageCode = lang }
}

func NewTestTemplate(opts ...TemplateOption) *domain.Template {
	now := time.Now().UTC()
	t := &domain.Template{
		TemplateID:   uuid.New().String(),
		EventType:    "TEST_EVENT_" + uuid.New().String()[:8],
		Channel:      "push",
		LanguageCode: "fr",
		Title:        "Titre test {{name}}",
		Subject:      "Sujet test",
		Body:         "Corps du message {{info}}",
		BodyHTML:     "<p>Corps HTML {{info}}</p>",
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

func InsertTemplate(ctx context.Context, pool *pgxpool.Pool, t *domain.Template) error {
	_, err := pool.Exec(ctx, `
		INSERT INTO notification_templates
			(template_id, event_type, channel, language_code, title, subject, body, body_html, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		t.TemplateID, t.EventType, t.Channel, t.LanguageCode,
		t.Title, t.Subject, t.Body, t.BodyHTML, t.IsActive, t.CreatedAt, t.UpdatedAt,
	)
	return err
}

// =============================================================================
// UserNotificationPreference fixtures
// =============================================================================

type PreferenceOption func(*domain.UserNotificationPreference)

func WithPrefUserID(uid string) PreferenceOption {
	return func(p *domain.UserNotificationPreference) { p.UserID = uid }
}

func WithPrefPushDisabled() PreferenceOption {
	return func(p *domain.UserNotificationPreference) { p.PushEnabled = false }
}

func WithPrefEmailDisabled() PreferenceOption {
	return func(p *domain.UserNotificationPreference) { p.EmailEnabled = false }
}

func NewTestPreference(opts ...PreferenceOption) *domain.UserNotificationPreference {
	now := time.Now().UTC()
	p := &domain.UserNotificationPreference{
		PreferenceID: uuid.New().String(),
		UserID:       "user-" + uuid.New().String()[:8],
		PushEnabled:  true,
		EmailEnabled: true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func InsertPreference(ctx context.Context, pool *pgxpool.Pool, p *domain.UserNotificationPreference) error {
	_, err := pool.Exec(ctx, `
		INSERT INTO user_notification_preferences
			(preference_id, user_id, push_enabled, email_enabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		p.PreferenceID, p.UserID, p.PushEnabled, p.EmailEnabled, p.CreatedAt, p.UpdatedAt,
	)
	return err
}

// =============================================================================
// UserDeviceToken fixtures
// =============================================================================

type DeviceTokenOption func(*domain.UserDeviceToken)

func WithDeviceUserID(uid string) DeviceTokenOption {
	return func(d *domain.UserDeviceToken) { d.UserID = uid }
}

func WithDeviceFCMToken(token string) DeviceTokenOption {
	return func(d *domain.UserDeviceToken) { d.FCMToken = token }
}

func WithDevicePlatform(platform string) DeviceTokenOption {
	return func(d *domain.UserDeviceToken) { d.Platform = platform }
}

func WithDeviceInactive() DeviceTokenOption {
	return func(d *domain.UserDeviceToken) { d.IsActive = false }
}

func NewTestDeviceToken(opts ...DeviceTokenOption) *domain.UserDeviceToken {
	now := time.Now().UTC()
	d := &domain.UserDeviceToken{
		TokenID:    uuid.New().String(),
		UserID:     "user-" + uuid.New().String()[:8],
		FCMToken:   "fcm-token-" + uuid.New().String(),
		Platform:   "android",
		DeviceName: "Test Device",
		IsActive:   true,
		LastUsedAt: now,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

func InsertDeviceToken(ctx context.Context, pool *pgxpool.Pool, d *domain.UserDeviceToken) error {
	_, err := pool.Exec(ctx, `
		INSERT INTO user_device_tokens
			(token_id, user_id, fcm_token, platform, device_name, is_active, last_used_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		d.TokenID, d.UserID, d.FCMToken, d.Platform, d.DeviceName,
		d.IsActive, d.LastUsedAt, d.CreatedAt, d.UpdatedAt,
	)
	return err
}

// =============================================================================
// Notification fixtures
// =============================================================================

type NotificationOption func(*domain.Notification)

func WithNotifEventID(id string) NotificationOption {
	return func(n *domain.Notification) { n.EventID = id }
}

func WithNotifChannel(ch string) NotificationOption {
	return func(n *domain.Notification) { n.Channel = ch }
}

func WithNotifStatus(s string) NotificationOption {
	return func(n *domain.Notification) { n.Status = s }
}

func WithNotifUserID(uid string) NotificationOption {
	return func(n *domain.Notification) { n.UserID = uid }
}

func WithNotifAttemptCount(a int16) NotificationOption {
	return func(n *domain.Notification) { n.AttemptCount = a }
}

func WithNotifMaxAttempts(m int16) NotificationOption {
	return func(n *domain.Notification) { n.MaxAttempts = m }
}

func NewTestNotification(opts ...NotificationOption) *domain.Notification {
	now := time.Now().UTC()
	n := &domain.Notification{
		NotificationID: uuid.New().String(),
		EventID:        "evt-" + uuid.New().String(),
		UserID:         "user-" + uuid.New().String()[:8],
		EventType:      "TEST_EVENT",
		Channel:        "push",
		TemplateID:     uuid.New().String(),
		ResolvedTitle:  "Titre test",
		ResolvedBody:   "Corps test",
		Status:         "pending",
		AttemptCount:   0,
		MaxAttempts:    3,
		ProviderName:   "fcm",
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	for _, opt := range opts {
		opt(n)
	}
	return n
}

func InsertNotification(ctx context.Context, pool *pgxpool.Pool, n *domain.Notification) error {
	err := pool.QueryRow(ctx, `
		INSERT INTO notifications
			(notification_id, event_id, user_id, event_type, channel, template_id,
			 resolved_title, resolved_subject, resolved_body, recipient_address,
			 status, reference_id, reference_type, attempt_count, max_attempts,
			 provider_name, created_at, updated_at)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
		RETURNING notification_id`,
		n.EventID, n.UserID, n.EventType, n.Channel, n.TemplateID,
		n.ResolvedTitle, n.ResolvedSubject, n.ResolvedBody, n.RecipientAddress,
		n.Status, n.ReferenceID, n.ReferenceType, n.AttemptCount, n.MaxAttempts,
		n.ProviderName, n.CreatedAt, n.UpdatedAt,
	).Scan(&n.NotificationID)
	return err
}

// =============================================================================
// InboxEntry fixtures
// =============================================================================

type InboxOption func(*domain.InboxEntry)

func WithInboxUserID(uid string) InboxOption {
	return func(e *domain.InboxEntry) { e.UserID = uid }
}

func WithInboxRead() InboxOption {
	return func(e *domain.InboxEntry) { e.IsRead = true }
}

func WithInboxEventType(et string) InboxOption {
	return func(e *domain.InboxEntry) { e.EventType = et }
}

func NewTestInboxEntry(opts ...InboxOption) *domain.InboxEntry {
	now := time.Now().UTC()
	e := &domain.InboxEntry{
		InboxID:   uuid.New().String(),
		UserID:    "user-" + uuid.New().String()[:8],
		EventType: "TEST_EVENT",
		Title:     "Titre inbox test",
		Body:      "Corps inbox test",
		IsRead:    false,
		CreatedAt: now,
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

func InsertInboxEntry(ctx context.Context, pool *pgxpool.Pool, e *domain.InboxEntry) error {
	var notifID interface{}
	if e.NotificationID != "" {
		notifID = e.NotificationID
	}
	err := pool.QueryRow(ctx, `
		INSERT INTO notification_inbox
			(inbox_id, user_id, event_type, title, body, action_type, action_id, is_read, notification_id, created_at)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING inbox_id`,
		e.UserID, e.EventType, e.Title, e.Body, e.ActionType, e.ActionID,
		e.IsRead, notifID, e.CreatedAt,
	).Scan(&e.InboxID)
	return err
}
