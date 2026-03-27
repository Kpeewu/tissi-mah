package mocks

import (
	"context"

	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/payment-service/internal/service/interfaces"
	"github.com/stretchr/testify/mock"
)

type MockPaymentService struct {
	mock.Mock
}

func (m *MockPaymentService) CreatePayment(ctx context.Context, input *serviceInterfaces.CreatePaymentInput) (*serviceInterfaces.CreatePaymentResult, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*serviceInterfaces.CreatePaymentResult), args.Error(1)
}

func (m *MockPaymentService) GetPaymentStatus(ctx context.Context, paymentID string) (*serviceInterfaces.PaymentStatusResult, error) {
	args := m.Called(ctx, paymentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*serviceInterfaces.PaymentStatusResult), args.Error(1)
}

func (m *MockPaymentService) GetPaymentByBooking(ctx context.Context, bookingID string) (*serviceInterfaces.PaymentByBookingResult, error) {
	args := m.Called(ctx, bookingID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*serviceInterfaces.PaymentByBookingResult), args.Error(1)
}

func (m *MockPaymentService) ProcessWebhook(ctx context.Context, input *serviceInterfaces.ProcessWebhookInput) error {
	args := m.Called(ctx, input)
	return args.Error(0)
}

func (m *MockPaymentService) RequestRefund(ctx context.Context, input *serviceInterfaces.RequestRefundInput) (*serviceInterfaces.RequestRefundResult, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*serviceInterfaces.RequestRefundResult), args.Error(1)
}

func (m *MockPaymentService) GetRefundStatus(ctx context.Context, refundID string) (*serviceInterfaces.RefundStatusResult, error) {
	args := m.Called(ctx, refundID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*serviceInterfaces.RefundStatusResult), args.Error(1)
}

func (m *MockPaymentService) ReleasePayment(ctx context.Context, bookingID string) error {
	args := m.Called(ctx, bookingID)
	return args.Error(0)
}

func (m *MockPaymentService) GetPayoutStatus(ctx context.Context, payoutID string) (*serviceInterfaces.PayoutStatusResult, error) {
	args := m.Called(ctx, payoutID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*serviceInterfaces.PayoutStatusResult), args.Error(1)
}

func (m *MockPaymentService) GetDriverPayouts(ctx context.Context, driverID string, pageIndex int) ([]*serviceInterfaces.PayoutPreviewResult, error) {
	args := m.Called(ctx, driverID, pageIndex)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*serviceInterfaces.PayoutPreviewResult), args.Error(1)
}
