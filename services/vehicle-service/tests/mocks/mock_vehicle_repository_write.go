package mocks

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/vehicle-service/internal/domain"
	"github.com/stretchr/testify/mock"
)

type MockVehicleRepositoryWrite struct {
	mock.Mock
}

func (m *MockVehicleRepositoryWrite) Create(ctx context.Context, vehicle *domain.Vehicle) (string, error) {
	args := m.Called(ctx, vehicle)
	return args.String(0), args.Error(1)
}

func (m *MockVehicleRepositoryWrite) Update(ctx context.Context, vehicle *domain.Vehicle) (*domain.Vehicle, error) {
	args := m.Called(ctx, vehicle)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Vehicle), args.Error(1)
}

func (m *MockVehicleRepositoryWrite) Delete(ctx context.Context, vehicleID string) error {
	args := m.Called(ctx, vehicleID)
	return args.Error(0)
}

func (m *MockVehicleRepositoryWrite) SetVerified(ctx context.Context, vehicleID string, isVerified bool) error {
	args := m.Called(ctx, vehicleID, isVerified)
	return args.Error(0)
}
