package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"
)

// MockUserClient est un mock de client.UserClient pour les tests.
type MockUserClient struct {
	mock.Mock
}

func (m *MockUserClient) GetInternalUserIDByFirebaseID(ctx context.Context, firebaseUID string) (string, error) {
	args := m.Called(ctx, firebaseUID)
	return args.String(0), args.Error(1)
}

func (m *MockUserClient) Close() error {
	args := m.Called()
	return args.Error(0)
}
