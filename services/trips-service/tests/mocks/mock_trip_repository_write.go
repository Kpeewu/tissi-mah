package mocks

import (
	"context"
	"time"

	"github.com/Kpeewu/tissi-mah/services/trips-service/internal/domain"
	"github.com/stretchr/testify/mock"
)

// MockTripRepositoryWrite est un mock du repository d'écriture TripRepositoryWrite.
type MockTripRepositoryWrite struct {
	mock.Mock
}

func (m *MockTripRepositoryWrite) Create(ctx context.Context, trip *domain.Trip, waypoints []*domain.Waypoint) (string, error) {
	args := m.Called(ctx, trip, waypoints)
	return args.String(0), args.Error(1)
}

func (m *MockTripRepositoryWrite) CreateRecurringPattern(ctx context.Context, pattern *domain.RecurringPattern, patternWaypoints []*domain.PatternWaypoint) (string, error) {
	args := m.Called(ctx, pattern, patternWaypoints)
	return args.String(0), args.Error(1)
}

func (m *MockTripRepositoryWrite) UpdateDepartureDatetime(ctx context.Context, tripID, driverID string, newDatetime time.Time) error {
	args := m.Called(ctx, tripID, driverID, newDatetime)
	return args.Error(0)
}

func (m *MockTripRepositoryWrite) UpdateVehicle(ctx context.Context, tripID, driverID, vehicleID string) error {
	args := m.Called(ctx, tripID, driverID, vehicleID)
	return args.Error(0)
}

func (m *MockTripRepositoryWrite) UpdateAllowances(ctx context.Context, tripID, driverID string, allowPets, allowFood, allowSmoking, allowLuggages bool) error {
	args := m.Called(ctx, tripID, driverID, allowPets, allowFood, allowSmoking, allowLuggages)
	return args.Error(0)
}

func (m *MockTripRepositoryWrite) UpdateAutoApprove(ctx context.Context, tripID, driverID string, autoApprove bool) error {
	args := m.Called(ctx, tripID, driverID, autoApprove)
	return args.Error(0)
}

func (m *MockTripRepositoryWrite) StartTrip(ctx context.Context, tripID, driverID string) error {
	args := m.Called(ctx, tripID, driverID)
	return args.Error(0)
}

func (m *MockTripRepositoryWrite) EndTrip(ctx context.Context, tripID, driverID string) error {
	args := m.Called(ctx, tripID, driverID)
	return args.Error(0)
}

func (m *MockTripRepositoryWrite) ConfirmWaypointArrival(ctx context.Context, waypointID, driverID string) error {
	args := m.Called(ctx, waypointID, driverID)
	return args.Error(0)
}

func (m *MockTripRepositoryWrite) ConfirmWaypointDeparture(ctx context.Context, waypointID, driverID string) error {
	args := m.Called(ctx, waypointID, driverID)
	return args.Error(0)
}
