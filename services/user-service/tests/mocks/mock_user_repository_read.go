package mocks

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/user-service/internal/domain"
	"github.com/stretchr/testify/mock"
)

type MockUserRepositoryRead struct {
	mock.Mock
}

func (m *MockUserRepositoryRead) GetByUserID(ctx context.Context, userID string) (*domain.User, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserRepositoryRead) GetByUserIDs(ctx context.Context, userIDs []string) ([]*domain.User, error) {
	args := m.Called(ctx, userIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.User), args.Error(1)
}

func (m *MockUserRepositoryRead) GetByAuthID(ctx context.Context, authID string) (*domain.User, error) {
	args := m.Called(ctx, authID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserRepositoryRead) GetByFirebaseID(ctx context.Context, firebaseID string) (*domain.User, error) {
	args := m.Called(ctx, firebaseID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}
