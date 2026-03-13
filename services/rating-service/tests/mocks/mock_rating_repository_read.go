package mocks

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/rating-service/internal/domain"
	"github.com/stretchr/testify/mock"
)

type MockRatingRepositoryRead struct {
	mock.Mock
}

func (m *MockRatingRepositoryRead) GetByID(ctx context.Context, ratingID string) (*domain.Rating, error) {
	args := m.Called(ctx, ratingID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Rating), args.Error(1)
}

func (m *MockRatingRepositoryRead) GetByUserRatedID(ctx context.Context, userRatedID string) ([]*domain.Rating, error) {
	args := m.Called(ctx, userRatedID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Rating), args.Error(1)
}

func (m *MockRatingRepositoryRead) GetByRaterAndUserRated(ctx context.Context, raterID string, userRatedID string) (*domain.Rating, error) {
	args := m.Called(ctx, raterID, userRatedID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Rating), args.Error(1)
}

func (m *MockRatingRepositoryRead) ExistsByRaterAndUserRated(ctx context.Context, raterID string, userRatedID string) (bool, error) {
	args := m.Called(ctx, raterID, userRatedID)
	return args.Bool(0), args.Error(1)
}

func (m *MockRatingRepositoryRead) GetAverageByUserRatedID(ctx context.Context, userRatedID string) (float64, int32, error) {
	args := m.Called(ctx, userRatedID)
	return args.Get(0).(float64), args.Get(1).(int32), args.Error(2)
}
