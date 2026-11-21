package mocks

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/rating-service/internal/domain"
	"github.com/stretchr/testify/mock"
)

type MockRatingService struct {
	mock.Mock
}

func (m *MockRatingService) RateUser(ctx context.Context, raterID string, userRatedID string, numberOfStars int16, comment string) (*domain.Rating, error) {
	args := m.Called(ctx, raterID, userRatedID, numberOfStars, comment)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Rating), args.Error(1)
}

func (m *MockRatingService) GetUserRatings(ctx context.Context, userRatedID string) ([]*domain.Rating, error) {
	args := m.Called(ctx, userRatedID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Rating), args.Error(1)
}

func (m *MockRatingService) GetUserRatingsAverage(ctx context.Context, userRatedID string) (float64, int32, error) {
	args := m.Called(ctx, userRatedID)
	return args.Get(0).(float64), args.Get(1).(int32), args.Error(2)
}

func (m *MockRatingService) UpdateRating(ctx context.Context, raterID string, ratingID string, userRatedID string, numberOfStars int16, comment string) (*domain.Rating, error) {
	args := m.Called(ctx, raterID, ratingID, userRatedID, numberOfStars, comment)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Rating), args.Error(1)
}
