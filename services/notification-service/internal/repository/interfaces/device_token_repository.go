package interfaces

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/notification-service/internal/domain"
)

type DeviceTokenRepository interface {
	GetActiveByUserID(ctx context.Context, userID string) ([]*domain.UserDeviceToken, error)
	Upsert(ctx context.Context, token *domain.UserDeviceToken) (string, error)
	InvalidateByFCMToken(ctx context.Context, fcmToken string) error
	DeleteByFCMToken(ctx context.Context, fcmToken string) error
}
