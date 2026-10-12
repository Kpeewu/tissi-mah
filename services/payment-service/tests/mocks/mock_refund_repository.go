package mocks

import (
	"context"
	"time"

	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/domain"
	paymentErrors "github.com/Kpeewu/tissi-mah/services/payment-service/pkg/errors"
	"github.com/stretchr/testify/mock"
)

type MockRefundRepositoryRead struct {
	mock.Mock
}

func (m *MockRefundRepositoryRead) hasExpectation(method string) bool {
	for _, call := range m.ExpectedCalls {
		if call.Method == method {
			return true
		}
	}
	return false
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

// GetByPaymentID retourne ErrorRefundNotFound sans expectation : createRefundForPayment le consulte
// à chaque demande (idempotence), les tests n'ont à le configurer que pour un remboursement existant.
func (m *MockRefundRepositoryRead) GetByPaymentID(ctx context.Context, paymentID string) (*domain.Refund, error) {
	if !m.hasExpectation("GetByPaymentID") {
		return nil, paymentErrors.ErrorRefundNotFound
	}
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

// GetByProviderReference retourne ErrorRefundNotFound sans expectation déclarée (webhook payout chauffeur).
func (m *MockRefundRepositoryRead) GetByProviderReference(ctx context.Context, providerReference string) (*domain.Refund, error) {
	if !m.hasExpectation("GetByProviderReference") {
		return nil, paymentErrors.ErrorRefundNotFound
	}
	args := m.Called(ctx, providerReference)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Refund), args.Error(1)
}

func (m *MockRefundRepositoryRead) GetProcessingRefundsToReconcile(ctx context.Context, updatedBefore time.Time, limit int) ([]*domain.Refund, error) {
	args := m.Called(ctx, updatedBefore, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Refund), args.Error(1)
}

func (m *MockRefundRepositoryWrite) SetRefundProviderReference(ctx context.Context, refundID string, paymentProviderReference string) error {
	args := m.Called(ctx, refundID, paymentProviderReference)
	return args.Error(0)
}

func (m *MockRefundRepositoryWrite) MarkRefundPayoutFailed(ctx context.Context, refundID string, reason string) error {
	args := m.Called(ctx, refundID, reason)
	return args.Error(0)
}
