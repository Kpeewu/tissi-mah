package mocks

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/file-service/internal/client"
	"github.com/stretchr/testify/mock"
)

// MockVehicleClient est un mock de client.VehicleClient pour les tests.
type MockVehicleClient struct {
	mock.Mock
}

func (m *MockVehicleClient) GetVehicleInfo(ctx context.Context, vehicleID string) (*client.VehicleInfo, error) {
	args := m.Called(ctx, vehicleID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*client.VehicleInfo), args.Error(1)
}

func (m *MockVehicleClient) Close() error {
	args := m.Called()
	return args.Error(0)
}
