package mocks

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/vehicle-service/internal/domain"
	"github.com/stretchr/testify/mock"
)

type MockVehicleRepositoryRead struct {
	mock.Mock
}

func (m *MockVehicleRepositoryRead) GetByID(ctx context.Context, vehicleID string) (*domain.Vehicle, error) {
	args := m.Called(ctx, vehicleID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Vehicle), args.Error(1)
}

func (m *MockVehicleRepositoryRead) GetByUserID(ctx context.Context, userID string) ([]*domain.VehiclePreview, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.VehiclePreview), args.Error(1)
}

func (m *MockVehicleRepositoryRead) ExistsByLicencePlate(ctx context.Context, licencePlate string) (bool, error) {
	args := m.Called(ctx, licencePlate)
	return args.Bool(0), args.Error(1)
}
