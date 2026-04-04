package interfaces

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/notification-service/internal/domain"
)

type TemplateRepository interface {
	GetByEventTypeAndChannel(ctx context.Context, eventType, channel, languageCode string) (*domain.Template, error)
}
