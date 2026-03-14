package mocks

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/trips-service/internal/domain"
	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/trips-service/internal/service/interfaces"
	"github.com/stretchr/testify/mock"
)

// MockTripService est un mock du service TripService.
type MockTripService struct {
	mock.Mock
}

func (m *MockTripService) CreateTrip(ctx context.Context, input *serviceInterfaces.CreateTripInput) (*domain.Trip, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Trip), args.Error(1)
}

func (m *MockTripService) CreateRecurringTrip(ctx context.Context, input *serviceInterfaces.CreateRecurringTripInput) (string, error) {
	args := m.Called(ctx, input)
	return args.String(0), args.Error(1)
}

func (m *MockTripService) GetTripsPreviews(ctx context.Context, input *serviceInterfaces.GetTripsPreviewsInput) ([]*serviceInterfaces.TripPreviewResult, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*serviceInterfaces.TripPreviewResult), args.Error(1)
}

func (m *MockTripService) GetCompletedTripsPreviews(ctx context.Context, input *serviceInterfaces.GetTripsPreviewsInput) ([]*serviceInterfaces.CompletedTripPreviewResult, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*serviceInterfaces.CompletedTripPreviewResult), args.Error(1)
}

func (m *MockTripService) ChangeTripDateAndTime(ctx context.Context, input *serviceInterfaces.ChangeTripDateAndTimeInput) error {
	args := m.Called(ctx, input)
	return args.Error(0)
}

func (m *MockTripService) ChangeTripVehicle(ctx context.Context, input *serviceInterfaces.ChangeTripVehicleInput) error {
	args := m.Called(ctx, input)
	return args.Error(0)
}

func (m *MockTripService) ChangeTripAllowances(ctx context.Context, input *serviceInterfaces.ChangeTripAllowancesInput) error {
	args := m.Called(ctx, input)
	return args.Error(0)
}

func (m *MockTripService) ChangeAutoApprove(ctx context.Context, input *serviceInterfaces.ChangeAutoApproveInput) error {
	args := m.Called(ctx, input)
	return args.Error(0)
}

func (m *MockTripService) StartTrip(ctx context.Context, input *serviceInterfaces.StartTripInput) error {
	args := m.Called(ctx, input)
	return args.Error(0)
}

func (m *MockTripService) EndTrip(ctx context.Context, input *serviceInterfaces.EndTripInput) error {
	args := m.Called(ctx, input)
	return args.Error(0)
}

func (m *MockTripService) ConfirmWaypointArrival(ctx context.Context, input *serviceInterfaces.ConfirmWaypointArrivalInput) error {
	args := m.Called(ctx, input)
	return args.Error(0)
}

func (m *MockTripService) ConfirmWaypointDeparture(ctx context.Context, input *serviceInterfaces.ConfirmWaypointDepartureInput) error {
	args := m.Called(ctx, input)
	return args.Error(0)
}
