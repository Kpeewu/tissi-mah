package mocks

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/domain"
	"github.com/stretchr/testify/mock"
)

type MockPaymentRepositoryRead struct {
	mock.Mock
}

func (m *MockPaymentRepositoryRead) GetByID(ctx context.Context, paymentID string) (*domain.Payment, error) {
	args := m.Called(ctx, paymentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Payment), args.Error(1)
}

func (m *MockPaymentRepositoryRead) GetByBookingID(ctx context.Context, bookingID string) (*domain.Payment, error) {
	args := m.Called(ctx, bookingID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Payment), args.Error(1)
}

func (m *MockPaymentRepositoryRead) GetByExternalTransactionID(ctx context.Context, externalID string) (*domain.Payment, error) {
	args := m.Called(ctx, externalID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Payment), args.Error(1)
}

func (m *MockPaymentRepositoryRead) GetWebhookEvent(ctx context.Context, fedapayEventID string) (*domain.WebhookEvent, error) {
	args := m.Called(ctx, fedapayEventID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.WebhookEvent), args.Error(1)
}
