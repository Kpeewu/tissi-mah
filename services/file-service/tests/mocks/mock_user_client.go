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

// GetUserIDByFirebaseID sert de UserIDResolver à l'intercepteur. Accepte en valeur de retour
// un string ou une func(context.Context, string) string (résolution dynamique).
func (m *MockUserClient) GetUserIDByFirebaseID(ctx context.Context, firebaseUID string) (string, error) {
	args := m.Called(ctx, firebaseUID)
	if fn, ok := args.Get(0).(func(context.Context, string) string); ok {
		return fn(ctx, firebaseUID), args.Error(1)
	}
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
