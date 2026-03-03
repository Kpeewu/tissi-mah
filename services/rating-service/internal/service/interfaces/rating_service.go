package interfaces

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/rating-service/internal/domain"
)

type RatingService interface {
	// Créer une note (raterID extrait du contexte JWT)
	CreateRating(ctx context.Context, userRatedID string, numberOfStars int16, comment string) (*domain.Rating, error)

	// Récupérer une note par son ID
	GetRating(ctx context.Context, ratingID string) (*domain.Rating, error)

	// Récupérer toutes les notes reçues par un utilisateur
	GetRatingsForUser(ctx context.Context, userRatedID string) ([]*domain.Rating, error)

	// Récupérer la moyenne et le nombre total de notes d'un utilisateur
	GetAverageRating(ctx context.Context, userRatedID string) (float64, int32, error)

	// Modifier une note (seul le rater peut modifier)
	UpdateRating(ctx context.Context, ratingID string, numberOfStars int16, comment string) (*domain.Rating, error)

	// Supprimer une note (seul le rater peut supprimer)
	DeleteRating(ctx context.Context, ratingID string) error
}
