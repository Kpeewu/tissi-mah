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

func (m *MockUserService) CreateUser(ctx context.Context, authID string, firebaseID string, name string, firstName string, profilePhotoURL string, birthDate string) (*domain.User, error) {
	args := m.Called(ctx, authID, firebaseID, name, firstName, profilePhotoURL, birthDate)
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

func (m *MockUserService) GetUserByFirebaseID(ctx context.Context, firebaseID string) (*domain.User, error) {
	args := m.Called(ctx, firebaseID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserService) GetUsersByUserIDs(ctx context.Context, userIDs []string) ([]*domain.User, error) {
	args := m.Called(ctx, userIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.User), args.Error(1)
}

func (m *MockUserService) GetUserProfileByUserID(ctx context.Context, userID string) (*domain.User, string, string, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.String(1), args.String(2), args.Error(3)
	}
	return args.Get(0).(*domain.User), args.String(1), args.String(2), args.Error(3)
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

func (m *MockUserService) CreateDriverAccount(ctx context.Context, createDriver bool) error {
	args := m.Called(ctx, createDriver)
	return args.Error(0)
}

func (m *MockUserService) AddTripPreferences(ctx context.Context, preferences []domain.TripPreference) error {
	args := m.Called(ctx, preferences)
	return args.Error(0)
}

func (m *MockUserService) UpdateProfile(ctx context.Context, req serviceInterfaces.UpdateProfileRequest) (*serviceInterfaces.FullProfile, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*serviceInterfaces.FullProfile), args.Error(1)
}

func (m *MockUserService) SoftDeleteUser(ctx context.Context, authID string) error {
	args := m.Called(ctx, authID)
	return args.Error(0)
}

func (m *MockUserService) UpdateProfileVerification(ctx context.Context, userID string, driver, passenger *bool) (bool, bool, error) {
	args := m.Called(ctx, userID, driver, passenger)
	return args.Bool(0), args.Bool(1), args.Error(2)
}
