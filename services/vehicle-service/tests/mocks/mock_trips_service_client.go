package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"
)

type MockTripsServiceClient struct {
	mock.Mock
}

func (m *MockTripsServiceClient) GetVehicleCompletedTripCount(ctx context.Context, vehicleID string) (int32, error) {
	args := m.Called(ctx, vehicleID)
	return int32(args.Int(0)), args.Error(1)
}

func (m *MockTripsServiceClient) Close() error {
	args := m.Called()
	return args.Error(0)
}
