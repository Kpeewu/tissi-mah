package mocks

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/notification-service/internal/domain"
	"github.com/stretchr/testify/mock"
)

type MockInboxRepository struct {
	mock.Mock
}

func (m *MockInboxRepository) Create(ctx context.Context, entry *domain.InboxEntry) error {
	args := m.Called(ctx, entry)
	return args.Error(0)
}

func (m *MockInboxRepository) GetByUserID(ctx context.Context, userID string, page, pageSize int) ([]*domain.InboxEntry, int, error) {
	args := m.Called(ctx, userID, page, pageSize)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*domain.InboxEntry), args.Int(1), args.Error(2)
}

func (m *MockInboxRepository) MarkAsRead(ctx context.Context, inboxID, userID string) error {
	args := m.Called(ctx, inboxID, userID)
	return args.Error(0)
}

func (m *MockInboxRepository) MarkAllAsRead(ctx context.Context, userID string) (int, error) {
	args := m.Called(ctx, userID)
	return args.Int(0), args.Error(1)
}

func (m *MockInboxRepository) GetUnreadCount(ctx context.Context, userID string) (int, error) {
	args := m.Called(ctx, userID)
	return args.Int(0), args.Error(1)
}

func (m *MockInboxRepository) PurgeOldRead(ctx context.Context, days int) (int64, error) {
	args := m.Called(ctx, days)
	return args.Get(0).(int64), args.Error(1)
}
