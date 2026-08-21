package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"
)

// MockVehicleClient est un mock du client VehicleClient.
type MockVehicleClient struct {
	mock.Mock
}

func (m *MockVehicleClient) GetVehicleInfo(ctx context.Context, driverID, vehicleID string) (brand, plate string, numberOfSeats int, isVerified bool, err error) {
	args := m.Called(ctx, driverID, vehicleID)
	return args.String(0), args.String(1), args.Int(2), args.Bool(3), args.Error(4)
}

func (m *MockVehicleClient) Close() error {
	args := m.Called()
	return args.Error(0)
}
