package mocks

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/file-service/internal/domain"
	"github.com/stretchr/testify/mock"
)

type MockVehicleDocumentRepositoryWrite struct {
	mock.Mock
}

func (m *MockVehicleDocumentRepositoryWrite) Create(ctx context.Context, doc *domain.VehicleDocument) (string, error) {
	args := m.Called(ctx, doc)
	return args.String(0), args.Error(1)
}

func (m *MockVehicleDocumentRepositoryWrite) Update(ctx context.Context, doc *domain.VehicleDocument) (*domain.VehicleDocument, error) {
	args := m.Called(ctx, doc)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.VehicleDocument), args.Error(1)
}

func (m *MockVehicleDocumentRepositoryWrite) Delete(ctx context.Context, documentID string) error {
	args := m.Called(ctx, documentID)
	return args.Error(0)
}

func (m *MockVehicleDocumentRepositoryWrite) MarkAsReplaced(ctx context.Context, documentID string, replacedBy string) error {
	args := m.Called(ctx, documentID, replacedBy)
	return args.Error(0)
}

func (m *MockVehicleDocumentRepositoryWrite) DeleteAllByUserID(ctx context.Context, userID string) ([]string, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}
