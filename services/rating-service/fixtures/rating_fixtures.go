package fixtures

import (
	"context"
	"time"

	"github.com/Kpeewu/tissi-mah/services/rating-service/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RatingOption func(*domain.Rating)

// WithRatingID personnalise le RatingID
func WithRatingID(ratingID string) RatingOption {
	return func(r *domain.Rating) {
		r.RatingID = ratingID
	}
}

// WithRaterID personnalise le RaterID
func WithRaterID(raterID string) RatingOption {
	return func(r *domain.Rating) {
		r.RaterID = raterID
	}
}

// WithUserRatedID personnalise le UserRatedID
func WithUserRatedID(userRatedID string) RatingOption {
	return func(r *domain.Rating) {
		r.UserRatedID = userRatedID
	}
}

// WithStars personnalise le nombre d'étoiles
func WithStars(stars int16) RatingOption {
	return func(r *domain.Rating) {
		r.NumberOfStars = stars
	}
}

// WithComment ajoute un commentaire
func WithComment(comment string) RatingOption {
	return func(r *domain.Rating) {
		r.Comment = &comment
	}
}

// WithNoComment met le commentaire à nil
func WithNoComment() RatingOption {
	return func(r *domain.Rating) {
		r.Comment = nil
	}
}

// NewTestRating crée un Rating avec des valeurs par défaut
func NewTestRating(opts ...RatingOption) *domain.Rating {
	now := time.Now().UTC()
	id := uuid.New().String()

	defaultComment := "Excellent trajet, conducteur ponctuel"

	rating := &domain.Rating{
		RatingID:      id,
		RaterID:       uuid.New().String(),
		UserRatedID:   uuid.New().String(),
		NumberOfStars: 4,
		Comment:       &defaultComment,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	// Appliquer les options personnalisées
	for _, opt := range opts {
		opt(rating)
	}

	return rating
}

// InsertRating insère un Rating dans la base de données de test
func InsertRating(ctx context.Context, pool *pgxpool.Pool, rating *domain.Rating) error {
	query := `
		INSERT INTO ratings (
			rating_id, rater_id, user_rated_id, number_of_stars, comment,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err := pool.Exec(ctx, query,
		rating.RatingID,
		rating.RaterID,
		rating.UserRatedID,
		rating.NumberOfStars,
		rating.Comment,
		rating.CreatedAt,
		rating.UpdatedAt,
	)

	return err
}
