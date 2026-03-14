package mocks

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/trips-service/internal/domain"
	"github.com/stretchr/testify/mock"
)

// MockTripRepositoryRead est un mock du repository de lecture TripRepositoryRead.
type MockTripRepositoryRead struct {
	mock.Mock
}

func (m *MockTripRepositoryRead) GetDriverTripsPreviews(ctx context.Context, driverID string, pageIndex int) ([]*domain.TripPreview, error) {
	args := m.Called(ctx, driverID, pageIndex)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.TripPreview), args.Error(1)
}

func (m *MockTripRepositoryRead) GetDriverCompletedTripsPreviews(ctx context.Context, driverID string, pageIndex int) ([]*domain.TripPreview, error) {
	args := m.Called(ctx, driverID, pageIndex)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.TripPreview), args.Error(1)
}

func (m *MockTripRepositoryRead) GetTripTotalSeats(ctx context.Context, tripID string) (int16, error) {
	args := m.Called(ctx, tripID)
	if args.Get(1) != nil {
		return 0, args.Error(1)
	}
	return args.Get(0).(int16), nil
}
