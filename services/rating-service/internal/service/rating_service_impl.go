package service

import (
	"context"
	"math"

	"github.com/Kpeewu/tissi-mah/services/rating-service/internal/cache"
	"github.com/Kpeewu/tissi-mah/services/rating-service/internal/client"
	"github.com/Kpeewu/tissi-mah/services/rating-service/internal/domain"
	repoInterfaces "github.com/Kpeewu/tissi-mah/services/rating-service/internal/repository/interfaces"
	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/rating-service/internal/service/interfaces"
	ratingErrors "github.com/Kpeewu/tissi-mah/services/rating-service/pkg/errors"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type ratingServiceImpl struct {
	readRepo   repoInterfaces.RatingRepositoryRead
	writeRepo  repoInterfaces.RatingRepositoryWrite
	userClient client.UserClient
	cache      *cache.RatingCache // nil si Redis indisponible
	logger     *zap.Logger
}

func NewRatingService(
	readRepo repoInterfaces.RatingRepositoryRead,
	writeRepo repoInterfaces.RatingRepositoryWrite,
	userClient client.UserClient,
	ratingCache *cache.RatingCache,
	logger *zap.Logger) serviceInterfaces.RatingService {

	return &ratingServiceImpl{
		readRepo:   readRepo,
		writeRepo:  writeRepo,
		userClient: userClient,
		cache:      ratingCache,
		logger:     logger,
	}
}

// RateUser crée une nouvelle note pour un utilisateur
func (s *ratingServiceImpl) RateUser(ctx context.Context, raterID string, userRatedID string, numberOfStars int16, comment string) (*domain.Rating, error) {
	if raterID == "" {
		s.logger.Error("rater_id is required")
		return nil, ratingErrors.ErrorMissingRaterID
	}
	if userRatedID == "" {
		s.logger.Error("user_rated_id is required")
		return nil, ratingErrors.ErrorMissingUserRatedID
	}

	s.logger.Debug("rate user",
		zap.String("raterID", raterID),
		zap.String("userRatedID", userRatedID),
		zap.Int16("stars", numberOfStars),
	)

	// Vérification : pas d'auto-notation
	if raterID == userRatedID {
		s.logger.Warn("self rating attempt", zap.String("raterID", raterID))
		return nil, ratingErrors.ErrorSelfRating
	}

	// Vérification : nombre d'étoiles valide
	if numberOfStars < domain.MinStars || numberOfStars > domain.MaxStars {
		s.logger.Warn("invalid stars", zap.Int16("stars", numberOfStars))
		return nil, ratingErrors.ErrorInvalidStars
	}

	// Vérification : le rater existe dans user-service
	if err := s.validateUserExists(ctx, raterID, "rater"); err != nil {
		return nil, err
	}

	// Vérification : le user_rated existe dans user-service
	if err := s.validateUserExists(ctx, userRatedID, "user_rated"); err != nil {
		return nil, err
	}

	// Vérification : une seule note par couple rater/rated
	exists, err := s.readRepo.ExistsByRaterAndUserRated(ctx, raterID, userRatedID)
	if err != nil {
		return nil, err
	}
	if exists {
		s.logger.Warn("rating already exists",
			zap.String("raterID", raterID),
			zap.String("userRatedID", userRatedID),
		)
		return nil, ratingErrors.ErrorRatingAlreadyExists
	}

	var commentPtr *string
	if comment != "" {
		commentPtr = &comment
	}

	rating := &domain.Rating{
		RatingID:      uuid.New().String(),
		RaterID:       raterID,
		UserRatedID:   userRatedID,
		NumberOfStars: numberOfStars,
		Comment:       commentPtr,
	}

	ratingID, err := s.writeRepo.Create(ctx, rating)
	if err != nil {
		s.logger.Error("create rating failed", zap.Error(err))
		return nil, err
	}

	s.logger.Info("rating created", zap.String("ratingID", ratingID))

	// Invalider le cache de l'utilisateur noté
	s.invalidateCache(ctx, userRatedID)

	return s.readRepo.GetByID(ctx, ratingID)
}

// GetUserRatings récupère toutes les notes reçues par un utilisateur
func (s *ratingServiceImpl) GetUserRatings(ctx context.Context, userRatedID string) ([]*domain.Rating, error) {
	if userRatedID == "" {
		return nil, ratingErrors.ErrorMissingUserRatedID
	}

	// Vérifier le cache
	if s.cache != nil {
		cached, err := s.cache.GetUserRatings(ctx, userRatedID)
		if err == nil && cached != nil {
			return cached, nil
		}
	}

	ratings, err := s.readRepo.GetByUserRatedID(ctx, userRatedID)
	if err != nil {
		return nil, err
	}

	// Mettre en cache
	if s.cache != nil {
		s.cache.SetUserRatings(ctx, userRatedID, ratings) //nolint:errcheck
	}

	return ratings, nil
}

// GetUserRatingsAverage récupère la moyenne (1 décimale) et le nombre total de notes d'un utilisateur
func (s *ratingServiceImpl) GetUserRatingsAverage(ctx context.Context, userRatedID string) (float64, int32, error) {
	if userRatedID == "" {
		return 0, 0, ratingErrors.ErrorMissingUserRatedID
	}

	// Vérifier le cache
	if s.cache != nil {
		cached, err := s.cache.GetUserAverage(ctx, userRatedID)
		if err == nil && cached != nil {
			return cached.Average, cached.TotalRatings, nil
		}
	}

	average, total, err := s.readRepo.GetAverageByUserRatedID(ctx, userRatedID)
	if err != nil {
		return 0, 0, err
	}

	// Arrondir à 1 chiffre après la virgule
	average = math.Round(average*10) / 10

	// Mettre en cache
	if s.cache != nil {
		s.cache.SetUserAverage(ctx, userRatedID, average, total) //nolint:errcheck
	}

	return average, total, nil
}

// UpdateRating modifie une note existante (seul le rater peut modifier)
func (s *ratingServiceImpl) UpdateRating(ctx context.Context, raterID string, ratingID string, userRatedID string, numberOfStars int16, comment string) (*domain.Rating, error) {
	if raterID == "" {
		s.logger.Error("rater_id is required")
		return nil, ratingErrors.ErrorMissingRaterID
	}
	if ratingID == "" {
		return nil, ratingErrors.ErrorRatingNotFound
	}

	// Vérification : nombre d'étoiles valide
	if numberOfStars < domain.MinStars || numberOfStars > domain.MaxStars {
		return nil, ratingErrors.ErrorInvalidStars
	}

	// Récupération de la note existante
	existing, err := s.readRepo.GetByID(ctx, ratingID)
	if err != nil {
		return nil, err
	}

	// Vérification : seul le rater peut modifier sa note
	if existing.RaterID != raterID {
		s.logger.Warn("unauthorized update attempt",
			zap.String("raterID", raterID),
			zap.String("ratingOwner", existing.RaterID),
		)
		return nil, ratingErrors.ErrorUnauthorizedAction
	}

	var commentPtr *string
	if comment != "" {
		commentPtr = &comment
	}

	existing.NumberOfStars = numberOfStars
	existing.Comment = commentPtr

	updated, err := s.writeRepo.Update(ctx, existing)
	if err != nil {
		return nil, err
	}

	// Invalider le cache de l'utilisateur noté
	s.invalidateCache(ctx, existing.UserRatedID)

	return updated, nil
}

// validateUserExists vérifie qu'un utilisateur existe dans user-service.
func (s *ratingServiceImpl) validateUserExists(ctx context.Context, authID string, role string) error {
	exists, err := s.userClient.UserExists(ctx, authID)
	if err != nil {
		s.logger.Error("user-service check failed",
			zap.String("authID", authID),
			zap.String("role", role),
			zap.Error(err),
		)
		return ratingErrors.ErrorInternalServer
	}
	if !exists {
		s.logger.Warn("user not found",
			zap.String("authID", authID),
			zap.String("role", role),
		)
		return ratingErrors.ErrorUserNotFound
	}
	return nil
}

// invalidateCache supprime le cache lié à un utilisateur noté.
func (s *ratingServiceImpl) invalidateCache(ctx context.Context, userRatedID string) {
	if s.cache != nil {
		s.cache.InvalidateUser(ctx, userRatedID)
	}
}
