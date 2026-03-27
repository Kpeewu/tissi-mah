package mocks

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/domain"
	"github.com/stretchr/testify/mock"
)

type MockPaymentRepositoryWrite struct {
	mock.Mock
}

func (m *MockPaymentRepositoryWrite) CreatePayment(ctx context.Context, payment *domain.Payment) error {
	args := m.Called(ctx, payment)
	return args.Error(0)
}

func (m *MockPaymentRepositoryWrite) UpdatePaymentStatus(ctx context.Context, paymentID string, status domain.PaymentStatus, externalTransactionID string) error {
	args := m.Called(ctx, paymentID, status, externalTransactionID)
	return args.Error(0)
}

func (m *MockPaymentRepositoryWrite) MarkPaymentFailed(ctx context.Context, paymentID string, reason string) error {
	args := m.Called(ctx, paymentID, reason)
	return args.Error(0)
}

func (m *MockPaymentRepositoryWrite) SaveWebhookEvent(ctx context.Context, event *domain.WebhookEvent) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}
