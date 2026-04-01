package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"
)

// MockUserClient est un mock du client UserClient.
type MockUserClient struct {
	mock.Mock
}

func (m *MockUserClient) IsVerifiedDriver(ctx context.Context, userID string) (bool, error) {
	args := m.Called(ctx, userID)
	return args.Bool(0), args.Error(1)
}

func (m *MockUserClient) GetDriverName(ctx context.Context, userID string) (string, error) {
	args := m.Called(ctx, userID)
	return args.String(0), args.Error(1)
}

func (m *MockUserClient) GetDriverInfo(ctx context.Context, userID string) (string, string, error) {
	args := m.Called(ctx, userID)
	return args.String(0), args.String(1), args.Error(2)
}

func (m *MockUserClient) Close() error {
	args := m.Called()
	return args.Error(0)
}
