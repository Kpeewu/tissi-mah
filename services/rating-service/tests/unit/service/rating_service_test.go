package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Kpeewu/tissi-mah/services/rating-service/fixtures"
	"github.com/Kpeewu/tissi-mah/services/rating-service/internal/client"
	"github.com/Kpeewu/tissi-mah/services/rating-service/internal/domain"
	"github.com/Kpeewu/tissi-mah/services/rating-service/internal/service"
	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/rating-service/internal/service/interfaces"
	ratingErrors "github.com/Kpeewu/tissi-mah/services/rating-service/pkg/errors"
	"github.com/Kpeewu/tissi-mah/services/rating-service/tests/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// --- Helpers ---

func newTestService() (*mocks.MockRatingRepositoryRead, *mocks.MockRatingRepositoryWrite, *mocks.MockUserClient, serviceInterfaces.RatingService) {
	mockReadRepo := new(mocks.MockRatingRepositoryRead)
	mockWriteRepo := new(mocks.MockRatingRepositoryWrite)
	mockUserClient := new(mocks.MockUserClient)
	logger := zap.NewNop()
	svc := service.NewRatingService(mockReadRepo, mockWriteRepo, mockUserClient, nil, nil, logger)
	return mockReadRepo, mockWriteRepo, mockUserClient, svc
}

// =============================================================================
// RateUser
// =============================================================================

func TestRateUser(t *testing.T) {
	t.Run("succès - crée une note", func(t *testing.T) {
		mockReadRepo, mockWriteRepo, mockUserClient, svc := newTestService()
		ctx := context.Background()

		raterID := "rater-123"
		userRatedID := "rated-456"

		// Les deux utilisateurs existent
		mockUserClient.On("UserExists", mock.Anything, raterID).Return(true, nil)
		mockUserClient.On("UserExists", mock.Anything, userRatedID).Return(true, nil)

		// Pas de note existante pour ce couple
		mockReadRepo.On("ExistsByRaterAndUserRated", mock.Anything, raterID, userRatedID).Return(false, nil)

		// Création réussie
		mockWriteRepo.On("Create", mock.Anything, mock.MatchedBy(func(r *domain.Rating) bool {
			return r.RaterID == raterID && r.UserRatedID == userRatedID && r.NumberOfStars == 4
		})).Return("rating-new-id", nil)

		// Récupération après création
		created := fixtures.NewTestRating(
			fixtures.WithRatingID("rating-new-id"),
			fixtures.WithRaterID(raterID),
			fixtures.WithUserRatedID(userRatedID),
			fixtures.WithStars(4),
			fixtures.WithComment("Bon trajet"),
		)
		mockReadRepo.On("GetByID", mock.Anything, "rating-new-id").Return(created, nil)

		result, err := svc.RateUser(ctx, raterID, userRatedID, 4, "Bon trajet")

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "rating-new-id", result.RatingID)
		assert.Equal(t, raterID, result.RaterID)
		assert.Equal(t, userRatedID, result.UserRatedID)
		assert.Equal(t, int16(4), result.NumberOfStars)
		mockReadRepo.AssertExpectations(t)
		mockWriteRepo.AssertExpectations(t)
		mockUserClient.AssertExpectations(t)
	})

	t.Run("erreur - rater_id vide", func(t *testing.T) {
		_, _, _, svc := newTestService()
		ctx := context.Background()

		result, err := svc.RateUser(ctx, "", "rated-456", 4, "")

		assert.Nil(t, result)
		assert.ErrorIs(t, err, ratingErrors.ErrorMissingRaterID)
	})

	t.Run("erreur - user_rated_id vide", func(t *testing.T) {
		_, _, _, svc := newTestService()
		ctx := context.Background()

		result, err := svc.RateUser(ctx, "rater-123", "", 4, "")

		assert.Nil(t, result)
		assert.ErrorIs(t, err, ratingErrors.ErrorMissingUserRatedID)
	})

	t.Run("erreur - auto-notation", func(t *testing.T) {
		_, _, _, svc := newTestService()
		ctx := context.Background()

		result, err := svc.RateUser(ctx, "same-user", "same-user", 5, "")

		assert.Nil(t, result)
		assert.ErrorIs(t, err, ratingErrors.ErrorSelfRating)
	})

	t.Run("erreur - étoiles trop basses (0)", func(t *testing.T) {
		_, _, _, svc := newTestService()
		ctx := context.Background()

		result, err := svc.RateUser(ctx, "rater-123", "rated-456", 0, "")

		assert.Nil(t, result)
		assert.ErrorIs(t, err, ratingErrors.ErrorInvalidStars)
	})

	t.Run("erreur - étoiles trop hautes (6)", func(t *testing.T) {
		_, _, _, svc := newTestService()
		ctx := context.Background()

		result, err := svc.RateUser(ctx, "rater-123", "rated-456", 6, "")

		assert.Nil(t, result)
		assert.ErrorIs(t, err, ratingErrors.ErrorInvalidStars)
	})

	t.Run("erreur - rater n'existe pas dans user-service", func(t *testing.T) {
		_, _, mockUserClient, svc := newTestService()
		ctx := context.Background()

		mockUserClient.On("UserExists", mock.Anything, "rater-unknown").Return(false, nil)

		result, err := svc.RateUser(ctx, "rater-unknown", "rated-456", 4, "")

		assert.Nil(t, result)
		assert.ErrorIs(t, err, ratingErrors.ErrorUserNotFound)
		mockUserClient.AssertExpectations(t)
	})

	t.Run("erreur - user_rated n'existe pas dans user-service", func(t *testing.T) {
		_, _, mockUserClient, svc := newTestService()
		ctx := context.Background()

		mockUserClient.On("UserExists", mock.Anything, "rater-123").Return(true, nil)
		mockUserClient.On("UserExists", mock.Anything, "rated-unknown").Return(false, nil)

		result, err := svc.RateUser(ctx, "rater-123", "rated-unknown", 4, "")

		assert.Nil(t, result)
		assert.ErrorIs(t, err, ratingErrors.ErrorUserNotFound)
		mockUserClient.AssertExpectations(t)
	})

	t.Run("erreur - user-service indisponible", func(t *testing.T) {
		_, _, mockUserClient, svc := newTestService()
		ctx := context.Background()

		mockUserClient.On("UserExists", mock.Anything, "rater-123").Return(false, errors.New("connection refused"))

		result, err := svc.RateUser(ctx, "rater-123", "rated-456", 4, "")

		assert.Nil(t, result)
		assert.ErrorIs(t, err, ratingErrors.ErrorInternalServer)
		mockUserClient.AssertExpectations(t)
	})

	t.Run("erreur - note déjà existante pour ce couple", func(t *testing.T) {
		mockReadRepo, _, mockUserClient, svc := newTestService()
		ctx := context.Background()

		mockUserClient.On("UserExists", mock.Anything, "rater-123").Return(true, nil)
		mockUserClient.On("UserExists", mock.Anything, "rated-456").Return(true, nil)
		mockReadRepo.On("ExistsByRaterAndUserRated", mock.Anything, "rater-123", "rated-456").Return(true, nil)

		result, err := svc.RateUser(ctx, "rater-123", "rated-456", 4, "")

		assert.Nil(t, result)
		assert.ErrorIs(t, err, ratingErrors.ErrorRatingAlreadyExists)
		mockReadRepo.AssertExpectations(t)
	})

	t.Run("erreur - échec de la création en base", func(t *testing.T) {
		mockReadRepo, mockWriteRepo, mockUserClient, svc := newTestService()
		ctx := context.Background()

		mockUserClient.On("UserExists", mock.Anything, "rater-123").Return(true, nil)
		mockUserClient.On("UserExists", mock.Anything, "rated-456").Return(true, nil)
		mockReadRepo.On("ExistsByRaterAndUserRated", mock.Anything, "rater-123", "rated-456").Return(false, nil)
		mockWriteRepo.On("Create", mock.Anything, mock.Anything).Return("", ratingErrors.ErrorInternalServer)

		result, err := svc.RateUser(ctx, "rater-123", "rated-456", 4, "")

		assert.Nil(t, result)
		assert.ErrorIs(t, err, ratingErrors.ErrorInternalServer)
		mockWriteRepo.AssertExpectations(t)
	})

	t.Run("succès - commentaire vide passe nil au domaine", func(t *testing.T) {
		mockReadRepo, mockWriteRepo, mockUserClient, svc := newTestService()
		ctx := context.Background()

		mockUserClient.On("UserExists", mock.Anything, "rater-123").Return(true, nil)
		mockUserClient.On("UserExists", mock.Anything, "rated-456").Return(true, nil)
		mockReadRepo.On("ExistsByRaterAndUserRated", mock.Anything, "rater-123", "rated-456").Return(false, nil)

		// Vérifier que le commentaire nil est passé quand le comment est vide
		mockWriteRepo.On("Create", mock.Anything, mock.MatchedBy(func(r *domain.Rating) bool {
			return r.Comment == nil
		})).Return("rating-id", nil)

		created := fixtures.NewTestRating(fixtures.WithRatingID("rating-id"), fixtures.WithNoComment())
		mockReadRepo.On("GetByID", mock.Anything, "rating-id").Return(created, nil)

		result, err := svc.RateUser(ctx, "rater-123", "rated-456", 4, "")

		require.NoError(t, err)
		require.NotNil(t, result)
		mockWriteRepo.AssertExpectations(t)
	})
}

// =============================================================================
// GetUserRatings
// =============================================================================

func TestGetUserRatings(t *testing.T) {
	t.Run("succès - retourne les notes enrichies avec l'identité du noteur", func(t *testing.T) {
		mockReadRepo, _, mockUserClient, svc := newTestService()
		ctx := context.Background()

		rater1ID := "ed9f0de2-fedd-4b0b-a5cc-af6d1a7fb30e"
		rater2ID := "5a959758-72d4-467a-a899-58dcb01eda3b"
		ratings := []*domain.Rating{
			fixtures.NewTestRating(fixtures.WithRaterID(rater1ID), fixtures.WithUserRatedID("user-123"), fixtures.WithStars(5)),
			fixtures.NewTestRating(fixtures.WithRaterID(rater2ID), fixtures.WithUserRatedID("user-123"), fixtures.WithStars(3)),
		}
		mockReadRepo.On("GetByUserRatedID", mock.Anything, "user-123").Return(ratings, nil)

		profiles := map[string]*client.UserProfile{
			rater1ID: {FirstName: "Kofi", LastName: "Mensah", ProfileImageURL: "https://cdn.example.com/kofi.jpg"},
			rater2ID: {FirstName: "Ama", LastName: "Asante", ProfileImageURL: ""},
		}
		mockUserClient.On("GetUsersByUserIDs", mock.Anything, mock.Anything).Return(profiles, nil)

		result, err := svc.GetUserRatings(ctx, "user-123")

		require.NoError(t, err)
		assert.Len(t, result, 2)
		assert.Equal(t, int16(5), result[0].NumberOfStars)
		assert.Equal(t, "Kofi", result[0].RaterFirstName)
		assert.Equal(t, "Mensah", result[0].RaterLastName)
		assert.Equal(t, "https://cdn.example.com/kofi.jpg", result[0].RaterProfileImageURL)
		assert.Equal(t, int16(3), result[1].NumberOfStars)
		assert.Equal(t, "Ama", result[1].RaterFirstName)
		mockReadRepo.AssertExpectations(t)
		mockUserClient.AssertExpectations(t)
	})

	t.Run("dégradation gracieuse - user-service indisponible", func(t *testing.T) {
		mockReadRepo, _, mockUserClient, svc := newTestService()
		ctx := context.Background()

		ratings := []*domain.Rating{
			fixtures.NewTestRating(fixtures.WithUserRatedID("user-123"), fixtures.WithStars(4)),
		}
		mockReadRepo.On("GetByUserRatedID", mock.Anything, "user-123").Return(ratings, nil)
		mockUserClient.On("GetUsersByUserIDs", mock.Anything, mock.Anything).Return(nil, errors.New("user-service down"))

		result, err := svc.GetUserRatings(ctx, "user-123")

		require.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Empty(t, result[0].RaterFirstName)
		assert.Empty(t, result[0].RaterProfileImageURL)
	})

	t.Run("succès - retourne une liste vide si aucune note", func(t *testing.T) {
		mockReadRepo, _, _, svc := newTestService()
		ctx := context.Background()

		mockReadRepo.On("GetByUserRatedID", mock.Anything, "user-no-ratings").Return([]*domain.Rating{}, nil)

		result, err := svc.GetUserRatings(ctx, "user-no-ratings")

		require.NoError(t, err)
		assert.Empty(t, result)
		mockReadRepo.AssertExpectations(t)
	})

	t.Run("erreur - user_rated_id vide", func(t *testing.T) {
		_, _, _, svc := newTestService()
		ctx := context.Background()

		result, err := svc.GetUserRatings(ctx, "")

		assert.Nil(t, result)
		assert.ErrorIs(t, err, ratingErrors.ErrorMissingUserRatedID)
	})

	t.Run("erreur - échec de la récupération en base", func(t *testing.T) {
		mockReadRepo, _, _, svc := newTestService()
		ctx := context.Background()

		mockReadRepo.On("GetByUserRatedID", mock.Anything, "user-123").Return(nil, ratingErrors.ErrorDataRetrievalFailed)

		result, err := svc.GetUserRatings(ctx, "user-123")

		assert.Nil(t, result)
		assert.ErrorIs(t, err, ratingErrors.ErrorDataRetrievalFailed)
		mockReadRepo.AssertExpectations(t)
	})
}

// =============================================================================
// GetUserRatingsAverage
// =============================================================================

func TestGetUserRatingsAverage(t *testing.T) {
	t.Run("succès - retourne la moyenne arrondie à 1 décimale", func(t *testing.T) {
		mockReadRepo, _, _, svc := newTestService()
		ctx := context.Background()

		// 4.333... doit être arrondi à 4.3
		mockReadRepo.On("GetAverageByUserRatedID", mock.Anything, "user-123").Return(4.333333, int32(3), nil)

		average, total, err := svc.GetUserRatingsAverage(ctx, "user-123")

		require.NoError(t, err)
		assert.Equal(t, 4.3, average)
		assert.Equal(t, int32(3), total)
		mockReadRepo.AssertExpectations(t)
	})

	t.Run("succès - arrondi vers le haut (4.75 → 4.8)", func(t *testing.T) {
		mockReadRepo, _, _, svc := newTestService()
		ctx := context.Background()

		mockReadRepo.On("GetAverageByUserRatedID", mock.Anything, "user-123").Return(4.75, int32(4), nil)

		average, _, err := svc.GetUserRatingsAverage(ctx, "user-123")

		require.NoError(t, err)
		assert.Equal(t, 4.8, average)
		mockReadRepo.AssertExpectations(t)
	})

	t.Run("succès - aucune note retourne 0.0", func(t *testing.T) {
		mockReadRepo, _, _, svc := newTestService()
		ctx := context.Background()

		mockReadRepo.On("GetAverageByUserRatedID", mock.Anything, "user-123").Return(0.0, int32(0), nil)

		average, total, err := svc.GetUserRatingsAverage(ctx, "user-123")

		require.NoError(t, err)
		assert.Equal(t, 0.0, average)
		assert.Equal(t, int32(0), total)
		mockReadRepo.AssertExpectations(t)
	})

	t.Run("erreur - user_rated_id vide", func(t *testing.T) {
		_, _, _, svc := newTestService()
		ctx := context.Background()

		average, total, err := svc.GetUserRatingsAverage(ctx, "")

		assert.Equal(t, 0.0, average)
		assert.Equal(t, int32(0), total)
		assert.ErrorIs(t, err, ratingErrors.ErrorMissingUserRatedID)
	})

	t.Run("erreur - échec de la récupération en base", func(t *testing.T) {
		mockReadRepo, _, _, svc := newTestService()
		ctx := context.Background()

		mockReadRepo.On("GetAverageByUserRatedID", mock.Anything, "user-123").Return(0.0, int32(0), ratingErrors.ErrorDataRetrievalFailed)

		_, _, err := svc.GetUserRatingsAverage(ctx, "user-123")

		assert.ErrorIs(t, err, ratingErrors.ErrorDataRetrievalFailed)
		mockReadRepo.AssertExpectations(t)
	})
}

// =============================================================================
// UpdateRating
// =============================================================================

func TestUpdateRating(t *testing.T) {
	t.Run("succès - met à jour une note existante", func(t *testing.T) {
		mockReadRepo, mockWriteRepo, _, svc := newTestService()
		ctx := context.Background()

		existing := fixtures.NewTestRating(
			fixtures.WithRatingID("rating-001"),
			fixtures.WithRaterID("rater-123"),
			fixtures.WithUserRatedID("rated-456"),
			fixtures.WithStars(3),
		)
		mockReadRepo.On("GetByID", mock.Anything, "rating-001").Return(existing, nil)

		updated := fixtures.NewTestRating(
			fixtures.WithRatingID("rating-001"),
			fixtures.WithRaterID("rater-123"),
			fixtures.WithUserRatedID("rated-456"),
			fixtures.WithStars(5),
			fixtures.WithComment("Mise à jour"),
		)
		mockWriteRepo.On("Update", mock.Anything, mock.MatchedBy(func(r *domain.Rating) bool {
			return r.RatingID == "rating-001" && r.NumberOfStars == 5
		})).Return(updated, nil)

		result, err := svc.UpdateRating(ctx, "rater-123", "rating-001", "rated-456", 5, "Mise à jour")

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, int16(5), result.NumberOfStars)
		mockReadRepo.AssertExpectations(t)
		mockWriteRepo.AssertExpectations(t)
	})

	t.Run("erreur - rater_id vide", func(t *testing.T) {
		_, _, _, svc := newTestService()
		ctx := context.Background()

		result, err := svc.UpdateRating(ctx, "", "rating-001", "rated-456", 5, "")

		assert.Nil(t, result)
		assert.ErrorIs(t, err, ratingErrors.ErrorMissingRaterID)
	})

	t.Run("erreur - rating_id vide", func(t *testing.T) {
		_, _, _, svc := newTestService()
		ctx := context.Background()

		result, err := svc.UpdateRating(ctx, "rater-123", "", "rated-456", 5, "")

		assert.Nil(t, result)
		assert.ErrorIs(t, err, ratingErrors.ErrorRatingNotFound)
	})

	t.Run("erreur - étoiles invalides", func(t *testing.T) {
		_, _, _, svc := newTestService()
		ctx := context.Background()

		result, err := svc.UpdateRating(ctx, "rater-123", "rating-001", "rated-456", 0, "")

		assert.Nil(t, result)
		assert.ErrorIs(t, err, ratingErrors.ErrorInvalidStars)
	})

	t.Run("erreur - note non trouvée", func(t *testing.T) {
		mockReadRepo, _, _, svc := newTestService()
		ctx := context.Background()

		mockReadRepo.On("GetByID", mock.Anything, "rating-unknown").Return(nil, ratingErrors.ErrorRatingNotFound)

		result, err := svc.UpdateRating(ctx, "rater-123", "rating-unknown", "rated-456", 5, "")

		assert.Nil(t, result)
		assert.ErrorIs(t, err, ratingErrors.ErrorRatingNotFound)
		mockReadRepo.AssertExpectations(t)
	})

	t.Run("erreur - rater_id ne correspond pas au propriétaire", func(t *testing.T) {
		mockReadRepo, _, _, svc := newTestService()
		ctx := context.Background()

		existing := fixtures.NewTestRating(
			fixtures.WithRatingID("rating-001"),
			fixtures.WithRaterID("original-rater"),
		)
		mockReadRepo.On("GetByID", mock.Anything, "rating-001").Return(existing, nil)

		result, err := svc.UpdateRating(ctx, "other-rater", "rating-001", "rated-456", 5, "")

		assert.Nil(t, result)
		assert.ErrorIs(t, err, ratingErrors.ErrorUnauthorizedAction)
		mockReadRepo.AssertExpectations(t)
	})

	t.Run("erreur - échec de la mise à jour en base", func(t *testing.T) {
		mockReadRepo, mockWriteRepo, _, svc := newTestService()
		ctx := context.Background()

		existing := fixtures.NewTestRating(
			fixtures.WithRatingID("rating-001"),
			fixtures.WithRaterID("rater-123"),
		)
		mockReadRepo.On("GetByID", mock.Anything, "rating-001").Return(existing, nil)
		mockWriteRepo.On("Update", mock.Anything, mock.Anything).Return(nil, ratingErrors.ErrorInternalServer)

		result, err := svc.UpdateRating(ctx, "rater-123", "rating-001", "rated-456", 5, "")

		assert.Nil(t, result)
		assert.ErrorIs(t, err, ratingErrors.ErrorInternalServer)
		mockWriteRepo.AssertExpectations(t)
	})

	t.Run("succès - commentaire vide passe nil", func(t *testing.T) {
		mockReadRepo, mockWriteRepo, _, svc := newTestService()
		ctx := context.Background()

		existing := fixtures.NewTestRating(
			fixtures.WithRatingID("rating-001"),
			fixtures.WithRaterID("rater-123"),
		)
		mockReadRepo.On("GetByID", mock.Anything, "rating-001").Return(existing, nil)

		updated := fixtures.NewTestRating(
			fixtures.WithRatingID("rating-001"),
			fixtures.WithRaterID("rater-123"),
			fixtures.WithStars(5),
			fixtures.WithNoComment(),
		)
		mockWriteRepo.On("Update", mock.Anything, mock.MatchedBy(func(r *domain.Rating) bool {
			return r.Comment == nil
		})).Return(updated, nil)

		result, err := svc.UpdateRating(ctx, "rater-123", "rating-001", "rated-456", 5, "")

		require.NoError(t, err)
		assert.Nil(t, result.Comment)
		mockWriteRepo.AssertExpectations(t)
	})
}
