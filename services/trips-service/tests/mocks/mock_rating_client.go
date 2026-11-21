package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"
)

// MockRatingClient est un mock du client RatingClient.
type MockRatingClient struct {
	mock.Mock
}

// GetDriverRatingAverage renvoie les valeurs programmées : moyenne, nombre de notes, erreur.
func (m *MockRatingClient) GetDriverRatingAverage(ctx context.Context, driverID string) (float64, int32, error) {
	args := m.Called(ctx, driverID)
	return args.Get(0).(float64), int32(args.Int(1)), args.Error(2)
}

func (m *MockRatingClient) Close() error {
	args := m.Called()
	return args.Error(0)
}
