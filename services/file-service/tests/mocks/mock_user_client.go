package mocks

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/file-service/internal/client"
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

func (m *MockUserClient) GetUserProfileByFirebaseID(ctx context.Context, firebaseUID string) (*client.UserProfile, error) {
	args := m.Called(ctx, firebaseUID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*client.UserProfile), args.Error(1)
}

func (m *MockUserClient) Close() error {
	args := m.Called()
	return args.Error(0)
}
