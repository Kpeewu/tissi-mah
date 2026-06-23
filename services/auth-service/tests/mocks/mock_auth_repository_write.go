package mocks

import (
	"context"
	"time"

	"github.com/Kpeewu/tissi-mah/services/auth-service/internal/domain"
	"github.com/stretchr/testify/mock"
)

type MockAuthRepositoryWrite struct {
	mock.Mock
}

func (m *MockAuthRepositoryWrite) Create(ctx context.Context, auth *domain.Auth) (string, error) {
	args := m.Called(ctx, auth)
	return args.String(0), args.Error(1)
}

func (m *MockAuthRepositoryWrite) Update(ctx context.Context, auth *domain.Auth) (*domain.Auth, error) {
	args := m.Called(ctx, auth)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Auth), args.Error(1)
}

func (m *MockAuthRepositoryWrite) Delete(ctx context.Context, auth *domain.Auth) error {
	args := m.Called(ctx, auth)
	return args.Error(0)
}

func (m *MockAuthRepositoryWrite) Suspend(ctx context.Context, authID string, suspendedUntil *time.Time, isBanned bool) error {
	args := m.Called(ctx, authID, suspendedUntil, isBanned)
	return args.Error(0)
}
