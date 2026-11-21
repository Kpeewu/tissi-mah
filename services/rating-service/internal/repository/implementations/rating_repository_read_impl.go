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

type ratingReadRepositoryImpl struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

func NewRatingReadRepository(pool *pgxpool.Pool, logger *zap.Logger) i.RatingRepositoryRead {
	return &ratingReadRepositoryImpl{
		pool:   pool,
		logger: logger,
	}
}

// get a rating by its id
func (r *ratingReadRepositoryImpl) GetByID(ctx context.Context, ratingID string) (*domain.Rating, error) {
	r.logger.Debug("get rating by id", zap.String("ratingID", ratingID))

	query := `SELECT rating_id, rater_id, user_rated_id, number_of_stars, comment,
					 created_at, updated_at
			  FROM ratings WHERE rating_id = $1`

	rating := &domain.Rating{}

	err := r.pool.QueryRow(ctx, query, ratingID).Scan(
		&rating.RatingID,
		&rating.RaterID,
		&rating.UserRatedID,
		&rating.NumberOfStars,
		&rating.Comment,
		&rating.CreatedAt,
		&rating.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			r.logger.Debug("rating not found", zap.String("ratingID", ratingID))
			return nil, ratingErrors.ErrorRatingNotFound
		}
		r.logger.Error("get rating by id failed", zap.Error(err), zap.String("ratingID", ratingID))
		return nil, ratingErrors.ErrorDataRetrievalFailed
	}

	return rating, nil
}

// get all ratings received by a user
func (r *ratingReadRepositoryImpl) GetByUserRatedID(ctx context.Context, userRatedID string) ([]*domain.Rating, error) {
	r.logger.Debug("get ratings for user", zap.String("userRatedID", userRatedID))

	query := `SELECT rating_id, rater_id, user_rated_id, number_of_stars, comment,
					 created_at, updated_at
			  FROM ratings WHERE user_rated_id = $1
			  ORDER BY created_at DESC`

	rows, err := r.pool.Query(ctx, query, userRatedID)
	if err != nil {
		r.logger.Error("get ratings for user failed", zap.Error(err), zap.String("userRatedID", userRatedID))
		return nil, ratingErrors.ErrorDataRetrievalFailed
	}
	defer rows.Close()

	var ratings []*domain.Rating
	for rows.Next() {
		rating := &domain.Rating{}
		err := rows.Scan(
			&rating.RatingID,
			&rating.RaterID,
			&rating.UserRatedID,
			&rating.NumberOfStars,
			&rating.Comment,
			&rating.CreatedAt,
			&rating.UpdatedAt,
		)
		if err != nil {
			r.logger.Error("scan rating row failed", zap.Error(err))
			return nil, ratingErrors.ErrorDataRetrievalFailed
		}
		ratings = append(ratings, rating)
	}

	if err := rows.Err(); err != nil {
		r.logger.Error("rows iteration failed", zap.Error(err))
		return nil, ratingErrors.ErrorDataRetrievalFailed
	}

	return ratings, nil
}

// get a specific rating given by rater to rated user
func (r *ratingReadRepositoryImpl) GetByRaterAndUserRated(ctx context.Context, raterID string, userRatedID string) (*domain.Rating, error) {
	r.logger.Debug("get rating by rater and rated",
		zap.String("raterID", raterID),
		zap.String("userRatedID", userRatedID),
	)

	query := `SELECT rating_id, rater_id, user_rated_id, number_of_stars, comment,
					 created_at, updated_at
			  FROM ratings WHERE rater_id = $1 AND user_rated_id = $2`

	rating := &domain.Rating{}

	err := r.pool.QueryRow(ctx, query, raterID, userRatedID).Scan(
		&rating.RatingID,
		&rating.RaterID,
		&rating.UserRatedID,
		&rating.NumberOfStars,
		&rating.Comment,
		&rating.CreatedAt,
		&rating.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			r.logger.Debug("rating not found for pair",
				zap.String("raterID", raterID),
				zap.String("userRatedID", userRatedID),
			)
			return nil, ratingErrors.ErrorRatingNotFound
		}
		r.logger.Error("get rating by pair failed", zap.Error(err))
		return nil, ratingErrors.ErrorDataRetrievalFailed
	}

	return rating, nil
}

// check if a rating already exists for this rater/rated pair
func (r *ratingReadRepositoryImpl) ExistsByRaterAndUserRated(ctx context.Context, raterID string, userRatedID string) (bool, error) {
	r.logger.Debug("checking rating exists",
		zap.String("raterID", raterID),
		zap.String("userRatedID", userRatedID),
	)

	query := `SELECT EXISTS(SELECT 1 FROM ratings WHERE rater_id = $1 AND user_rated_id = $2)`

	var exists bool
	err := r.pool.QueryRow(ctx, query, raterID, userRatedID).Scan(&exists)

	if err != nil {
		r.logger.Error("rating exists check failed", zap.Error(err))
		return false, ratingErrors.ErrorDataRetrievalFailed
	}

	return exists, nil
}

// get the average rating and total count for a user
func (r *ratingReadRepositoryImpl) GetAverageByUserRatedID(ctx context.Context, userRatedID string) (float64, int32, error) {
	r.logger.Debug("get average rating for user", zap.String("userRatedID", userRatedID))

	query := `SELECT COALESCE(AVG(number_of_stars), 0), COUNT(*)
			  FROM ratings WHERE user_rated_id = $1`

	var average float64
	var total int32

	err := r.pool.QueryRow(ctx, query, userRatedID).Scan(&average, &total)

	if err != nil {
		r.logger.Error("get average rating failed", zap.Error(err), zap.String("userRatedID", userRatedID))
		return 0, 0, ratingErrors.ErrorDataRetrievalFailed
	}

	return average, total, nil
}
