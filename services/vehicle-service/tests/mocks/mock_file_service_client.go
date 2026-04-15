package mocks

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/vehicle-service/internal/domain"
	"github.com/stretchr/testify/mock"
)

type MockFileServiceClient struct {
	mock.Mock
}

func (m *MockFileServiceClient) GetVehicleDocuments(ctx context.Context, vehicleID string) (domain.VehicleDocuments, error) {
	args := m.Called(ctx, vehicleID)
	return args.Get(0).(domain.VehicleDocuments), args.Error(1)
}

func (m *MockFileServiceClient) Close() error {
	args := m.Called()
	return args.Error(0)
}
