package mocks

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/domain"
	"github.com/stretchr/testify/mock"
)

type MockPayoutHistoryRepositoryWrite struct {
	mock.Mock
}

func (m *MockPayoutHistoryRepositoryWrite) CreateHistoryEntry(ctx context.Context, entry *domain.PayoutStatusHistory) error {
	args := m.Called(ctx, entry)
	return args.Error(0)
}
