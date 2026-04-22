package mocks

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/domain"
	"github.com/stretchr/testify/mock"
)

type MockRefundRepositoryRead struct {
	mock.Mock
}

func (m *MockRefundRepositoryRead) GetByID(ctx context.Context, refundID string) (*domain.Refund, error) {
	args := m.Called(ctx, refundID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Refund), args.Error(1)
}

func (m *MockRefundRepositoryRead) GetByBookingID(ctx context.Context, bookingID string) (*domain.Refund, error) {
	args := m.Called(ctx, bookingID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Refund), args.Error(1)
}

func (m *MockRefundRepositoryRead) GetByPaymentID(ctx context.Context, paymentID string) (*domain.Refund, error) {
	args := m.Called(ctx, paymentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Refund), args.Error(1)
}

func (m *MockRefundRepositoryRead) GetPendingRefundsForPayout(ctx context.Context, limit int) ([]*domain.Refund, error) {
	args := m.Called(ctx, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Refund), args.Error(1)
}

type MockRefundRepositoryWrite struct {
	mock.Mock
}

func (m *MockRefundRepositoryWrite) CreateRefund(ctx context.Context, refund *domain.Refund) error {
	args := m.Called(ctx, refund)
	return args.Error(0)
}

func (m *MockRefundRepositoryWrite) UpdateRefundStatus(ctx context.Context, refundID string, status domain.RefundStatus) error {
	args := m.Called(ctx, refundID, status)
	return args.Error(0)
}

func (m *MockRefundRepositoryWrite) MarkRefundProcessing(ctx context.Context, refundID string) error {
	args := m.Called(ctx, refundID)
	return args.Error(0)
}

func (m *MockRefundRepositoryWrite) MarkRefundCompleted(ctx context.Context, refundID string, paymentProviderReference string) error {
	args := m.Called(ctx, refundID, paymentProviderReference)
	return args.Error(0)
}

func (m *MockRefundRepositoryWrite) MarkRefundFailed(ctx context.Context, refundID string, reason string, incrementRetry bool) error {
	args := m.Called(ctx, refundID, reason, incrementRetry)
	return args.Error(0)
}
