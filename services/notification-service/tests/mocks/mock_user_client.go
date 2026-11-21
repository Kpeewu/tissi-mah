package mocks

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/notification-service/internal/client"
	"github.com/stretchr/testify/mock"
)

type MockUserClient struct {
	mock.Mock
}

func (m *MockUserClient) GetUserIDByFirebaseID(ctx context.Context, firebaseUID string) (string, error) {
	args := m.Called(ctx, firebaseUID)
	return args.String(0), args.Error(1)
}

func (m *MockUserClient) GetUserByUserID(ctx context.Context, userID string) (*client.UserInfo, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*client.UserInfo), args.Error(1)
}

func (m *MockUserClient) Close() error {
	args := m.Called()
	return args.Error(0)
}
