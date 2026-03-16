package interfaces

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/file-service/internal/domain"
)

type DocumentReviewRepositoryWrite interface {
	// Crée une nouvelle revue de document
	Create(ctx context.Context, review *domain.DocumentReview) (string, error)

	// Met à jour une revue existante
	Update(ctx context.Context, review *domain.DocumentReview) error
}
