package mocks

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/notification-service/internal/domain"
	"github.com/stretchr/testify/mock"
)

type MockNotificationService struct {
	mock.Mock
}

func (m *MockNotificationService) GetInbox(ctx context.Context, userID string, page, pageSize int) ([]*domain.InboxEntry, int, error) {
	args := m.Called(ctx, userID, page, pageSize)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*domain.InboxEntry), args.Int(1), args.Error(2)
}

func (m *MockNotificationService) MarkAsRead(ctx context.Context, inboxID, userID string) error {
	args := m.Called(ctx, inboxID, userID)
	return args.Error(0)
}

func (m *MockNotificationService) MarkAllAsRead(ctx context.Context, userID string) (int, error) {
	args := m.Called(ctx, userID)
	return args.Int(0), args.Error(1)
}

func (m *MockNotificationService) GetUnreadCount(ctx context.Context, userID string) (int, error) {
	args := m.Called(ctx, userID)
	return args.Int(0), args.Error(1)
}

func (m *MockNotificationService) GetPreferences(ctx context.Context, userID string) (*domain.UserNotificationPreference, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.UserNotificationPreference), args.Error(1)
}

func (m *MockNotificationService) UpdatePreferences(ctx context.Context, userID string, pushEnabled, emailEnabled bool) error {
	args := m.Called(ctx, userID, pushEnabled, emailEnabled)
	return args.Error(0)
}

func (m *MockNotificationService) RegisterDeviceToken(ctx context.Context, userID, fcmToken, platform, deviceName string) (string, error) {
	args := m.Called(ctx, userID, fcmToken, platform, deviceName)
	return args.String(0), args.Error(1)
}

func (m *MockNotificationService) UnregisterDeviceToken(ctx context.Context, fcmToken string) error {
	args := m.Called(ctx, fcmToken)
	return args.Error(0)
}

func (m *MockNotificationService) InvalidateDeviceToken(ctx context.Context, fcmToken string) error {
	args := m.Called(ctx, fcmToken)
	return args.Error(0)
}
