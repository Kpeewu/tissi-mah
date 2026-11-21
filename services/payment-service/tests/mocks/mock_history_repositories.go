package mocks

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/domain"
	"github.com/stretchr/testify/mock"
)

// =============================================================================
// MockPayoutHistoryRepository
// =============================================================================

type MockPayoutHistoryRepository struct {
	mock.Mock
}

func (m *MockPayoutHistoryRepository) CreateHistoryEntry(ctx context.Context, entry *domain.PayoutStatusHistory) error {
	args := m.Called(ctx, entry)
	return args.Error(0)
}

func (m *MockPayoutHistoryRepository) ListByPayoutID(ctx context.Context, payoutID string) ([]*domain.PayoutStatusHistory, error) {
	args := m.Called(ctx, payoutID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.PayoutStatusHistory), args.Error(1)
}

// =============================================================================
// MockPaymentHistoryRepository
// =============================================================================

type MockPaymentHistoryRepository struct {
	mock.Mock
}

func (m *MockPaymentHistoryRepository) CreateHistoryEntry(ctx context.Context, entry *domain.PaymentStatusHistory) error {
	args := m.Called(ctx, entry)
	return args.Error(0)
}

func (m *MockPaymentHistoryRepository) ListByPaymentID(ctx context.Context, paymentID string) ([]*domain.PaymentStatusHistory, error) {
	args := m.Called(ctx, paymentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.PaymentStatusHistory), args.Error(1)
}

func (m *MockPaymentHistoryRepository) ListByBookingID(ctx context.Context, bookingID string) ([]*domain.PaymentStatusHistory, error) {
	args := m.Called(ctx, bookingID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.PaymentStatusHistory), args.Error(1)
}

// =============================================================================
// MockRefundHistoryRepository
// =============================================================================

type MockRefundHistoryRepository struct {
	mock.Mock
}

func (m *MockRefundHistoryRepository) CreateHistoryEntry(ctx context.Context, entry *domain.RefundStatusHistory) error {
	args := m.Called(ctx, entry)
	return args.Error(0)
}

func (m *MockRefundHistoryRepository) ListByRefundID(ctx context.Context, refundID string) ([]*domain.RefundStatusHistory, error) {
	args := m.Called(ctx, refundID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.RefundStatusHistory), args.Error(1)
}

func (m *MockRefundHistoryRepository) ListByBookingID(ctx context.Context, bookingID string) ([]*domain.RefundStatusHistory, error) {
	args := m.Called(ctx, bookingID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.RefundStatusHistory), args.Error(1)
}
