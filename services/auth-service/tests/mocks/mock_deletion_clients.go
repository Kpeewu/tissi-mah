package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"
)

// MockTripsClient
type MockTripsClient struct{ mock.Mock }

func (m *MockTripsClient) CheckDeletionEligibility(ctx context.Context, userID string) (bool, string, error) {
	args := m.Called(ctx, userID)
	return args.Bool(0), args.String(1), args.Error(2)
}

func (m *MockTripsClient) AnonymizeUserData(ctx context.Context, userID string) error {
	return m.Called(ctx, userID).Error(0)
}

func (m *MockTripsClient) Close() error { return m.Called().Error(0) }

// MockBookingClient
type MockBookingClient struct{ mock.Mock }

func (m *MockBookingClient) CheckDeletionEligibility(ctx context.Context, userID string) (bool, string, error) {
	args := m.Called(ctx, userID)
	return args.Bool(0), args.String(1), args.Error(2)
}

func (m *MockBookingClient) AnonymizeUserData(ctx context.Context, userID string) error {
	return m.Called(ctx, userID).Error(0)
}

func (m *MockBookingClient) GetPassengerBookingIDs(ctx context.Context, userID string) ([]string, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockBookingClient) Close() error { return m.Called().Error(0) }

// MockPaymentClient
type MockPaymentClient struct{ mock.Mock }

func (m *MockPaymentClient) CheckDeletionEligibility(ctx context.Context, userID string) (bool, string, error) {
	args := m.Called(ctx, userID)
	return args.Bool(0), args.String(1), args.Error(2)
}

func (m *MockPaymentClient) AnonymizeUserData(ctx context.Context, userID string, bookingIDs []string) error {
	return m.Called(ctx, userID, bookingIDs).Error(0)
}

func (m *MockPaymentClient) Close() error { return m.Called().Error(0) }

// MockChatClient
type MockChatClient struct{ mock.Mock }

func (m *MockChatClient) AnonymizeUserData(ctx context.Context, userID string) error {
	return m.Called(ctx, userID).Error(0)
}

func (m *MockChatClient) Close() error { return m.Called().Error(0) }

// MockFileClient
type MockFileClient struct{ mock.Mock }

func (m *MockFileClient) DeleteAllUserFiles(ctx context.Context, userID string) error {
	return m.Called(ctx, userID).Error(0)
}

func (m *MockFileClient) Close() error { return m.Called().Error(0) }
