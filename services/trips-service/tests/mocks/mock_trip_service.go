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

func (m *MockTripService) GetTripByID(ctx context.Context, input *serviceInterfaces.GetTripByIDInput) (*serviceInterfaces.TripDetailResult, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*serviceInterfaces.TripDetailResult), args.Error(1)
}

func (m *MockTripService) UpdateAvailableSeats(ctx context.Context, input *serviceInterfaces.UpdateAvailableSeatsInput) error {
	args := m.Called(ctx, input)
	return args.Error(0)
}

func (m *MockTripService) CancelTrip(ctx context.Context, input *serviceInterfaces.CancelTripInput) error {
	args := m.Called(ctx, input)
	return args.Error(0)
}

func (m *MockTripService) CancelWaypoint(ctx context.Context, input *serviceInterfaces.CancelWaypointInput) error {
	args := m.Called(ctx, input)
	return args.Error(0)
}

func (m *MockTripService) GetScheduledTripsPreviews(ctx context.Context, input *serviceInterfaces.GetScheduledTripsPreviewsInput) (*serviceInterfaces.ScheduledTripsPreviewsResult, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*serviceInterfaces.ScheduledTripsPreviewsResult), args.Error(1)
}

func (m *MockTripService) IncrementLegBookedSeats(ctx context.Context, input *serviceInterfaces.IncrementLegBookedSeatsInput) error {
	args := m.Called(ctx, input)
	return args.Error(0)
}

func (m *MockTripService) SyncLegBookedSeats(ctx context.Context, input *serviceInterfaces.SyncLegBookedSeatsInput) error {
	args := m.Called(ctx, input)
	return args.Error(0)
}

func (m *MockTripService) GetDriverTripDetails(ctx context.Context, input *serviceInterfaces.GetDriverTripDetailsInput) (*serviceInterfaces.DriverTripDetailResult, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*serviceInterfaces.DriverTripDetailResult), args.Error(1)
}

func (m *MockTripService) GetPassengerTripDetails(ctx context.Context, input *serviceInterfaces.GetPassengerTripDetailsInput) (*serviceInterfaces.PassengerTripDetailResult, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*serviceInterfaces.PassengerTripDetailResult), args.Error(1)
}

// compile-time check
var _ serviceInterfaces.TripService = (*MockTripService)(nil)

func (m *MockTripService) CheckDeletionEligibility(ctx context.Context, userID string) (bool, string, error) {
	args := m.Called(ctx, userID)
	return args.Bool(0), args.String(1), args.Error(2)
}

func (m *MockTripService) AnonymizeUserData(ctx context.Context, userID string) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}
