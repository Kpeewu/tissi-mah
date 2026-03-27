package mocks

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/client"
	"github.com/stretchr/testify/mock"
)

type MockBookingClient struct {
	mock.Mock
}

func (m *MockBookingClient) ConfirmPayment(ctx context.Context, bookingID string, transactionID string) error {
	args := m.Called(ctx, bookingID, transactionID)
	return args.Error(0)
}

func (m *MockBookingClient) GetBookingDetails(ctx context.Context, bookingID string) (*client.BookingDetails, error) {
	args := m.Called(ctx, bookingID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*client.BookingDetails), args.Error(1)
}

type MockUserClient struct {
	mock.Mock
}

func (m *MockUserClient) GetUserByUserID(ctx context.Context, userID string) (*client.UserInfo, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*client.UserInfo), args.Error(1)
}
