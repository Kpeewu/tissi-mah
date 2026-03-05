package service

import (
	"context"

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
	logger     *zap.Logger
}

func NewRatingService(
	readRepo repoInterfaces.RatingRepositoryRead,
	writeRepo repoInterfaces.RatingRepositoryWrite,
	userClient client.UserClient,
	logger *zap.Logger) serviceInterfaces.RatingService {

	return &ratingServiceImpl{
		readRepo:   readRepo,
		writeRepo:  writeRepo,
		userClient: userClient,
		logger:     logger,
	}
}

// CreateRating crée une nouvelle note pour un utilisateur
func (s *ratingServiceImpl) CreateRating(ctx context.Context, raterID string, userRatedID string, numberOfStars int16, comment string) (*domain.Rating, error) {
	if raterID == "" {
		s.logger.Error("rater_id is required")
		return nil, ratingErrors.ErrorInvalidStars
	}

	s.logger.Debug("create rating",
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

	return s.readRepo.GetByID(ctx, ratingID)
}

// GetRating récupère une note par son ID
func (s *ratingServiceImpl) GetRating(ctx context.Context, ratingID string) (*domain.Rating, error) {
	if ratingID == "" {
		return nil, ratingErrors.ErrorRatingNotFound
	}

	return s.readRepo.GetByID(ctx, ratingID)
}

// GetRatingsForUser récupère toutes les notes reçues par un utilisateur
func (s *ratingServiceImpl) GetRatingsForUser(ctx context.Context, userRatedID string) ([]*domain.Rating, error) {
	if userRatedID == "" {
		return nil, ratingErrors.ErrorDataRetrievalFailed
	}

	return s.readRepo.GetByUserRatedID(ctx, userRatedID)
}

// GetAverageRating récupère la moyenne et le nombre total de notes d'un utilisateur
func (s *ratingServiceImpl) GetAverageRating(ctx context.Context, userRatedID string) (float64, int32, error) {
	if userRatedID == "" {
		return 0, 0, ratingErrors.ErrorDataRetrievalFailed
	}

	return s.readRepo.GetAverageByUserRatedID(ctx, userRatedID)
}

// UpdateRating modifie une note existante (seul le rater peut modifier)
func (s *ratingServiceImpl) UpdateRating(ctx context.Context, raterID string, ratingID string, numberOfStars int16, comment string) (*domain.Rating, error) {
	if raterID == "" {
		s.logger.Error("rater_id is required")
		return nil, ratingErrors.ErrorInvalidStars
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

	return s.writeRepo.Update(ctx, existing)
}

// DeleteRating supprime une note (seul le rater peut supprimer)
func (s *ratingServiceImpl) DeleteRating(ctx context.Context, raterID string, ratingID string) error {
	if raterID == "" {
		s.logger.Error("rater_id is required")
		return ratingErrors.ErrorInvalidStars
	}

	// Récupération de la note existante
	existing, err := s.readRepo.GetByID(ctx, ratingID)
	if err != nil {
		return err
	}

	// Vérification : seul le rater peut supprimer sa note
	if existing.RaterID != raterID {
		s.logger.Warn("unauthorized delete attempt",
			zap.String("raterID", raterID),
			zap.String("ratingOwner", existing.RaterID),
		)
		return ratingErrors.ErrorUnauthorizedAction
	}

	return s.writeRepo.Delete(ctx, ratingID)
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
