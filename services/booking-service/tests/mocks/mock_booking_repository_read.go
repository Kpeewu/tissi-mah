package mocks

import (
	"context"
	"time"

	"github.com/Kpeewu/tissi-mah/services/booking-service/internal/domain"
	"github.com/stretchr/testify/mock"
)

type MockBookingRepositoryRead struct {
	mock.Mock
}

func (m *MockBookingRepositoryRead) GetByID(ctx context.Context, bookingID string) (*domain.Booking, error) {
	args := m.Called(ctx, bookingID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Booking), args.Error(1)
}

func (m *MockBookingRepositoryRead) GetByIDWithDetails(ctx context.Context, bookingID string) (*domain.Booking, []*domain.Segment, []*domain.StatusHistoryEntry, error) {
	args := m.Called(ctx, bookingID)
	var booking *domain.Booking
	var segments []*domain.Segment
	var history []*domain.StatusHistoryEntry
	if args.Get(0) != nil {
		booking = args.Get(0).(*domain.Booking)
	}
	if args.Get(1) != nil {
		segments = args.Get(1).([]*domain.Segment)
	}
	if args.Get(2) != nil {
		history = args.Get(2).([]*domain.StatusHistoryEntry)
	}
	return booking, segments, history, args.Error(3)
}

func (m *MockBookingRepositoryRead) GetPassengerBookings(ctx context.Context, passengerID string, pageIndex int, statusFilter string) ([]*domain.BookingPreview, error) {
	args := m.Called(ctx, passengerID, pageIndex, statusFilter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.BookingPreview), args.Error(1)
}

func (m *MockBookingRepositoryRead) GetDriverTripBookings(ctx context.Context, driverID, tripID string, pageIndex int) ([]*domain.BookingPreview, error) {
	args := m.Called(ctx, driverID, tripID, pageIndex)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.BookingPreview), args.Error(1)
}

func (m *MockBookingRepositoryRead) HasActiveBooking(ctx context.Context, passengerID, tripID string) (bool, error) {
	args := m.Called(ctx, passengerID, tripID)
	return args.Bool(0), args.Error(1)
}

func (m *MockBookingRepositoryRead) GetActiveBookingsSeatsForTrip(ctx context.Context, tripID string) (int, error) {
	args := m.Called(ctx, tripID)
	return args.Int(0), args.Error(1)
}

func (m *MockBookingRepositoryRead) GetActiveTripsWithBookings(ctx context.Context) ([]string, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockBookingRepositoryRead) GetCompletedBookingsPendingRelease(ctx context.Context, completedBefore time.Time) ([]*domain.Booking, error) {
	args := m.Called(ctx, completedBefore)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Booking), args.Error(1)
}
