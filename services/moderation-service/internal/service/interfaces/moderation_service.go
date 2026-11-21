package interfaces

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/moderation-service/internal/domain"
)

type ModerationService interface {
	ModerateText(ctx context.Context, contentID, contentType, text, authorID string) (*domain.ModerationResult, error)
	ModerateImage(ctx context.Context, contentID, contentType string, imageBytes []byte, mimeType, authorID string) (*domain.ModerationResult, error)
}
