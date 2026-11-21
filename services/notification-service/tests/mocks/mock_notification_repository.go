package mocks

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/notification-service/internal/domain"
	"github.com/stretchr/testify/mock"
)

type MockNotificationRepository struct {
	mock.Mock
}

func (m *MockNotificationRepository) Create(ctx context.Context, n *domain.Notification) error {
	args := m.Called(ctx, n)
	return args.Error(0)
}

func (m *MockNotificationRepository) UpdateStatus(ctx context.Context, notificationID, status, failureReason, providerMessageID string) error {
	args := m.Called(ctx, notificationID, status, failureReason, providerMessageID)
	return args.Error(0)
}

func (m *MockNotificationRepository) UpdateRetry(ctx context.Context, n *domain.Notification) error {
	args := m.Called(ctx, n)
	return args.Error(0)
}

func (m *MockNotificationRepository) ExistsByEventIDAndChannel(ctx context.Context, eventID, channel string) (bool, error) {
	args := m.Called(ctx, eventID, channel)
	return args.Bool(0), args.Error(1)
}

func (m *MockNotificationRepository) GetPendingForRetry(ctx context.Context, limit int) ([]*domain.Notification, error) {
	args := m.Called(ctx, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Notification), args.Error(1)
}

func (m *MockNotificationRepository) PurgeOldSent(ctx context.Context, days int) (int64, error) {
	args := m.Called(ctx, days)
	return args.Get(0).(int64), args.Error(1)
}
