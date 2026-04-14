package mocks

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/booking-service/internal/domain"
	"github.com/stretchr/testify/mock"
)

type MockBookingRepositoryWrite struct {
	mock.Mock
}

func (m *MockBookingRepositoryWrite) Create(ctx context.Context, booking *domain.Booking, segments []*domain.Segment, history *domain.StatusHistoryEntry) error {
	args := m.Called(ctx, booking, segments, history)
	return args.Error(0)
}

func (m *MockBookingRepositoryWrite) Approve(ctx context.Context, bookingID, driverID string) error {
	args := m.Called(ctx, bookingID, driverID)
	return args.Error(0)
}

func (m *MockBookingRepositoryWrite) Reject(ctx context.Context, bookingID, driverID, reason string) error {
	args := m.Called(ctx, bookingID, driverID, reason)
	return args.Error(0)
}

func (m *MockBookingRepositoryWrite) Cancel(ctx context.Context, bookingID, cancellerID, reason string) error {
	args := m.Called(ctx, bookingID, cancellerID, reason)
	return args.Error(0)
}

func (m *MockBookingRepositoryWrite) StartBookingsForWaypoint(ctx context.Context, tripID, waypointID string) ([]string, error) {
	args := m.Called(ctx, tripID, waypointID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockBookingRepositoryWrite) CompleteBookingsForWaypoint(ctx context.Context, tripID, waypointID string) ([]string, error) {
	args := m.Called(ctx, tripID, waypointID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockBookingRepositoryWrite) ReportNoShow(ctx context.Context, bookingID, reporterID, noShowType, description string) error {
	args := m.Called(ctx, bookingID, reporterID, noShowType, description)
	return args.Error(0)
}

func (m *MockBookingRepositoryWrite) ConfirmPayment(ctx context.Context, bookingID, transactionID string) error {
	args := m.Called(ctx, bookingID, transactionID)
	return args.Error(0)
}

func (m *MockBookingRepositoryWrite) FailPayment(ctx context.Context, bookingID, reason string) error {
	args := m.Called(ctx, bookingID, reason)
	return args.Error(0)
}

func (m *MockBookingRepositoryWrite) CancelBookingsForTrip(ctx context.Context, tripID string) ([]*domain.Booking, error) {
	args := m.Called(ctx, tripID)
	if b, ok := args.Get(0).([]*domain.Booking); ok {
		return b, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockBookingRepositoryWrite) CancelBookingsForWaypoint(ctx context.Context, tripID, waypointID string) ([]*domain.Booking, error) {
	args := m.Called(ctx, tripID, waypointID)
	if b, ok := args.Get(0).([]*domain.Booking); ok {
		return b, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockBookingRepositoryWrite) MarkPaymentReleased(ctx context.Context, bookingID string) error {
	args := m.Called(ctx, bookingID)
	return args.Error(0)
}
