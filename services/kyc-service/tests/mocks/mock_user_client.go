package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"
)

type MockUserClient struct {
	mock.Mock
}

func (m *MockUserClient) GetUserIDByFirebaseID(ctx context.Context, firebaseUID string) (string, error) {
	args := m.Called(ctx, firebaseUID)
	// Support both static string returns and callback functions (for pass-through mocks).
	if fn, ok := args.Get(0).(func(context.Context, string) string); ok {
		return fn(ctx, firebaseUID), args.Error(1)
	}
	return args.String(0), args.Error(1)
}

func (m *MockUserClient) Close() error {
	args := m.Called()
	return args.Error(0)
}
