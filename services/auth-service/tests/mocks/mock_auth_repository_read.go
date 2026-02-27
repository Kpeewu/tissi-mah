package mocks

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/auth-service/internal/domain"
	"github.com/stretchr/testify/mock"
)

type MockAuthRepositoryRead struct {
	mock.Mock
}

func (m *MockAuthRepositoryRead) GetByFirebaseID(ctx context.Context, firebaseID string) (*domain.Auth, error) {
	args := m.Called(ctx, firebaseID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Auth), args.Error(1)
}

func (m *MockAuthRepositoryRead) GetByAuthID(ctx context.Context, authID string) (*domain.Auth, error) {
	args := m.Called(ctx, authID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Auth), args.Error(1)
}

func (m *MockAuthRepositoryRead) GetByEmail(ctx context.Context, email string) (*domain.Auth, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Auth), args.Error(1)
}

func (m *MockAuthRepositoryRead) GetByPhoneNumber(ctx context.Context, phoneNumber string) (*domain.Auth, error) {
	args := m.Called(ctx, phoneNumber)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Auth), args.Error(1)
}

func (m *MockAuthRepositoryRead) EmailExists(ctx context.Context, email string) (bool, error) {
	args := m.Called(ctx, email)
	return args.Bool(0), args.Error(1)
}

func (m *MockAuthRepositoryRead) PhoneNumberExists(ctx context.Context, phoneNumber string) (bool, error) {
	args := m.Called(ctx, phoneNumber)
	return args.Bool(0), args.Error(1)
}
