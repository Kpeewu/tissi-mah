package handler_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/Kpeewu/tissi-mah/services/rating-service/internal/domain"
	grpcHandler "github.com/Kpeewu/tissi-mah/services/rating-service/internal/grpc"
	ratingErrors "github.com/Kpeewu/tissi-mah/services/rating-service/pkg/errors"
	ratingpb "github.com/Kpeewu/tissi-mah/services/rating-service/proto/gen"
	"github.com/Kpeewu/tissi-mah/services/rating-service/tests/mocks"
)

// --------------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------------

func newHandler() (*grpcHandler.RatingHandler, *mocks.MockRatingService) {
	mockService := new(mocks.MockRatingService)
	logger := zap.NewNop()
	handler := grpcHandler.NewRatingHandler(mockService, logger)
	return handler, mockService
}

func ptrString(s string) *string { return &s }

func assertGRPCCode(t *testing.T, err error, expected codes.Code) {
	t.Helper()
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok, "l'erreur doit être un statut gRPC")
	assert.Equal(t, expected, st.Code())
}

// --------------------------------------------------------------------------
// RateUser
// --------------------------------------------------------------------------

func TestRateUser_Success(t *testing.T) {
	handler, mockService := newHandler()
	ctx := context.Background()

	now := time.Now().UTC()
	comment := "Excellent"
	rating := &domain.Rating{
		RatingID:      "rating-001",
		RaterID:       "rater-123",
		UserRatedID:   "rated-456",
		NumberOfStars: 5,
		Comment:       &comment,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	mockService.On("RateUser", mock.Anything, "rater-123", "rated-456", int16(5), "Excellent").
		Return(rating, nil)

	resp, err := handler.RateUser(ctx, &ratingpb.RateUserRequest{
		RaterId:       "rater-123",
		UserRatedId:   "rated-456",
		NumberOfStars: 5,
		Comment:       "Excellent",
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, resp.Rating)
	assert.Equal(t, "rating-001", resp.Rating.RatingId)
	assert.Equal(t, "rater-123", resp.Rating.RaterId)
	assert.Equal(t, "rated-456", resp.Rating.UserRatedId)
	assert.Equal(t, int32(5), resp.Rating.NumberOfStars)
	assert.Equal(t, "Excellent", resp.Rating.Comment)
	assert.NotEmpty(t, resp.Rating.CreatedAt)
	assert.NotEmpty(t, resp.Rating.UpdatedAt)
	mockService.AssertExpectations(t)
}

func TestRateUser_MissingRaterID(t *testing.T) {
	handler, mockService := newHandler()
	ctx := context.Background()

	mockService.On("RateUser", mock.Anything, "", "rated-456", int16(4), "").
		Return(nil, ratingErrors.ErrorMissingRaterID)

	_, err := handler.RateUser(ctx, &ratingpb.RateUserRequest{
		UserRatedId:   "rated-456",
		NumberOfStars: 4,
	})

	assertGRPCCode(t, err, codes.InvalidArgument)
	mockService.AssertExpectations(t)
}

func TestRateUser_SelfRating(t *testing.T) {
	handler, mockService := newHandler()
	ctx := context.Background()

	mockService.On("RateUser", mock.Anything, "same-user", "same-user", int16(4), "").
		Return(nil, ratingErrors.ErrorSelfRating)

	_, err := handler.RateUser(ctx, &ratingpb.RateUserRequest{
		RaterId:       "same-user",
		UserRatedId:   "same-user",
		NumberOfStars: 4,
	})

	assertGRPCCode(t, err, codes.InvalidArgument)
	mockService.AssertExpectations(t)
}

func TestRateUser_AlreadyExists(t *testing.T) {
	handler, mockService := newHandler()
	ctx := context.Background()

	mockService.On("RateUser", mock.Anything, "rater-123", "rated-456", int16(4), "").
		Return(nil, ratingErrors.ErrorRatingAlreadyExists)

	_, err := handler.RateUser(ctx, &ratingpb.RateUserRequest{
		RaterId:       "rater-123",
		UserRatedId:   "rated-456",
		NumberOfStars: 4,
	})

	assertGRPCCode(t, err, codes.AlreadyExists)
	mockService.AssertExpectations(t)
}

func TestRateUser_NilComment(t *testing.T) {
	handler, mockService := newHandler()
	ctx := context.Background()

	now := time.Now().UTC()
	rating := &domain.Rating{
		RatingID:      "rating-001",
		RaterID:       "rater-123",
		UserRatedID:   "rated-456",
		NumberOfStars: 4,
		Comment:       nil,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	mockService.On("RateUser", mock.Anything, "rater-123", "rated-456", int16(4), "").
		Return(rating, nil)

	resp, err := handler.RateUser(ctx, &ratingpb.RateUserRequest{
		RaterId:       "rater-123",
		UserRatedId:   "rated-456",
		NumberOfStars: 4,
	})

	require.NoError(t, err)
	assert.Equal(t, "", resp.Rating.Comment)
	mockService.AssertExpectations(t)
}

// --------------------------------------------------------------------------
// GetUserRatings
// --------------------------------------------------------------------------

func TestGetUserRatings_Success(t *testing.T) {
	handler, mockService := newHandler()
	ctx := context.Background()

	now := time.Now().UTC()
	comment := "Super"
	ratings := []*domain.Rating{
		{RatingID: "r-1", RaterID: "rater-a", UserRatedID: "user-123", NumberOfStars: 5, Comment: &comment, CreatedAt: now, UpdatedAt: now},
		{RatingID: "r-2", RaterID: "rater-b", UserRatedID: "user-123", NumberOfStars: 3, Comment: nil, CreatedAt: now, UpdatedAt: now},
	}

	mockService.On("GetUserRatings", mock.Anything, "user-123").Return(ratings, nil)

	resp, err := handler.GetUserRatings(ctx, &ratingpb.GetUserRatingsRequest{UserRatedId: "user-123"})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Len(t, resp.Ratings, 2)
	assert.Equal(t, "r-1", resp.Ratings[0].RatingId)
	assert.Equal(t, int32(5), resp.Ratings[0].NumberOfStars)
	assert.Equal(t, "Super", resp.Ratings[0].Comment)
	assert.Equal(t, "r-2", resp.Ratings[1].RatingId)
	assert.Equal(t, "", resp.Ratings[1].Comment)
	mockService.AssertExpectations(t)
}

func TestGetUserRatings_Empty(t *testing.T) {
	handler, mockService := newHandler()
	ctx := context.Background()

	mockService.On("GetUserRatings", mock.Anything, "user-no-ratings").Return([]*domain.Rating{}, nil)

	resp, err := handler.GetUserRatings(ctx, &ratingpb.GetUserRatingsRequest{UserRatedId: "user-no-ratings"})

	require.NoError(t, err)
	assert.Empty(t, resp.Ratings)
	mockService.AssertExpectations(t)
}

func TestGetUserRatings_MissingUserRatedID(t *testing.T) {
	handler, mockService := newHandler()
	ctx := context.Background()

	mockService.On("GetUserRatings", mock.Anything, "").Return(nil, ratingErrors.ErrorMissingUserRatedID)

	_, err := handler.GetUserRatings(ctx, &ratingpb.GetUserRatingsRequest{UserRatedId: ""})

	assertGRPCCode(t, err, codes.InvalidArgument)
	mockService.AssertExpectations(t)
}

// --------------------------------------------------------------------------
// GetUserRatingsAverage
// --------------------------------------------------------------------------

func TestGetUserRatingsAverage_Success(t *testing.T) {
	handler, mockService := newHandler()
	ctx := context.Background()

	mockService.On("GetUserRatingsAverage", mock.Anything, "user-123").Return(4.3, int32(10), nil)

	resp, err := handler.GetUserRatingsAverage(ctx, &ratingpb.GetUserRatingsAverageRequest{UserRatedId: "user-123"})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, 4.3, resp.Average)
	assert.Equal(t, int32(10), resp.TotalRatings)
	mockService.AssertExpectations(t)
}

func TestGetUserRatingsAverage_NoRatings(t *testing.T) {
	handler, mockService := newHandler()
	ctx := context.Background()

	mockService.On("GetUserRatingsAverage", mock.Anything, "user-empty").Return(0.0, int32(0), nil)

	resp, err := handler.GetUserRatingsAverage(ctx, &ratingpb.GetUserRatingsAverageRequest{UserRatedId: "user-empty"})

	require.NoError(t, err)
	assert.Equal(t, 0.0, resp.Average)
	assert.Equal(t, int32(0), resp.TotalRatings)
	mockService.AssertExpectations(t)
}

func TestGetUserRatingsAverage_MissingUserRatedID(t *testing.T) {
	handler, mockService := newHandler()
	ctx := context.Background()

	mockService.On("GetUserRatingsAverage", mock.Anything, "").Return(0.0, int32(0), ratingErrors.ErrorMissingUserRatedID)

	_, err := handler.GetUserRatingsAverage(ctx, &ratingpb.GetUserRatingsAverageRequest{UserRatedId: ""})

	assertGRPCCode(t, err, codes.InvalidArgument)
	mockService.AssertExpectations(t)
}

// --------------------------------------------------------------------------
// UpdateRating
// --------------------------------------------------------------------------

func TestUpdateRating_Success(t *testing.T) {
	handler, mockService := newHandler()
	ctx := context.Background()

	now := time.Now().UTC()
	comment := "Mise à jour"
	updated := &domain.Rating{
		RatingID:      "rating-001",
		RaterID:       "rater-123",
		UserRatedID:   "rated-456",
		NumberOfStars: 5,
		Comment:       &comment,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	mockService.On("UpdateRating", mock.Anything, "rater-123", "rating-001", "rated-456", int16(5), "Mise à jour").
		Return(updated, nil)

	resp, err := handler.UpdateRating(ctx, &ratingpb.UpdateRatingRequest{
		RatingId:      "rating-001",
		RaterId:       "rater-123",
		UserRatedId:   "rated-456",
		NumberOfStars: 5,
		Comment:       "Mise à jour",
	})

	require.NoError(t, err)
	require.NotNil(t, resp.Rating)
	assert.Equal(t, "rating-001", resp.Rating.RatingId)
	assert.Equal(t, int32(5), resp.Rating.NumberOfStars)
	assert.Equal(t, "Mise à jour", resp.Rating.Comment)
	mockService.AssertExpectations(t)
}

func TestUpdateRating_NotFound(t *testing.T) {
	handler, mockService := newHandler()
	ctx := context.Background()

	mockService.On("UpdateRating", mock.Anything, "rater-123", "rating-unknown", "rated-456", int16(5), "").
		Return(nil, ratingErrors.ErrorRatingNotFound)

	_, err := handler.UpdateRating(ctx, &ratingpb.UpdateRatingRequest{
		RatingId:      "rating-unknown",
		RaterId:       "rater-123",
		UserRatedId:   "rated-456",
		NumberOfStars: 5,
	})

	assertGRPCCode(t, err, codes.NotFound)
	mockService.AssertExpectations(t)
}

func TestUpdateRating_Unauthorized(t *testing.T) {
	handler, mockService := newHandler()
	ctx := context.Background()

	mockService.On("UpdateRating", mock.Anything, "other-rater", "rating-001", "rated-456", int16(5), "").
		Return(nil, ratingErrors.ErrorUnauthorizedAction)

	_, err := handler.UpdateRating(ctx, &ratingpb.UpdateRatingRequest{
		RatingId:      "rating-001",
		RaterId:       "other-rater",
		UserRatedId:   "rated-456",
		NumberOfStars: 5,
	})

	assertGRPCCode(t, err, codes.PermissionDenied)
	mockService.AssertExpectations(t)
}

func TestUpdateRating_InvalidStars(t *testing.T) {
	handler, mockService := newHandler()
	ctx := context.Background()

	mockService.On("UpdateRating", mock.Anything, "rater-123", "rating-001", "rated-456", int16(0), "").
		Return(nil, ratingErrors.ErrorInvalidStars)

	_, err := handler.UpdateRating(ctx, &ratingpb.UpdateRatingRequest{
		RatingId:      "rating-001",
		RaterId:       "rater-123",
		UserRatedId:   "rated-456",
		NumberOfStars: 0,
	})

	assertGRPCCode(t, err, codes.InvalidArgument)
	mockService.AssertExpectations(t)
}

// --------------------------------------------------------------------------
// Health
// --------------------------------------------------------------------------

func TestHealth_Success(t *testing.T) {
	handler, _ := newHandler()
	ctx := context.Background()

	resp, err := handler.Health(ctx, &ratingpb.HealthRequest{})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "SERVING", resp.Status)
	assert.NotEmpty(t, resp.Version)
	assert.Greater(t, resp.Timestamp, int64(0))
}

// --------------------------------------------------------------------------
// toGRPCError mapping
// --------------------------------------------------------------------------

func TestToGRPCError_Mapping(t *testing.T) {
	handler, mockService := newHandler()
	ctx := context.Background()

	tests := []struct {
		name     string
		err      error
		expected codes.Code
	}{
		{"ErrorRatingNotFound → NotFound", ratingErrors.ErrorRatingNotFound, codes.NotFound},
		{"ErrorUserNotFound → NotFound", ratingErrors.ErrorUserNotFound, codes.NotFound},
		{"ErrorRatingAlreadyExists → AlreadyExists", ratingErrors.ErrorRatingAlreadyExists, codes.AlreadyExists},
		{"ErrorInvalidStars → InvalidArgument", ratingErrors.ErrorInvalidStars, codes.InvalidArgument},
		{"ErrorSelfRating → InvalidArgument", ratingErrors.ErrorSelfRating, codes.InvalidArgument},
		{"ErrorMissingRaterID → InvalidArgument", ratingErrors.ErrorMissingRaterID, codes.InvalidArgument},
		{"ErrorMissingUserRatedID → InvalidArgument", ratingErrors.ErrorMissingUserRatedID, codes.InvalidArgument},
		{"ErrorUnauthorizedAction → PermissionDenied", ratingErrors.ErrorUnauthorizedAction, codes.PermissionDenied},
		{"ErrorDataRetrievalFailed → Internal", ratingErrors.ErrorDataRetrievalFailed, codes.Internal},
		{"ErrorInternalServer → Internal", ratingErrors.ErrorInternalServer, codes.Internal},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockSvc := new(mocks.MockRatingService)
			mockSvc.On("GetUserRatings", mock.Anything, "test").Return(nil, tc.err)

			// Réutiliser le handler avec un nouveau mock pour isoler
			h := grpcHandler.NewRatingHandler(mockSvc, zap.NewNop())

			_, err := h.GetUserRatings(ctx, &ratingpb.GetUserRatingsRequest{UserRatedId: "test"})

			assertGRPCCode(t, err, tc.expected)
			mockSvc.AssertExpectations(t)
		})
	}

	// Erreur inconnue → Internal avec message générique
	t.Run("UnknownError → Internal", func(t *testing.T) {
		mockService.On("GetUserRatings", mock.Anything, "test-unknown").
			Return(nil, assert.AnError)

		_, err := handler.GetUserRatings(ctx, &ratingpb.GetUserRatingsRequest{UserRatedId: "test-unknown"})

		assertGRPCCode(t, err, codes.Internal)
		st, _ := status.FromError(err)
		assert.Equal(t, "internal server error", st.Message())
		mockService.AssertExpectations(t)
	})
}
