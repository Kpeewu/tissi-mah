package provider

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/moderation-service/internal/domain"
)

// TextModerator analyse un texte via une API externe.
type TextModerator interface {
	// ModerateText retourne un score de toxicité 0.0-1.0 et la catégorie détectée.
	// Retourne (0, NONE, nil) si l'API est indisponible ou ne détecte rien.
	ModerateText(ctx context.Context, text string) (score float32, category domain.Category, err error)
}

// ImageModerator analyse une image via une API externe.
type ImageModerator interface {
	// ModerateImage retourne un score 0.0-1.0 et la catégorie détectée.
	ModerateImage(ctx context.Context, imageBytes []byte, mimeType string) (score float32, category domain.Category, err error)
}
