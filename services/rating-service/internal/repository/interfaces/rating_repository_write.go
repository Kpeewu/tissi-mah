package interfaces

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/rating-service/internal/domain"
)

type RatingRepositoryWrite interface {
	// create a new rating
	Create(ctx context.Context, rating *domain.Rating) (string, error)

	// update an existing rating
	Update(ctx context.Context, rating *domain.Rating) (*domain.Rating, error)
}
