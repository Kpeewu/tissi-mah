package interfaces

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/notification-service/internal/domain"
)

type RoutingRepository interface {
	GetByEventType(ctx context.Context, eventType string) (*domain.EventRouting, error)
	GetAll(ctx context.Context) ([]*domain.EventRouting, error)
}
