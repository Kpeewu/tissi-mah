package mocks

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/vehicle-service/internal/domain"
	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/vehicle-service/internal/service/interfaces"
	"github.com/stretchr/testify/mock"
)

type MockVehicleService struct {
	mock.Mock
}

func (m *MockVehicleService) AddVehicle(ctx context.Context, input serviceInterfaces.AddVehicleInput) (string, error) {
	args := m.Called(ctx, input)
	return args.String(0), args.Error(1)
}

func (m *MockVehicleService) GetVehicleDetails(ctx context.Context, userID string, vehicleID string) (*domain.VehicleDetails, error) {
	args := m.Called(ctx, userID, vehicleID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.VehicleDetails), args.Error(1)
}

func (m *MockVehicleService) GetVehicleInfo(ctx context.Context, vehicleID string) (*domain.Vehicle, error) {
	args := m.Called(ctx, vehicleID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Vehicle), args.Error(1)
}

func (m *MockVehicleService) GetUserVehicles(ctx context.Context, userID string) ([]*domain.VehiclePreview, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.VehiclePreview), args.Error(1)
}

func (m *MockVehicleService) UpdateVehicle(ctx context.Context, input serviceInterfaces.UpdateVehicleInput) error {
	args := m.Called(ctx, input)
	return args.Error(0)
}

func (m *MockVehicleService) DeleteVehicle(ctx context.Context, userID string, vehicleID string) error {
	args := m.Called(ctx, userID, vehicleID)
	return args.Error(0)
}

func (m *MockVehicleService) VerifyVehicle(ctx context.Context, vehicleID string, isVerified bool) error {
	args := m.Called(ctx, vehicleID, isVerified)
	return args.Error(0)
}
