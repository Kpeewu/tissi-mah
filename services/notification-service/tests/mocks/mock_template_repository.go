package mocks

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/notification-service/internal/domain"
	"github.com/stretchr/testify/mock"
)

type MockTemplateRepository struct {
	mock.Mock
}

func (m *MockTemplateRepository) GetByEventTypeAndChannel(ctx context.Context, eventType, channel, languageCode string) (*domain.Template, error) {
	args := m.Called(ctx, eventType, channel, languageCode)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Template), args.Error(1)
}
