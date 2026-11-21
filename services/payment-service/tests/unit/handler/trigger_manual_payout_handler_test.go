package handler_test

import (
	"context"
	"testing"

	paymentGrpc "github.com/Kpeewu/tissi-mah/services/payment-service/internal/grpc"
	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/middleware"
	paymentErrors "github.com/Kpeewu/tissi-mah/services/payment-service/pkg/errors"
	paymentpb "github.com/Kpeewu/tissi-mah/services/payment-service/proto/gen"
	"github.com/Kpeewu/tissi-mah/services/payment-service/tests/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestTriggerManualPayoutHandler(t *testing.T) {
	req := &paymentpb.TriggerManualPayoutRequest{TripId: "trip-1"}

	t.Run("Unauthenticated si SupportUIDKey absent du contexte", func(t *testing.T) {
		mockSvc := new(mocks.MockPaymentService)
		h := paymentGrpc.NewPaymentHandler(mockSvc, zap.NewNop())

		resp, err := h.TriggerManualPayout(context.Background(), req)

		require.Error(t, err)
		require.False(t, resp.Success)
		st, _ := status.FromError(err)
		require.Equal(t, codes.Unauthenticated, st.Code())
		mockSvc.AssertNotCalled(t, "TriggerManualPayout")
	})

	t.Run("Unauthenticated si SupportUIDKey vide", func(t *testing.T) {
		mockSvc := new(mocks.MockPaymentService)
		h := paymentGrpc.NewPaymentHandler(mockSvc, zap.NewNop())
		ctx := context.WithValue(context.Background(), middleware.SupportUIDKey, "")

		resp, err := h.TriggerManualPayout(ctx, req)

		require.Error(t, err)
		require.False(t, resp.Success)
		st, _ := status.FromError(err)
		require.Equal(t, codes.Unauthenticated, st.Code())
		mockSvc.AssertNotCalled(t, "TriggerManualPayout")
	})

	t.Run("succès - renvoie NetAmount", func(t *testing.T) {
		mockSvc := new(mocks.MockPaymentService)
		mockSvc.On("TriggerManualPayout", mock.Anything, "trip-1", "support-1").Return(4500, nil)
		h := paymentGrpc.NewPaymentHandler(mockSvc, zap.NewNop())
		ctx := context.WithValue(context.Background(), middleware.SupportUIDKey, "support-1")

		resp, err := h.TriggerManualPayout(ctx, req)

		require.NoError(t, err)
		require.True(t, resp.Success)
		require.Equal(t, int32(4500), resp.NetAmount)
		mockSvc.AssertExpectations(t)
	})

	t.Run("ErrorTripNotReadyForPayout mappé en FailedPrecondition", func(t *testing.T) {
		mockSvc := new(mocks.MockPaymentService)
		mockSvc.On("TriggerManualPayout", mock.Anything, "trip-1", "support-1").
			Return(0, paymentErrors.ErrorTripNotReadyForPayout)
		h := paymentGrpc.NewPaymentHandler(mockSvc, zap.NewNop())
		ctx := context.WithValue(context.Background(), middleware.SupportUIDKey, "support-1")

		resp, err := h.TriggerManualPayout(ctx, req)

		require.Error(t, err)
		require.False(t, resp.Success)
		st, _ := status.FromError(err)
		require.Equal(t, codes.FailedPrecondition, st.Code())
	})

	t.Run("ErrorUnauthorized mappé en PermissionDenied", func(t *testing.T) {
		mockSvc := new(mocks.MockPaymentService)
		mockSvc.On("TriggerManualPayout", mock.Anything, "trip-1", "support-1").
			Return(0, paymentErrors.ErrorUnauthorized)
		h := paymentGrpc.NewPaymentHandler(mockSvc, zap.NewNop())
		ctx := context.WithValue(context.Background(), middleware.SupportUIDKey, "support-1")

		resp, err := h.TriggerManualPayout(ctx, req)

		require.Error(t, err)
		require.False(t, resp.Success)
		st, _ := status.FromError(err)
		require.Equal(t, codes.PermissionDenied, st.Code())
	})

	t.Run("ErrorSupportServiceUnavailable mappé en Unavailable", func(t *testing.T) {
		mockSvc := new(mocks.MockPaymentService)
		mockSvc.On("TriggerManualPayout", mock.Anything, "trip-1", "support-1").
			Return(0, paymentErrors.ErrorSupportServiceUnavailable)
		h := paymentGrpc.NewPaymentHandler(mockSvc, zap.NewNop())
		ctx := context.WithValue(context.Background(), middleware.SupportUIDKey, "support-1")

		resp, err := h.TriggerManualPayout(ctx, req)

		require.Error(t, err)
		require.False(t, resp.Success)
		st, _ := status.FromError(err)
		require.Equal(t, codes.Unavailable, st.Code())
	})

	t.Run("ErrorInvalidInput mappé en InvalidArgument", func(t *testing.T) {
		mockSvc := new(mocks.MockPaymentService)
		mockSvc.On("TriggerManualPayout", mock.Anything, "trip-1", "support-1").
			Return(0, paymentErrors.ErrorInvalidInput)
		h := paymentGrpc.NewPaymentHandler(mockSvc, zap.NewNop())
		ctx := context.WithValue(context.Background(), middleware.SupportUIDKey, "support-1")

		resp, err := h.TriggerManualPayout(ctx, req)

		require.Error(t, err)
		require.False(t, resp.Success)
		st, _ := status.FromError(err)
		require.Equal(t, codes.InvalidArgument, st.Code())
	})
}
