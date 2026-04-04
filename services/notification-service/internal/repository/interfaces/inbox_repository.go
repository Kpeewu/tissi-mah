package interfaces

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/notification-service/internal/domain"
)

type InboxRepository interface {
	Create(ctx context.Context, entry *domain.InboxEntry) error
	GetByUserID(ctx context.Context, userID string, page, pageSize int) ([]*domain.InboxEntry, int, error)
	MarkAsRead(ctx context.Context, inboxID, userID string) error
	MarkAllAsRead(ctx context.Context, userID string) (int, error)
	GetUnreadCount(ctx context.Context, userID string) (int, error)
	PurgeOldRead(ctx context.Context, days int) (int64, error)
}
