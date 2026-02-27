package mocks

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/auth-service/internal/domain"
	"github.com/stretchr/testify/mock"
)

type MockUserClient struct {
	mock.Mock
}

func (m *MockUserClient) CreateUser(ctx context.Context, authID string, firebaseID string, name string, firstName string, profilePhotoURL string) (*domain.UserPreview, error) {
	args := m.Called(ctx, authID, firebaseID, name, firstName, profilePhotoURL)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.UserPreview), args.Error(1)
}

func (m *MockUserClient) GetUserByAuthID(ctx context.Context, authID string) (*domain.UserPreview, error) {
	args := m.Called(ctx, authID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.UserPreview), args.Error(1)
}

func (m *MockUserClient) Close() error {
	args := m.Called()
	return args.Error(0)
}
