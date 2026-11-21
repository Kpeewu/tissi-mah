package mocks

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/rating-service/internal/domain"
	"github.com/stretchr/testify/mock"
)

type MockRatingRepositoryWrite struct {
	mock.Mock
}

func (m *MockRatingRepositoryWrite) Create(ctx context.Context, rating *domain.Rating) (string, error) {
	args := m.Called(ctx, rating)
	return args.String(0), args.Error(1)
}

func (m *MockRatingRepositoryWrite) Update(ctx context.Context, rating *domain.Rating) (*domain.Rating, error) {
	args := m.Called(ctx, rating)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Rating), args.Error(1)
}
