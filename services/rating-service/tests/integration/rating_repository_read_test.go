package integration

import (
	"context"
	"testing"

	"github.com/Kpeewu/tissi-mah/services/rating-service/fixtures"
	ratingErrors "github.com/Kpeewu/tissi-mah/services/rating-service/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// GetByID
// =============================================================================

func TestRatingRepositoryRead_GetByID(t *testing.T) {
	ctx := context.Background()
	repo := newTestRatingReadRepository()

	t.Run("should return rating when id exists", func(t *testing.T) {
		cleanupRatingsTable(t, ctx)

		testRating := fixtures.NewTestRating()
		err := fixtures.InsertRating(ctx, testPool, testRating)
		require.NoError(t, err, "Failed to insert test rating")

		result, err := repo.GetByID(ctx, testRating.RatingID)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, testRating.RatingID, result.RatingID)
		assert.Equal(t, testRating.RaterID, result.RaterID)
		assert.Equal(t, testRating.UserRatedID, result.UserRatedID)
		assert.Equal(t, testRating.NumberOfStars, result.NumberOfStars)
	})

	t.Run("should return ErrorRatingNotFound when id does not exist", func(t *testing.T) {
		cleanupRatingsTable(t, ctx)

		result, err := repo.GetByID(ctx, "nonexistent_rating_id")

		assert.Error(t, err)
		assert.ErrorIs(t, err, ratingErrors.ErrorRatingNotFound)
		assert.Nil(t, result)
	})
}

// =============================================================================
// GetByUserRatedID
// =============================================================================

func TestRatingRepositoryRead_GetByUserRatedID(t *testing.T) {
	ctx := context.Background()
	repo := newTestRatingReadRepository()

	t.Run("should return all ratings for a user", func(t *testing.T) {
		cleanupRatingsTable(t, ctx)

		userRatedID := "user-rated-001"
		r1 := fixtures.NewTestRating(fixtures.WithUserRatedID(userRatedID), fixtures.WithRaterID("rater-a"), fixtures.WithStars(5))
		r2 := fixtures.NewTestRating(fixtures.WithUserRatedID(userRatedID), fixtures.WithRaterID("rater-b"), fixtures.WithStars(3))

		require.NoError(t, fixtures.InsertRating(ctx, testPool, r1))
		require.NoError(t, fixtures.InsertRating(ctx, testPool, r2))

		result, err := repo.GetByUserRatedID(ctx, userRatedID)

		assert.NoError(t, err)
		assert.Len(t, result, 2)
	})

	t.Run("should return empty list when no ratings exist", func(t *testing.T) {
		cleanupRatingsTable(t, ctx)

		result, err := repo.GetByUserRatedID(ctx, "user-with-no-ratings")

		assert.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("should not return ratings for other users", func(t *testing.T) {
		cleanupRatingsTable(t, ctx)

		r1 := fixtures.NewTestRating(fixtures.WithUserRatedID("user-a"), fixtures.WithRaterID("rater-1"))
		r2 := fixtures.NewTestRating(fixtures.WithUserRatedID("user-b"), fixtures.WithRaterID("rater-2"))

		require.NoError(t, fixtures.InsertRating(ctx, testPool, r1))
		require.NoError(t, fixtures.InsertRating(ctx, testPool, r2))

		result, err := repo.GetByUserRatedID(ctx, "user-a")

		assert.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Equal(t, "user-a", result[0].UserRatedID)
	})
}

// =============================================================================
// GetByRaterAndUserRated
// =============================================================================

func TestRatingRepositoryRead_GetByRaterAndUserRated(t *testing.T) {
	ctx := context.Background()
	repo := newTestRatingReadRepository()

	t.Run("should return rating for rater/rated pair", func(t *testing.T) {
		cleanupRatingsTable(t, ctx)

		testRating := fixtures.NewTestRating(
			fixtures.WithRaterID("rater-123"),
			fixtures.WithUserRatedID("rated-456"),
		)
		require.NoError(t, fixtures.InsertRating(ctx, testPool, testRating))

		result, err := repo.GetByRaterAndUserRated(ctx, "rater-123", "rated-456")

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "rater-123", result.RaterID)
		assert.Equal(t, "rated-456", result.UserRatedID)
	})

	t.Run("should return ErrorRatingNotFound when pair does not exist", func(t *testing.T) {
		cleanupRatingsTable(t, ctx)

		result, err := repo.GetByRaterAndUserRated(ctx, "rater-unknown", "rated-unknown")

		assert.Error(t, err)
		assert.ErrorIs(t, err, ratingErrors.ErrorRatingNotFound)
		assert.Nil(t, result)
	})
}

// =============================================================================
// ExistsByRaterAndUserRated
// =============================================================================

func TestRatingRepositoryRead_ExistsByRaterAndUserRated(t *testing.T) {
	ctx := context.Background()
	repo := newTestRatingReadRepository()

	t.Run("should return true when rating exists", func(t *testing.T) {
		cleanupRatingsTable(t, ctx)

		testRating := fixtures.NewTestRating(
			fixtures.WithRaterID("rater-123"),
			fixtures.WithUserRatedID("rated-456"),
		)
		require.NoError(t, fixtures.InsertRating(ctx, testPool, testRating))

		exists, err := repo.ExistsByRaterAndUserRated(ctx, "rater-123", "rated-456")

		assert.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("should return false when rating does not exist", func(t *testing.T) {
		cleanupRatingsTable(t, ctx)

		exists, err := repo.ExistsByRaterAndUserRated(ctx, "rater-unknown", "rated-unknown")

		assert.NoError(t, err)
		assert.False(t, exists)
	})
}

// =============================================================================
// GetAverageByUserRatedID
// =============================================================================

func TestRatingRepositoryRead_GetAverageByUserRatedID(t *testing.T) {
	ctx := context.Background()
	repo := newTestRatingReadRepository()

	t.Run("should return correct average and total", func(t *testing.T) {
		cleanupRatingsTable(t, ctx)

		userRatedID := "user-avg-test"
		r1 := fixtures.NewTestRating(fixtures.WithUserRatedID(userRatedID), fixtures.WithRaterID("rater-a"), fixtures.WithStars(5))
		r2 := fixtures.NewTestRating(fixtures.WithUserRatedID(userRatedID), fixtures.WithRaterID("rater-b"), fixtures.WithStars(3))
		r3 := fixtures.NewTestRating(fixtures.WithUserRatedID(userRatedID), fixtures.WithRaterID("rater-c"), fixtures.WithStars(4))

		require.NoError(t, fixtures.InsertRating(ctx, testPool, r1))
		require.NoError(t, fixtures.InsertRating(ctx, testPool, r2))
		require.NoError(t, fixtures.InsertRating(ctx, testPool, r3))

		average, total, err := repo.GetAverageByUserRatedID(ctx, userRatedID)

		assert.NoError(t, err)
		assert.Equal(t, int32(3), total)
		assert.InDelta(t, 4.0, average, 0.01)
	})

	t.Run("should return 0 when no ratings", func(t *testing.T) {
		cleanupRatingsTable(t, ctx)

		average, total, err := repo.GetAverageByUserRatedID(ctx, "user-no-ratings")

		assert.NoError(t, err)
		assert.Equal(t, 0.0, average)
		assert.Equal(t, int32(0), total)
	})
}
