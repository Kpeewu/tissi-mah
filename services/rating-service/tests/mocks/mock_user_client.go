package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"
)

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
