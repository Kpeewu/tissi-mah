package interfaces

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/notification-service/internal/domain"
)

type NotificationRepository interface {
	Create(ctx context.Context, n *domain.Notification) error
	UpdateStatus(ctx context.Context, notificationID, status, failureReason, providerMessageID string) error
	UpdateRetry(ctx context.Context, n *domain.Notification) error
	ExistsByEventIDAndChannel(ctx context.Context, eventID, channel string) (bool, error)
	GetPendingForRetry(ctx context.Context, limit int) ([]*domain.Notification, error)
	PurgeOldSent(ctx context.Context, days int) (int64, error)
}
