package handler_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	paymentGrpc "github.com/Kpeewu/tissi-mah/services/payment-service/internal/grpc"
	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/middleware"
	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/payment-service/internal/service/interfaces"
	paymentErrors "github.com/Kpeewu/tissi-mah/services/payment-service/pkg/errors"
	paymentpb "github.com/Kpeewu/tissi-mah/services/payment-service/proto/gen"
	"github.com/Kpeewu/tissi-mah/services/payment-service/tests/mocks"
)

func supportCtx() context.Context {
	return context.WithValue(context.Background(), middleware.SupportUIDKey, "support-1")
}

func TestGetPaymentHistoryHandler(t *testing.T) {
	t.Run("Unauthenticated sans support UID", func(t *testing.T) {
		mockSvc := new(mocks.MockPaymentService)
		h := paymentGrpc.NewPaymentHandler(mockSvc, zap.NewNop())

		_, err := h.GetPaymentHistory(context.Background(), &paymentpb.GetPaymentHistoryRequest{PaymentId: "pay-1"})

		st, _ := status.FromError(err)
		require.Equal(t, codes.Unauthenticated, st.Code())
		mockSvc.AssertNotCalled(t, "GetPaymentHistory", mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("InvalidArgument sans identifiant", func(t *testing.T) {
		mockSvc := new(mocks.MockPaymentService)
		mockSvc.On("GetPaymentHistory", mock.Anything, "", "").Return(nil, paymentErrors.ErrorInvalidInput)
		h := paymentGrpc.NewPaymentHandler(mockSvc, zap.NewNop())

		resp, err := h.GetPaymentHistory(supportCtx(), &paymentpb.GetPaymentHistoryRequest{})

		st, _ := status.FromError(err)
		require.Equal(t, codes.InvalidArgument, st.Code())
		assert.Equal(t, paymentErrors.ErrorInvalidInput.Error(), resp.ErrorMessage)
	})

	t.Run("succès - mappe les entrées", func(t *testing.T) {
		mockSvc := new(mocks.MockPaymentService)
		mockSvc.On("GetPaymentHistory", mock.Anything, "", "bk-1").Return([]*serviceInterfaces.PaymentHistoryEntryResult{
			{HistoryID: "h-1", PaymentID: "pay-1", Status: "held", ProviderStatus: "approved",
				InitiatedBy: "system", Source: "fedapay_webhook", OccurredAt: "2026-10-12T08:30:00Z", Notes: "ok"},
		}, nil)
		h := paymentGrpc.NewPaymentHandler(mockSvc, zap.NewNop())

		resp, err := h.GetPaymentHistory(supportCtx(), &paymentpb.GetPaymentHistoryRequest{BookingId: "bk-1"})

		require.NoError(t, err)
		require.Len(t, resp.Entries, 1)
		e := resp.Entries[0]
		assert.Equal(t, "h-1", e.HistoryId)
		assert.Equal(t, "pay-1", e.PaymentId)
		assert.Equal(t, "held", e.Status)
		assert.Equal(t, "approved", e.ProviderStatus)
		assert.Equal(t, "system", e.InitiatedBy)
		assert.Equal(t, "fedapay_webhook", e.Source)
		assert.Equal(t, "2026-10-12T08:30:00Z", e.OccurredAt)
		assert.Equal(t, "ok", e.Notes)
	})
}

func TestGetRefundHistoryHandler(t *testing.T) {
	t.Run("Unauthenticated sans support UID", func(t *testing.T) {
		mockSvc := new(mocks.MockPaymentService)
		h := paymentGrpc.NewPaymentHandler(mockSvc, zap.NewNop())

		_, err := h.GetRefundHistory(context.Background(), &paymentpb.GetRefundHistoryRequest{RefundId: "ref-1"})

		st, _ := status.FromError(err)
		require.Equal(t, codes.Unauthenticated, st.Code())
	})

	t.Run("succès - mappe les entrées", func(t *testing.T) {
		mockSvc := new(mocks.MockPaymentService)
		mockSvc.On("GetRefundHistory", mock.Anything, "ref-1", "").Return([]*serviceInterfaces.RefundHistoryEntryResult{
			{HistoryID: "h-1", RefundID: "ref-1", Status: "retry_scheduled", RetryCount: 2,
				InitiatedBy: "system", Source: "refund_worker", OccurredAt: "2026-10-12T01:00:00Z", Notes: "fedapay: down"},
		}, nil)
		h := paymentGrpc.NewPaymentHandler(mockSvc, zap.NewNop())

		resp, err := h.GetRefundHistory(supportCtx(), &paymentpb.GetRefundHistoryRequest{RefundId: "ref-1"})

		require.NoError(t, err)
		require.Len(t, resp.Entries, 1)
		assert.Equal(t, "retry_scheduled", resp.Entries[0].Status)
		assert.Equal(t, int32(2), resp.Entries[0].RetryCount)
		assert.Equal(t, "fedapay: down", resp.Entries[0].Notes)
	})
}

func TestGetPayoutHistoryHandler(t *testing.T) {
	t.Run("Unauthenticated sans support UID", func(t *testing.T) {
		mockSvc := new(mocks.MockPaymentService)
		h := paymentGrpc.NewPaymentHandler(mockSvc, zap.NewNop())

		_, err := h.GetPayoutHistory(context.Background(), &paymentpb.GetPayoutHistoryRequest{PayoutId: "po-1"})

		st, _ := status.FromError(err)
		require.Equal(t, codes.Unauthenticated, st.Code())
	})

	t.Run("succès - mappe l'agent support", func(t *testing.T) {
		mockSvc := new(mocks.MockPaymentService)
		mockSvc.On("GetPayoutHistory", mock.Anything, "po-1").Return([]*serviceInterfaces.PayoutHistoryEntryResult{
			{HistoryID: "h-1", PayoutID: "po-1", Status: "launched_by_support", InitiatedBy: "support",
				SupportUserID: "support-1", SupportFirstName: "Awa", SupportLastName: "Diallo",
				OccurredAt: "2026-10-12T02:00:00Z"},
		}, nil)
		h := paymentGrpc.NewPaymentHandler(mockSvc, zap.NewNop())

		resp, err := h.GetPayoutHistory(supportCtx(), &paymentpb.GetPayoutHistoryRequest{PayoutId: "po-1"})

		require.NoError(t, err)
		require.Len(t, resp.Entries, 1)
		assert.Equal(t, "support-1", resp.Entries[0].SupportUserId)
		assert.Equal(t, "Awa", resp.Entries[0].SupportFirstName)
		assert.Equal(t, "Diallo", resp.Entries[0].SupportLastName)
	})
}
