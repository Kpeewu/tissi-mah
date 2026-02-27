package mocks

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/user-service/internal/domain"
	"github.com/stretchr/testify/mock"
)

type MockUserRepositoryWrite struct {
	mock.Mock
}

func (m *MockUserRepositoryWrite) Create(ctx context.Context, user *domain.User) (string, error) {
	args := m.Called(ctx, user)
	return args.String(0), args.Error(1)
}

func (m *MockUserRepositoryWrite) Update(ctx context.Context, user *domain.User) (*domain.User, error) {
	args := m.Called(ctx, user)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserRepositoryWrite) Delete(ctx context.Context, userID string) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}
