package mocks

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/user-service/internal/domain"
	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/user-service/internal/service/interfaces"
	"github.com/stretchr/testify/mock"
)

type MockUserService struct {
	mock.Mock
}

func (m *MockUserService) CreateUser(ctx context.Context, authID string, firebaseID string, name string, firstName string, profilePhotoURL string) (*domain.User, error) {
	args := m.Called(ctx, authID, firebaseID, name, firstName, profilePhotoURL)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserService) GetUserByAuthID(ctx context.Context, authID string) (*domain.User, error) {
	args := m.Called(ctx, authID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserService) GetUserByUserID(ctx context.Context, userID string) (*domain.User, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserService) GetMyProfile(ctx context.Context) (*serviceInterfaces.FullProfile, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*serviceInterfaces.FullProfile), args.Error(1)
}

func (m *MockUserService) CreateDriverAccount(ctx context.Context, profileID string, createDriver bool) error {
	args := m.Called(ctx, profileID, createDriver)
	return args.Error(0)
}

func (m *MockUserService) AddTripPreferences(ctx context.Context, profileID string, preferences []domain.TripPreference) error {
	args := m.Called(ctx, profileID, preferences)
	return args.Error(0)
}

func (m *MockUserService) UpdateProfile(ctx context.Context, req serviceInterfaces.UpdateProfileRequest) (*serviceInterfaces.FullProfile, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*serviceInterfaces.FullProfile), args.Error(1)
}
