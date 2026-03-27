package mocks

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/booking-service/internal/client"
	"github.com/stretchr/testify/mock"
)

// MockTripClient mock du client trips-service.
type MockTripClient struct {
	mock.Mock
}

func (m *MockTripClient) GetTripDetails(ctx context.Context, tripID string) (*client.TripDetails, error) {
	args := m.Called(ctx, tripID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*client.TripDetails), args.Error(1)
}

func (m *MockTripClient) UpdateAvailableSeats(ctx context.Context, tripID string, newAvailableSeats int) error {
	args := m.Called(ctx, tripID, newAvailableSeats)
	return args.Error(0)
}

func (m *MockTripClient) Close() error {
	args := m.Called()
	return args.Error(0)
}

// MockUserClient mock du client user-service.
type MockUserClient struct {
	mock.Mock
}

func (m *MockUserClient) UserExists(ctx context.Context, userID string) (bool, error) {
	args := m.Called(ctx, userID)
	return args.Bool(0), args.Error(1)
}

func (m *MockUserClient) Close() error {
	args := m.Called()
	return args.Error(0)
}

// MockPaymentClient mock du client payment-service.
type MockPaymentClient struct {
	mock.Mock
}

func (m *MockPaymentClient) RequestRefund(ctx context.Context, input *client.RefundInput) error {
	args := m.Called(ctx, input)
	return args.Error(0)
}

func (m *MockPaymentClient) ReleasePayment(ctx context.Context, bookingID string) error {
	args := m.Called(ctx, bookingID)
	return args.Error(0)
}

func (m *MockPaymentClient) Close() error {
	args := m.Called()
	return args.Error(0)
}
