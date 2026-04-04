package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"
)

// MockRatingClient est un mock du client RatingClient.
type MockRatingClient struct {
	mock.Mock
}

func (m *MockRatingClient) GetDriverRatingAverage(ctx context.Context, driverID string) (float64, error) {
	args := m.Called(ctx, driverID)
	return args.Get(0).(float64), args.Error(1)
}

func (m *MockRatingClient) Close() error {
	args := m.Called()
	return args.Error(0)
}
