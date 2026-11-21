package integration

import (
	"context"
	"testing"

	"github.com/Kpeewu/tissi-mah/services/rating-service/fixtures"
	"github.com/Kpeewu/tissi-mah/services/rating-service/internal/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Create
// =============================================================================

func TestRatingRepositoryWrite_Create(t *testing.T) {
	ctx := context.Background()
	writeRepo := newTestRatingWriteRepository()
	readRepo := newTestRatingReadRepository()

	t.Run("should create a rating and return its id", func(t *testing.T) {
		cleanupRatingsTable(t, ctx)

		rating := &domain.Rating{
			RatingID:      uuid.New().String(),
			RaterID:       uuid.New().String(),
			UserRatedID:   uuid.New().String(),
			NumberOfStars: 4,
			Comment:       ptrString("Bon trajet"),
		}

		ratingID, err := writeRepo.Create(ctx, rating)

		require.NoError(t, err)
		assert.NotEmpty(t, ratingID)
		assert.Equal(t, rating.RatingID, ratingID)

		// Vérifier en base
		result, err := readRepo.GetByID(ctx, ratingID)
		require.NoError(t, err)
		assert.Equal(t, rating.RaterID, result.RaterID)
		assert.Equal(t, rating.UserRatedID, result.UserRatedID)
		assert.Equal(t, rating.NumberOfStars, result.NumberOfStars)
		assert.Equal(t, "Bon trajet", result.GetComment())
		assert.False(t, result.CreatedAt.IsZero())
		assert.False(t, result.UpdatedAt.IsZero())
	})

	t.Run("should create a rating without comment", func(t *testing.T) {
		cleanupRatingsTable(t, ctx)

		rating := &domain.Rating{
			RatingID:      uuid.New().String(),
			RaterID:       uuid.New().String(),
			UserRatedID:   uuid.New().String(),
			NumberOfStars: 5,
			Comment:       nil,
		}

		ratingID, err := writeRepo.Create(ctx, rating)

		require.NoError(t, err)

		result, err := readRepo.GetByID(ctx, ratingID)
		require.NoError(t, err)
		assert.Nil(t, result.Comment)
	})

	t.Run("should fail on duplicate rater/rated pair", func(t *testing.T) {
		cleanupRatingsTable(t, ctx)

		rating1 := fixtures.NewTestRating(
			fixtures.WithRaterID("rater-dup"),
			fixtures.WithUserRatedID("rated-dup"),
		)
		require.NoError(t, fixtures.InsertRating(ctx, testPool, rating1))

		rating2 := &domain.Rating{
			RatingID:      uuid.New().String(),
			RaterID:       "rater-dup",
			UserRatedID:   "rated-dup",
			NumberOfStars: 3,
		}

		_, err := writeRepo.Create(ctx, rating2)

		assert.Error(t, err)
	})

	t.Run("should fail on self-rating (DB constraint)", func(t *testing.T) {
		cleanupRatingsTable(t, ctx)

		rating := &domain.Rating{
			RatingID:      uuid.New().String(),
			RaterID:       "same-user",
			UserRatedID:   "same-user",
			NumberOfStars: 5,
		}

		_, err := writeRepo.Create(ctx, rating)

		assert.Error(t, err)
	})

	t.Run("should fail on invalid stars (DB constraint)", func(t *testing.T) {
		cleanupRatingsTable(t, ctx)

		rating := &domain.Rating{
			RatingID:      uuid.New().String(),
			RaterID:       uuid.New().String(),
			UserRatedID:   uuid.New().String(),
			NumberOfStars: 6,
		}

		_, err := writeRepo.Create(ctx, rating)

		assert.Error(t, err)
	})
}

// =============================================================================
// Update
// =============================================================================

func TestRatingRepositoryWrite_Update(t *testing.T) {
	ctx := context.Background()
	writeRepo := newTestRatingWriteRepository()

	t.Run("should update stars and comment", func(t *testing.T) {
		cleanupRatingsTable(t, ctx)

		original := fixtures.NewTestRating(
			fixtures.WithStars(3),
			fixtures.WithComment("Initial"),
		)
		require.NoError(t, fixtures.InsertRating(ctx, testPool, original))

		original.NumberOfStars = 5
		newComment := "Mise à jour"
		original.Comment = &newComment

		updated, err := writeRepo.Update(ctx, original)

		require.NoError(t, err)
		require.NotNil(t, updated)
		assert.Equal(t, int16(5), updated.NumberOfStars)
		assert.Equal(t, "Mise à jour", updated.GetComment())
		assert.True(t, updated.UpdatedAt.After(original.CreatedAt) || updated.UpdatedAt.Equal(original.CreatedAt))
	})

	t.Run("should update to nil comment", func(t *testing.T) {
		cleanupRatingsTable(t, ctx)

		original := fixtures.NewTestRating(fixtures.WithComment("To remove"))
		require.NoError(t, fixtures.InsertRating(ctx, testPool, original))

		original.Comment = nil

		updated, err := writeRepo.Update(ctx, original)

		require.NoError(t, err)
		assert.Nil(t, updated.Comment)
	})

	t.Run("should fail on nonexistent rating", func(t *testing.T) {
		cleanupRatingsTable(t, ctx)

		nonexistent := fixtures.NewTestRating(fixtures.WithRatingID("nonexistent-id"))

		_, err := writeRepo.Update(ctx, nonexistent)

		assert.Error(t, err)
	})
}

// --- helpers ---

func ptrString(s string) *string { return &s }
