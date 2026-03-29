package mocks

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/auth-service/internal/domain"
	"github.com/stretchr/testify/mock"
)

type MockAuthService struct {
	mock.Mock
}

func (m *MockAuthService) GetUserByFirebaseID(ctx context.Context, firebaseID string) (*domain.Auth, error) {
	args := m.Called(ctx, firebaseID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Auth), args.Error(1)
}

func (m *MockAuthService) RegisterUser(ctx context.Context, name string, firstName string, email string, phoneNumber string, profilePhotoURL string) (*domain.UserPreview, error) {
	args := m.Called(ctx, name, firstName, email, phoneNumber, profilePhotoURL)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.UserPreview), args.Error(1)
}

func (m *MockAuthService) CheckEmail(ctx context.Context, email string) (bool, error) {
	args := m.Called(ctx, email)
	return args.Bool(0), args.Error(1)
}

func (m *MockAuthService) CheckPhoneNumber(ctx context.Context, phoneNumber string) (bool, error) {
	args := m.Called(ctx, phoneNumber)
	return args.Bool(0), args.Error(1)
}

func (m *MockAuthService) DeleteUserAccount(ctx context.Context, firebaseID string) error {
	args := m.Called(ctx, firebaseID)
	return args.Error(0)
}

func (m *MockAuthService) GetAuthInfo(ctx context.Context, authID string) (*domain.Auth, error) {
	args := m.Called(ctx, authID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Auth), args.Error(1)
}
