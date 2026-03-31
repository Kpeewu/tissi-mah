package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"
)

// MockBookingClient est un mock du client BookingClient.
type MockBookingClient struct {
	mock.Mock
}

func (m *MockBookingClient) StartBookingsForWaypoint(ctx context.Context, tripID, waypointID string) error {
	args := m.Called(ctx, tripID, waypointID)
	return args.Error(0)
}

func (m *MockBookingClient) CompleteBookingsForWaypoint(ctx context.Context, tripID, waypointID string) error {
	args := m.Called(ctx, tripID, waypointID)
	return args.Error(0)
}

func (m *MockBookingClient) CancelBookingsForWaypoint(ctx context.Context, tripID, waypointID string) error {
	args := m.Called(ctx, tripID, waypointID)
	return args.Error(0)
}

func (m *MockBookingClient) CancelBookingsForTrip(ctx context.Context, tripID string) error {
	args := m.Called(ctx, tripID)
	return args.Error(0)
}

func (m *MockBookingClient) Close() error {
	args := m.Called()
	return args.Error(0)
}
