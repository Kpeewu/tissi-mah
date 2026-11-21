package interfaces

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/rating-service/internal/domain"
)

type RatingRepositoryRead interface {
	// get a rating by its id
	GetByID(ctx context.Context, ratingID string) (*domain.Rating, error)

	// get all ratings received by a user
	GetByUserRatedID(ctx context.Context, userRatedID string) ([]*domain.Rating, error)

	// get a specific rating given by rater to rated user
	GetByRaterAndUserRated(ctx context.Context, raterID string, userRatedID string) (*domain.Rating, error)

	// check if a rating already exists for this rater/rated pair
	ExistsByRaterAndUserRated(ctx context.Context, raterID string, userRatedID string) (bool, error)

	// get the average rating and total count for a user
	GetAverageByUserRatedID(ctx context.Context, userRatedID string) (float64, int32, error)
}
