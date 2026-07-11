package mocks

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/kyc-service/internal/domain"
	"github.com/stretchr/testify/mock"
)

type MockUserClient struct {
	mock.Mock
}

func (m *MockUserClient) GetUserByUserID(ctx context.Context, userID string) (*domain.UserInfo, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.UserInfo), args.Error(1)
}

func (m *MockUserClient) GetUserIDByFirebaseID(ctx context.Context, firebaseUID string) (string, error) {
	args := m.Called(ctx, firebaseUID)
	// Support both static string returns and callback functions (for pass-through mocks).
	if fn, ok := args.Get(0).(func(context.Context, string) string); ok {
		return fn(ctx, firebaseUID), args.Error(1)
	}
	return args.String(0), args.Error(1)
}

func (m *MockUserClient) GetUsersByUserIDs(ctx context.Context, userIDs []string) (map[string]*domain.UserInfo, error) {
	args := m.Called(ctx, userIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]*domain.UserInfo), args.Error(1)
}

func (m *MockUserClient) UpdateProfileVerification(ctx context.Context, userID string, driver, passenger bool) error {
	args := m.Called(ctx, userID, driver, passenger)
	return args.Error(0)
}

func (m *MockUserClient) Close() error {
	args := m.Called()
	return args.Error(0)
}
