package mocks

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/notification-service/internal/domain"
	"github.com/stretchr/testify/mock"
)

type MockRoutingRepository struct {
	mock.Mock
}

func (m *MockRoutingRepository) GetByEventType(ctx context.Context, eventType string) (*domain.EventRouting, error) {
	args := m.Called(ctx, eventType)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.EventRouting), args.Error(1)
}

func (m *MockRoutingRepository) GetAll(ctx context.Context) ([]*domain.EventRouting, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.EventRouting), args.Error(1)
}
