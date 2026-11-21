package interfaces

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/notification-service/internal/domain"
)

type PreferenceRepository interface {
	GetByUserID(ctx context.Context, userID string) (*domain.UserNotificationPreference, error)
	Upsert(ctx context.Context, pref *domain.UserNotificationPreference) error
}
