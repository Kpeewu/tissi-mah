package interfaces

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/rating-service/internal/domain"
)

type RatingService interface {
	// Créer une note (raterID et userRatedID fournis dans le body)
	RateUser(ctx context.Context, raterID string, userRatedID string, numberOfStars int16, comment string) (*domain.Rating, error)

	// Récupérer toutes les notes reçues par un utilisateur
	GetUserRatings(ctx context.Context, userRatedID string) ([]*domain.Rating, error)

	// Récupérer la moyenne et le nombre total de notes d'un utilisateur
	GetUserRatingsAverage(ctx context.Context, userRatedID string) (float64, int32, error)

	// Modifier une note (seul le rater peut modifier)
	UpdateRating(ctx context.Context, raterID string, ratingID string, userRatedID string, numberOfStars int16, comment string) (*domain.Rating, error)
}
