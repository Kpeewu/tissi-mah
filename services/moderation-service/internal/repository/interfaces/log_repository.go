package interfaces

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/moderation-service/internal/domain"
)

type ModerationLogRepository interface {
	Create(ctx context.Context, log *domain.ModerationLog) error
}
