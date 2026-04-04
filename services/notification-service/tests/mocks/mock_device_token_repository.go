package mocks

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/notification-service/internal/domain"
	"github.com/stretchr/testify/mock"
)

type MockDeviceTokenRepository struct {
	mock.Mock
}

func (m *MockDeviceTokenRepository) GetActiveByUserID(ctx context.Context, userID string) ([]*domain.UserDeviceToken, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.UserDeviceToken), args.Error(1)
}

func (m *MockDeviceTokenRepository) Upsert(ctx context.Context, token *domain.UserDeviceToken) (string, error) {
	args := m.Called(ctx, token)
	return args.String(0), args.Error(1)
}

func (m *MockDeviceTokenRepository) InvalidateByFCMToken(ctx context.Context, fcmToken string) error {
	args := m.Called(ctx, fcmToken)
	return args.Error(0)
}

func (m *MockDeviceTokenRepository) DeleteByFCMToken(ctx context.Context, fcmToken string) error {
	args := m.Called(ctx, fcmToken)
	return args.Error(0)
}
