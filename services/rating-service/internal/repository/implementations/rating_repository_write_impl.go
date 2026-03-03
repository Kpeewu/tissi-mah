package implementations

import (
	"context"
	"errors"

	"github.com/Kpeewu/tissi-mah/services/rating-service/internal/domain"
	i "github.com/Kpeewu/tissi-mah/services/rating-service/internal/repository/interfaces"
	ratingErrors "github.com/Kpeewu/tissi-mah/services/rating-service/pkg/errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type ratingWriteRepositoryImpl struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

func NewRatingWriteRepository(pool *pgxpool.Pool, logger *zap.Logger) i.RatingRepositoryWrite {
	return &ratingWriteRepositoryImpl{
		pool:   pool,
		logger: logger,
	}
}

// create a new rating
func (r *ratingWriteRepositoryImpl) Create(ctx context.Context, rating *domain.Rating) (string, error) {
	r.logger.Debug("creating rating",
		zap.String("ratingID", rating.RatingID),
		zap.String("raterID", rating.RaterID),
		zap.String("userRatedID", rating.UserRatedID),
		zap.Int16("stars", rating.NumberOfStars),
	)

	query := `INSERT INTO ratings (rating_id, rater_id, user_rated_id, number_of_stars, comment)
			  VALUES ($1, $2, $3, $4, $5) RETURNING rating_id`

	var ratingID string
	err := r.pool.QueryRow(ctx, query,
		rating.RatingID, rating.RaterID, rating.UserRatedID,
		rating.NumberOfStars, rating.Comment,
	).Scan(&ratingID)

	if err != nil {
		r.logger.Error("insert rating failed", zap.Error(err),
			zap.String("ratingID", rating.RatingID),
		)
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ratingErrors.ErrorDataRetrievalFailed
		}
		return "", ratingErrors.ErrorInternalServer
	}

	r.logger.Info("rating created", zap.String("ratingID", ratingID))
	return ratingID, nil
}

// update an existing rating
func (r *ratingWriteRepositoryImpl) Update(ctx context.Context, rating *domain.Rating) (*domain.Rating, error) {
	r.logger.Debug("updating rating", zap.String("ratingID", rating.RatingID))

	query := `UPDATE ratings SET
					number_of_stars = $1, comment = $2, updated_at = NOW()
			  WHERE rating_id = $3
			  RETURNING rating_id, rater_id, user_rated_id, number_of_stars, comment, created_at, updated_at`

	updated := &domain.Rating{}
	err := r.pool.QueryRow(ctx, query,
		rating.NumberOfStars, rating.Comment, rating.RatingID,
	).Scan(
		&updated.RatingID,
		&updated.RaterID,
		&updated.UserRatedID,
		&updated.NumberOfStars,
		&updated.Comment,
		&updated.CreatedAt,
		&updated.UpdatedAt,
	)

	if err != nil {
		r.logger.Error("update rating failed", zap.Error(err), zap.String("ratingID", rating.RatingID))
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ratingErrors.ErrorRatingNotFound
		}
		return nil, ratingErrors.ErrorInternalServer
	}

	r.logger.Info("rating updated", zap.String("ratingID", rating.RatingID))
	return updated, nil
}

// delete a rating
func (r *ratingWriteRepositoryImpl) Delete(ctx context.Context, ratingID string) error {
	r.logger.Debug("deleting rating", zap.String("ratingID", ratingID))

	query := `DELETE FROM ratings WHERE rating_id = $1`

	result, err := r.pool.Exec(ctx, query, ratingID)

	if err != nil {
		r.logger.Error("delete rating failed", zap.Error(err), zap.String("ratingID", ratingID))
		return ratingErrors.ErrorCantDeleteRating
	}

	if result.RowsAffected() == 0 {
		r.logger.Debug("rating not found for deletion", zap.String("ratingID", ratingID))
		return ratingErrors.ErrorRatingNotFound
	}

	r.logger.Info("rating deleted", zap.String("ratingID", ratingID))
	return nil
}
