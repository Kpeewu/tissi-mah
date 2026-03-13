package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/Kpeewu/tissi-mah/services/rating-service/fixtures"
	ratingpb "github.com/Kpeewu/tissi-mah/services/rating-service/proto/gen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// =============================================================================
// RateUser E2E
// =============================================================================

func TestE2E_RateUser(t *testing.T) {
	ctx := context.Background()

	t.Run("succès - créer une note via gRPC", func(t *testing.T) {
		cleanupRatingsTable(t, ctx)

		resp, err := grpcClient.RateUser(ctx, &ratingpb.RateUserRequest{
			RaterId:       "e2e-rater-001",
			UserRatedId:   "e2e-rated-001",
			NumberOfStars: 5,
			Comment:       "Trajet parfait",
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		require.NotNil(t, resp.Rating)
		assert.NotEmpty(t, resp.Rating.RatingId)
		assert.Equal(t, "e2e-rater-001", resp.Rating.RaterId)
		assert.Equal(t, "e2e-rated-001", resp.Rating.UserRatedId)
		assert.Equal(t, int32(5), resp.Rating.NumberOfStars)
		assert.Equal(t, "Trajet parfait", resp.Rating.Comment)

		// Vérifier que les timestamps sont en ISO 8601
		_, err = time.Parse(time.RFC3339, resp.Rating.CreatedAt)
		assert.NoError(t, err, "CreatedAt doit être en ISO 8601")
		_, err = time.Parse(time.RFC3339, resp.Rating.UpdatedAt)
		assert.NoError(t, err, "UpdatedAt doit être en ISO 8601")
	})

	t.Run("erreur - auto-notation", func(t *testing.T) {
		cleanupRatingsTable(t, ctx)

		_, err := grpcClient.RateUser(ctx, &ratingpb.RateUserRequest{
			RaterId:       "same-user",
			UserRatedId:   "same-user",
			NumberOfStars: 5,
		})

		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
		assert.Contains(t, st.Message(), "ErrorSelfRating")
	})

	t.Run("erreur - étoiles invalides (0)", func(t *testing.T) {
		cleanupRatingsTable(t, ctx)

		_, err := grpcClient.RateUser(ctx, &ratingpb.RateUserRequest{
			RaterId:       "e2e-rater",
			UserRatedId:   "e2e-rated",
			NumberOfStars: 0,
		})

		require.Error(t, err)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("erreur - étoiles invalides (6)", func(t *testing.T) {
		cleanupRatingsTable(t, ctx)

		_, err := grpcClient.RateUser(ctx, &ratingpb.RateUserRequest{
			RaterId:       "e2e-rater",
			UserRatedId:   "e2e-rated",
			NumberOfStars: 6,
		})

		require.Error(t, err)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("erreur - rater_id vide", func(t *testing.T) {
		cleanupRatingsTable(t, ctx)

		_, err := grpcClient.RateUser(ctx, &ratingpb.RateUserRequest{
			UserRatedId:   "e2e-rated",
			NumberOfStars: 4,
		})

		require.Error(t, err)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.InvalidArgument, st.Code())
		assert.Contains(t, st.Message(), "ErrorMissingRaterID")
	})

	t.Run("erreur - user_rated_id vide", func(t *testing.T) {
		cleanupRatingsTable(t, ctx)

		_, err := grpcClient.RateUser(ctx, &ratingpb.RateUserRequest{
			RaterId:       "e2e-rater",
			NumberOfStars: 4,
		})

		require.Error(t, err)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.InvalidArgument, st.Code())
		assert.Contains(t, st.Message(), "ErrorMissingUserRatedID")
	})

	t.Run("erreur - note déjà existante pour ce couple", func(t *testing.T) {
		cleanupRatingsTable(t, ctx)

		// Première note : OK
		_, err := grpcClient.RateUser(ctx, &ratingpb.RateUserRequest{
			RaterId:       "e2e-rater-dup",
			UserRatedId:   "e2e-rated-dup",
			NumberOfStars: 4,
		})
		require.NoError(t, err)

		// Deuxième note : erreur
		_, err = grpcClient.RateUser(ctx, &ratingpb.RateUserRequest{
			RaterId:       "e2e-rater-dup",
			UserRatedId:   "e2e-rated-dup",
			NumberOfStars: 5,
		})

		require.Error(t, err)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.AlreadyExists, st.Code())
	})
}

// =============================================================================
// GetUserRatings E2E
// =============================================================================

func TestE2E_GetUserRatings(t *testing.T) {
	ctx := context.Background()

	t.Run("succès - récupère toutes les notes d'un utilisateur", func(t *testing.T) {
		cleanupRatingsTable(t, ctx)

		// Créer 2 notes
		_, err := grpcClient.RateUser(ctx, &ratingpb.RateUserRequest{
			RaterId: "rater-a", UserRatedId: "user-target", NumberOfStars: 5, Comment: "Super",
		})
		require.NoError(t, err)
		_, err = grpcClient.RateUser(ctx, &ratingpb.RateUserRequest{
			RaterId: "rater-b", UserRatedId: "user-target", NumberOfStars: 3,
		})
		require.NoError(t, err)

		// Récupérer
		resp, err := grpcClient.GetUserRatings(ctx, &ratingpb.GetUserRatingsRequest{UserRatedId: "user-target"})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Len(t, resp.Ratings, 2)
	})

	t.Run("succès - liste vide quand aucune note", func(t *testing.T) {
		cleanupRatingsTable(t, ctx)

		resp, err := grpcClient.GetUserRatings(ctx, &ratingpb.GetUserRatingsRequest{UserRatedId: "user-no-ratings"})

		require.NoError(t, err)
		assert.Empty(t, resp.Ratings)
	})

	t.Run("erreur - user_rated_id vide", func(t *testing.T) {
		_, err := grpcClient.GetUserRatings(ctx, &ratingpb.GetUserRatingsRequest{UserRatedId: ""})

		require.Error(t, err)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})
}

// =============================================================================
// GetUserRatingsAverage E2E
// =============================================================================

func TestE2E_GetUserRatingsAverage(t *testing.T) {
	ctx := context.Background()

	t.Run("succès - calcule la moyenne correctement", func(t *testing.T) {
		cleanupRatingsTable(t, ctx)

		// 5 + 3 + 4 = 12 / 3 = 4.0
		userRatedID := "user-avg-e2e"
		_, err := grpcClient.RateUser(ctx, &ratingpb.RateUserRequest{RaterId: "r-a", UserRatedId: userRatedID, NumberOfStars: 5})
		require.NoError(t, err)
		_, err = grpcClient.RateUser(ctx, &ratingpb.RateUserRequest{RaterId: "r-b", UserRatedId: userRatedID, NumberOfStars: 3})
		require.NoError(t, err)
		_, err = grpcClient.RateUser(ctx, &ratingpb.RateUserRequest{RaterId: "r-c", UserRatedId: userRatedID, NumberOfStars: 4})
		require.NoError(t, err)

		resp, err := grpcClient.GetUserRatingsAverage(ctx, &ratingpb.GetUserRatingsAverageRequest{UserRatedId: userRatedID})

		require.NoError(t, err)
		assert.Equal(t, 4.0, resp.Average)
		assert.Equal(t, int32(3), resp.TotalRatings)
	})

	t.Run("succès - arrondi à 1 décimale (4.333 → 4.3)", func(t *testing.T) {
		cleanupRatingsTable(t, ctx)

		// 5 + 4 + 4 = 13 / 3 = 4.333...
		userRatedID := "user-avg-round"
		_, err := grpcClient.RateUser(ctx, &ratingpb.RateUserRequest{RaterId: "r-1", UserRatedId: userRatedID, NumberOfStars: 5})
		require.NoError(t, err)
		_, err = grpcClient.RateUser(ctx, &ratingpb.RateUserRequest{RaterId: "r-2", UserRatedId: userRatedID, NumberOfStars: 4})
		require.NoError(t, err)
		_, err = grpcClient.RateUser(ctx, &ratingpb.RateUserRequest{RaterId: "r-3", UserRatedId: userRatedID, NumberOfStars: 4})
		require.NoError(t, err)

		resp, err := grpcClient.GetUserRatingsAverage(ctx, &ratingpb.GetUserRatingsAverageRequest{UserRatedId: userRatedID})

		require.NoError(t, err)
		assert.Equal(t, 4.3, resp.Average)
	})

	t.Run("succès - 0 quand aucune note", func(t *testing.T) {
		cleanupRatingsTable(t, ctx)

		resp, err := grpcClient.GetUserRatingsAverage(ctx, &ratingpb.GetUserRatingsAverageRequest{UserRatedId: "user-no-avg"})

		require.NoError(t, err)
		assert.Equal(t, 0.0, resp.Average)
		assert.Equal(t, int32(0), resp.TotalRatings)
	})
}

// =============================================================================
// UpdateRating E2E
// =============================================================================

func TestE2E_UpdateRating(t *testing.T) {
	ctx := context.Background()

	t.Run("succès - modifier sa propre note", func(t *testing.T) {
		cleanupRatingsTable(t, ctx)

		// Créer une note
		createResp, err := grpcClient.RateUser(ctx, &ratingpb.RateUserRequest{
			RaterId: "e2e-rater-upd", UserRatedId: "e2e-rated-upd", NumberOfStars: 3, Comment: "Initial",
		})
		require.NoError(t, err)
		ratingID := createResp.Rating.RatingId

		// Modifier
		updateResp, err := grpcClient.UpdateRating(ctx, &ratingpb.UpdateRatingRequest{
			RatingId:      ratingID,
			RaterId:       "e2e-rater-upd",
			UserRatedId:   "e2e-rated-upd",
			NumberOfStars: 5,
			Comment:       "Bien mieux !",
		})

		require.NoError(t, err)
		require.NotNil(t, updateResp.Rating)
		assert.Equal(t, ratingID, updateResp.Rating.RatingId)
		assert.Equal(t, int32(5), updateResp.Rating.NumberOfStars)
		assert.Equal(t, "Bien mieux !", updateResp.Rating.Comment)
	})

	t.Run("erreur - modifier la note d'un autre utilisateur", func(t *testing.T) {
		cleanupRatingsTable(t, ctx)

		// Créer une note
		createResp, err := grpcClient.RateUser(ctx, &ratingpb.RateUserRequest{
			RaterId: "original-rater", UserRatedId: "rated-user", NumberOfStars: 4,
		})
		require.NoError(t, err)
		ratingID := createResp.Rating.RatingId

		// Tentative de modification par un autre rater
		_, err = grpcClient.UpdateRating(ctx, &ratingpb.UpdateRatingRequest{
			RatingId:      ratingID,
			RaterId:       "other-rater",
			UserRatedId:   "rated-user",
			NumberOfStars: 1,
		})

		require.Error(t, err)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.PermissionDenied, st.Code())
	})

	t.Run("erreur - note inexistante", func(t *testing.T) {
		cleanupRatingsTable(t, ctx)

		_, err := grpcClient.UpdateRating(ctx, &ratingpb.UpdateRatingRequest{
			RatingId:      "nonexistent-id",
			RaterId:       "rater-x",
			UserRatedId:   "rated-x",
			NumberOfStars: 5,
		})

		require.Error(t, err)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.NotFound, st.Code())
	})

	t.Run("la moyenne est mise à jour après modification", func(t *testing.T) {
		cleanupRatingsTable(t, ctx)

		userRatedID := "rated-avg-update"

		// Créer 2 notes : 4 + 4 = 8 / 2 = 4.0
		resp1, err := grpcClient.RateUser(ctx, &ratingpb.RateUserRequest{
			RaterId: "rater-1", UserRatedId: userRatedID, NumberOfStars: 4,
		})
		require.NoError(t, err)
		resp2, err := grpcClient.RateUser(ctx, &ratingpb.RateUserRequest{
			RaterId: "rater-2", UserRatedId: userRatedID, NumberOfStars: 4,
		})
		require.NoError(t, err)
		_ = resp2

		avgResp, err := grpcClient.GetUserRatingsAverage(ctx, &ratingpb.GetUserRatingsAverageRequest{UserRatedId: userRatedID})
		require.NoError(t, err)
		assert.Equal(t, 4.0, avgResp.Average)

		// Modifier la première note : 2 + 4 = 6 / 2 = 3.0
		_, err = grpcClient.UpdateRating(ctx, &ratingpb.UpdateRatingRequest{
			RatingId:      resp1.Rating.RatingId,
			RaterId:       "rater-1",
			UserRatedId:   userRatedID,
			NumberOfStars: 2,
		})
		require.NoError(t, err)

		avgResp, err = grpcClient.GetUserRatingsAverage(ctx, &ratingpb.GetUserRatingsAverageRequest{UserRatedId: userRatedID})
		require.NoError(t, err)
		assert.Equal(t, 3.0, avgResp.Average)
	})
}

// =============================================================================
// Health E2E
// =============================================================================

func TestE2E_Health(t *testing.T) {
	ctx := context.Background()

	resp, err := grpcClient.Health(ctx, &ratingpb.HealthRequest{})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "SERVING", resp.Status)
	assert.NotEmpty(t, resp.Version)
	assert.Greater(t, resp.Timestamp, int64(0))
}

// =============================================================================
// Scénario complet E2E
// =============================================================================

func TestE2E_FullScenario(t *testing.T) {
	ctx := context.Background()
	cleanupRatingsTable(t, ctx)

	userRatedID := "driver-full-scenario"

	// 1. Health check
	health, err := grpcClient.Health(ctx, &ratingpb.HealthRequest{})
	require.NoError(t, err)
	assert.Equal(t, "SERVING", health.Status)

	// 2. Vérifier 0 notes au départ
	avgResp, err := grpcClient.GetUserRatingsAverage(ctx, &ratingpb.GetUserRatingsAverageRequest{UserRatedId: userRatedID})
	require.NoError(t, err)
	assert.Equal(t, 0.0, avgResp.Average)
	assert.Equal(t, int32(0), avgResp.TotalRatings)

	// 3. Passager A note le conducteur : 5 étoiles
	rateResp1, err := grpcClient.RateUser(ctx, &ratingpb.RateUserRequest{
		RaterId: "passenger-a", UserRatedId: userRatedID, NumberOfStars: 5, Comment: "Parfait",
	})
	require.NoError(t, err)
	assert.Equal(t, int32(5), rateResp1.Rating.NumberOfStars)

	// 4. Passager B note le conducteur : 3 étoiles
	_, err = grpcClient.RateUser(ctx, &ratingpb.RateUserRequest{
		RaterId: "passenger-b", UserRatedId: userRatedID, NumberOfStars: 3,
	})
	require.NoError(t, err)

	// 5. Vérifier la moyenne : (5+3)/2 = 4.0
	avgResp, err = grpcClient.GetUserRatingsAverage(ctx, &ratingpb.GetUserRatingsAverageRequest{UserRatedId: userRatedID})
	require.NoError(t, err)
	assert.Equal(t, 4.0, avgResp.Average)
	assert.Equal(t, int32(2), avgResp.TotalRatings)

	// 6. Récupérer les notes
	listResp, err := grpcClient.GetUserRatings(ctx, &ratingpb.GetUserRatingsRequest{UserRatedId: userRatedID})
	require.NoError(t, err)
	assert.Len(t, listResp.Ratings, 2)

	// 7. Passager A met à jour sa note : 5 → 1
	_, err = grpcClient.UpdateRating(ctx, &ratingpb.UpdateRatingRequest{
		RatingId:      rateResp1.Rating.RatingId,
		RaterId:       "passenger-a",
		UserRatedId:   userRatedID,
		NumberOfStars: 1,
		Comment:       "Finalement non",
	})
	require.NoError(t, err)

	// 8. Vérifier la nouvelle moyenne : (1+3)/2 = 2.0
	avgResp, err = grpcClient.GetUserRatingsAverage(ctx, &ratingpb.GetUserRatingsAverageRequest{UserRatedId: userRatedID})
	require.NoError(t, err)
	assert.Equal(t, 2.0, avgResp.Average)

	// 9. Passager B ne peut pas modifier la note du passager A
	_, err = grpcClient.UpdateRating(ctx, &ratingpb.UpdateRatingRequest{
		RatingId:      rateResp1.Rating.RatingId,
		RaterId:       "passenger-b",
		UserRatedId:   userRatedID,
		NumberOfStars: 5,
	})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.PermissionDenied, st.Code())

	// 10. Passager A ne peut pas re-noter le même conducteur
	_, err = grpcClient.RateUser(ctx, &ratingpb.RateUserRequest{
		RaterId: "passenger-a", UserRatedId: userRatedID, NumberOfStars: 4,
	})
	require.Error(t, err)
	st, _ = status.FromError(err)
	assert.Equal(t, codes.AlreadyExists, st.Code())

	// 11. Vérifier que les timestamps sont ISO 8601
	for _, r := range listResp.Ratings {
		_, parseErr := time.Parse(time.RFC3339, r.CreatedAt)
		assert.NoError(t, parseErr, "CreatedAt doit être ISO 8601")
	}

	// 12. Insérer un rating pour un autre user — ne doit pas affecter driver-full-scenario
	_, err = grpcClient.RateUser(ctx, &ratingpb.RateUserRequest{
		RaterId: "passenger-c", UserRatedId: "other-driver", NumberOfStars: 1,
	})
	require.NoError(t, err)

	avgResp, err = grpcClient.GetUserRatingsAverage(ctx, &ratingpb.GetUserRatingsAverageRequest{UserRatedId: userRatedID})
	require.NoError(t, err)
	assert.Equal(t, 2.0, avgResp.Average)
	assert.Equal(t, int32(2), avgResp.TotalRatings)
}

// =============================================================================
// Scénario DB constraints E2E
// =============================================================================

func TestE2E_DBConstraints(t *testing.T) {
	ctx := context.Background()

	t.Run("contrainte unique rater/rated empêche le doublon", func(t *testing.T) {
		cleanupRatingsTable(t, ctx)

		// Insérer directement en base pour bypass la validation service
		rating := fixtures.NewTestRating(
			fixtures.WithRaterID("constraint-rater"),
			fixtures.WithUserRatedID("constraint-rated"),
		)
		require.NoError(t, fixtures.InsertRating(ctx, testPool, rating))

		// Tenter via gRPC
		_, err := grpcClient.RateUser(ctx, &ratingpb.RateUserRequest{
			RaterId: "constraint-rater", UserRatedId: "constraint-rated", NumberOfStars: 3,
		})

		require.Error(t, err)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.AlreadyExists, st.Code())
	})
}
