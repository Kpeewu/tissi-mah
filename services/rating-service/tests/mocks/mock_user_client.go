package mocks

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/rating-service/internal/client"
	"github.com/stretchr/testify/mock"
)

type MockUserClient struct {
	mock.Mock
}

func (m *MockUserClient) UserExists(ctx context.Context, userID string) (bool, error) {
	args := m.Called(ctx, userID)
	return args.Bool(0), args.Error(1)
}

func (m *MockUserClient) GetUsersByUserIDs(ctx context.Context, userIDs []string) (map[string]*client.UserProfile, error) {
	args := m.Called(ctx, userIDs)
	if args.Get(0) == nil {
		return map[string]*client.UserProfile{}, args.Error(1)
	}
	return args.Get(0).(map[string]*client.UserProfile), args.Error(1)
}

func (m *MockUserClient) Close() error {
	args := m.Called()
	return args.Error(0)
}
