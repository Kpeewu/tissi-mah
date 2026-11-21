package interfaces

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/notification-service/internal/domain"
)

// NotificationService définit les opérations métier du service de notification.
type NotificationService interface {
	// Inbox
	GetInbox(ctx context.Context, userID string, page, pageSize int) ([]*domain.InboxEntry, int, error)
	MarkAsRead(ctx context.Context, inboxID, userID string) error
	MarkAllAsRead(ctx context.Context, userID string) (int, error)
	GetUnreadCount(ctx context.Context, userID string) (int, error)

	// Preferences
	GetPreferences(ctx context.Context, userID string) (*domain.UserNotificationPreference, error)
	UpdatePreferences(ctx context.Context, userID string, pushEnabled, emailEnabled bool) error

	// Device tokens
	RegisterDeviceToken(ctx context.Context, userID, fcmToken, platform, deviceName string) (string, error)
	UnregisterDeviceToken(ctx context.Context, fcmToken string) error
	InvalidateDeviceToken(ctx context.Context, fcmToken string) error
}
