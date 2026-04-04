package mocks

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/notification-service/internal/domain"
	"github.com/stretchr/testify/mock"
)

type MockPreferenceRepository struct {
	mock.Mock
}

func (m *MockPreferenceRepository) GetByUserID(ctx context.Context, userID string) (*domain.UserNotificationPreference, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.UserNotificationPreference), args.Error(1)
}

func (m *MockPreferenceRepository) Upsert(ctx context.Context, pref *domain.UserNotificationPreference) error {
	args := m.Called(ctx, pref)
	return args.Error(0)
}
