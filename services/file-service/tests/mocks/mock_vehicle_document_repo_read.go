package mocks

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/file-service/internal/domain"
	"github.com/stretchr/testify/mock"
)

type MockVehicleDocumentRepositoryRead struct {
	mock.Mock
}

func (m *MockVehicleDocumentRepositoryRead) GetByID(ctx context.Context, documentID string) (*domain.VehicleDocument, error) {
	args := m.Called(ctx, documentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.VehicleDocument), args.Error(1)
}

func (m *MockVehicleDocumentRepositoryRead) GetByVehicleID(ctx context.Context, vehicleID string) ([]*domain.VehicleDocument, error) {
	args := m.Called(ctx, vehicleID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.VehicleDocument), args.Error(1)
}

func (m *MockVehicleDocumentRepositoryRead) GetByUserID(ctx context.Context, userID string) ([]*domain.VehicleDocument, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.VehicleDocument), args.Error(1)
}

func (m *MockVehicleDocumentRepositoryRead) GetCurrentByVehicleIDAndType(ctx context.Context, vehicleID string, documentType string) (*domain.VehicleDocument, error) {
	args := m.Called(ctx, vehicleID, documentType)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.VehicleDocument), args.Error(1)
}

func (m *MockVehicleDocumentRepositoryRead) ListCurrentByStatuses(ctx context.Context, statuses []string, limit int32) ([]*domain.VehicleDocument, error) {
	args := m.Called(ctx, statuses, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.VehicleDocument), args.Error(1)
}
