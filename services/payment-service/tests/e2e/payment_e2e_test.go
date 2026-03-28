package e2e

import (
	"testing"
	"time"

	"github.com/Kpeewu/tissi-mah/services/payment-service/fixtures"
	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/domain"
	paymentpb "github.com/Kpeewu/tissi-mah/services/payment-service/proto/gen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// =============================================================================
// Health E2E
// =============================================================================

func TestE2E_Health(t *testing.T) {
	ctx := ctxWithUID("e2e-test-user")

	resp, err := grpcClient.Health(ctx, &paymentpb.HealthRequest{})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "ok", resp.Status)
	assert.NotEmpty(t, resp.Version)
	assert.Greater(t, resp.Timestamp, int64(0))
}

// =============================================================================
// GetPaymentStatus E2E
// =============================================================================

func TestE2E_GetPaymentStatus(t *testing.T) {
	ctx := ctxWithUID("e2e-test-user")

	t.Run("succes - retourne le statut d un paiement existant", func(t *testing.T) {
		cleanupPaymentTables(t, ctx)

		payment := fixtures.NewTestPayment(
			fixtures.WithPaymentID("e2e-pay-1"),
			fixtures.WithPaymentBookingID("e2e-booking-1"),
			fixtures.WithPaymentAmount(7500),
			fixtures.WithPaymentStatus(domain.PaymentStatusHeld),
		)
		require.NoError(t, fixtures.InsertPayment(ctx, testPool, payment))

		resp, err := grpcClient.GetPaymentStatus(ctx, &paymentpb.GetPaymentStatusRequest{
			PaymentId: "e2e-pay-1",
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, "e2e-pay-1", resp.PaymentId)
		assert.Equal(t, "e2e-booking-1", resp.BookingId)
		assert.Equal(t, int32(7500), resp.Amount)
		assert.Equal(t, "held", resp.Status)
	})

	t.Run("erreur - paiement inexistant", func(t *testing.T) {
		cleanupPaymentTables(t, ctx)

		_, err := grpcClient.GetPaymentStatus(ctx, &paymentpb.GetPaymentStatusRequest{
			PaymentId: "e2e-pay-inexistant",
		})

		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.NotFound, st.Code())
	})
}

// =============================================================================
// GetPaymentByBooking E2E
// =============================================================================

func TestE2E_GetPaymentByBooking(t *testing.T) {
	ctx := ctxWithUID("e2e-test-user")

	t.Run("succes - retourne le paiement par booking", func(t *testing.T) {
		cleanupPaymentTables(t, ctx)

		payment := fixtures.NewTestPayment(
			fixtures.WithPaymentBookingID("e2e-booking-find"),
			fixtures.WithPaymentAmount(4000),
		)
		require.NoError(t, fixtures.InsertPayment(ctx, testPool, payment))

		resp, err := grpcClient.GetPaymentByBooking(ctx, &paymentpb.GetPaymentByBookingRequest{
			BookingId: "e2e-booking-find",
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, "e2e-booking-find", resp.BookingId)
		assert.Equal(t, int32(4000), resp.Amount)
	})
}

// =============================================================================
// ReleasePayment E2E
// =============================================================================

func TestE2E_ReleasePayment(t *testing.T) {
	ctx := ctxWithUID("e2e-test-user")

	t.Run("succes - libere un paiement held", func(t *testing.T) {
		cleanupPaymentTables(t, ctx)

		now := time.Now().UTC()
		payment := fixtures.NewTestPayment(
			fixtures.WithPaymentID("e2e-pay-release"),
			fixtures.WithPaymentBookingID("e2e-booking-release"),
			fixtures.WithPaymentStatus(domain.PaymentStatusHeld),
			fixtures.WithPaymentCompletedAt(now),
		)
		require.NoError(t, fixtures.InsertPayment(ctx, testPool, payment))

		resp, err := grpcClient.ReleasePayment(ctx, &paymentpb.ReleasePaymentRequest{
			BookingId: "e2e-booking-release",
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.True(t, resp.Success)

		// Verifier que le statut est maintenant released
		statusResp, err := grpcClient.GetPaymentStatus(ctx, &paymentpb.GetPaymentStatusRequest{
			PaymentId: "e2e-pay-release",
		})
		require.NoError(t, err)
		assert.Equal(t, "released", statusResp.Status)
	})

	t.Run("erreur - paiement non held", func(t *testing.T) {
		cleanupPaymentTables(t, ctx)

		payment := fixtures.NewTestPayment(
			fixtures.WithPaymentBookingID("e2e-booking-pending"),
			fixtures.WithPaymentStatus(domain.PaymentStatusPending),
		)
		require.NoError(t, fixtures.InsertPayment(ctx, testPool, payment))

		_, err := grpcClient.ReleasePayment(ctx, &paymentpb.ReleasePaymentRequest{
			BookingId: "e2e-booking-pending",
		})

		require.Error(t, err)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.FailedPrecondition, st.Code())
	})
}

// =============================================================================
// RequestRefund E2E
// =============================================================================

func TestE2E_RequestRefund(t *testing.T) {
	ctx := ctxWithUID("e2e-test-user")

	t.Run("succes - remboursement complet annulation chauffeur", func(t *testing.T) {
		cleanupPaymentTables(t, ctx)

		now := time.Now().UTC()
		payment := fixtures.NewTestPayment(
			fixtures.WithPaymentBookingID("e2e-booking-refund"),
			fixtures.WithPaymentAmount(5000),
			fixtures.WithPaymentStatus(domain.PaymentStatusHeld),
			fixtures.WithPaymentCompletedAt(now),
		)
		require.NoError(t, fixtures.InsertPayment(ctx, testPool, payment))

		departure := time.Now().UTC().Add(48 * time.Hour)
		resp, err := grpcClient.RequestRefund(ctx, &paymentpb.RequestRefundRequest{
			BookingId:         "e2e-booking-refund",
			RefundReason:      "cancelledByDriver",
			OriginalAmount:    5000,
			ServiceFee:        500,
			DepartureDatetime: departure.Format(time.RFC3339),
			CancelledAt:       time.Now().UTC().Format(time.RFC3339),
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.NotEmpty(t, resp.RefundId)
		assert.Equal(t, "completed", resp.Status)
		assert.Equal(t, int32(5500), resp.RefundAmount) // 5000 + 500 (frais rembourses)
	})

	t.Run("erreur - paiement non held/released", func(t *testing.T) {
		cleanupPaymentTables(t, ctx)

		payment := fixtures.NewTestPayment(
			fixtures.WithPaymentBookingID("e2e-booking-refund-fail"),
			fixtures.WithPaymentStatus(domain.PaymentStatusPending),
		)
		require.NoError(t, fixtures.InsertPayment(ctx, testPool, payment))

		_, err := grpcClient.RequestRefund(ctx, &paymentpb.RequestRefundRequest{
			BookingId:         "e2e-booking-refund-fail",
			RefundReason:      "cancelledByDriver",
			OriginalAmount:    5000,
			ServiceFee:        500,
			DepartureDatetime: time.Now().UTC().Add(48 * time.Hour).Format(time.RFC3339),
			CancelledAt:       time.Now().UTC().Format(time.RFC3339),
		})

		require.Error(t, err)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.FailedPrecondition, st.Code())
	})
}

// =============================================================================
// GetRefundStatus E2E
// =============================================================================

func TestE2E_GetRefundStatus(t *testing.T) {
	ctx := ctxWithUID("e2e-test-user")

	t.Run("succes - retourne le statut du remboursement", func(t *testing.T) {
		cleanupPaymentTables(t, ctx)

		payment := fixtures.NewTestPayment(fixtures.WithPaymentID("e2e-pay-for-ref"))
		require.NoError(t, fixtures.InsertPayment(ctx, testPool, payment))

		refund := fixtures.NewTestRefund(
			fixtures.WithRefundID("e2e-ref-1"),
			fixtures.WithRefundPaymentID("e2e-pay-for-ref"),
			fixtures.WithRefundBookingID("e2e-booking-ref"),
			fixtures.WithRefundAmount(5500),
			fixtures.WithRefundStatus(domain.RefundStatusCompleted),
		)
		require.NoError(t, fixtures.InsertRefund(ctx, testPool, refund))

		resp, err := grpcClient.GetRefundStatus(ctx, &paymentpb.GetRefundStatusRequest{
			RefundId: "e2e-ref-1",
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, "e2e-ref-1", resp.RefundId)
		assert.Equal(t, "completed", resp.Status)
		assert.Equal(t, int32(5500), resp.RefundAmount)
	})

	t.Run("erreur - remboursement inexistant", func(t *testing.T) {
		cleanupPaymentTables(t, ctx)

		_, err := grpcClient.GetRefundStatus(ctx, &paymentpb.GetRefundStatusRequest{
			RefundId: "e2e-ref-inexistant",
		})

		require.Error(t, err)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.NotFound, st.Code())
	})
}

// =============================================================================
// Scenario complet E2E : paiement → release → refund
// =============================================================================

func TestE2E_FullPaymentLifecycle(t *testing.T) {
	ctx := ctxWithUID("e2e-test-user")
	cleanupPaymentTables(t, ctx)

	// 1. Inserer un paiement held (simule un webhook FedaPay reussi)
	now := time.Now().UTC()
	payment := fixtures.NewTestPayment(
		fixtures.WithPaymentID("e2e-lifecycle-pay"),
		fixtures.WithPaymentBookingID("e2e-lifecycle-booking"),
		fixtures.WithPaymentAmount(6000),
		fixtures.WithPaymentStatus(domain.PaymentStatusHeld),
		fixtures.WithPaymentCompletedAt(now),
	)
	require.NoError(t, fixtures.InsertPayment(ctx, testPool, payment))

	// 2. Verifier le statut → held
	statusResp, err := grpcClient.GetPaymentStatus(ctx, &paymentpb.GetPaymentStatusRequest{
		PaymentId: "e2e-lifecycle-pay",
	})
	require.NoError(t, err)
	assert.Equal(t, "held", statusResp.Status)

	// 3. Liberer le paiement → released
	releaseResp, err := grpcClient.ReleasePayment(ctx, &paymentpb.ReleasePaymentRequest{
		BookingId: "e2e-lifecycle-booking",
	})
	require.NoError(t, err)
	assert.True(t, releaseResp.Success)

	// 4. Verifier le statut → released
	statusResp, err = grpcClient.GetPaymentStatus(ctx, &paymentpb.GetPaymentStatusRequest{
		PaymentId: "e2e-lifecycle-pay",
	})
	require.NoError(t, err)
	assert.Equal(t, "released", statusResp.Status)

	// 5. Demander un remboursement sur le paiement released
	departure := time.Now().UTC().Add(48 * time.Hour)
	refundResp, err := grpcClient.RequestRefund(ctx, &paymentpb.RequestRefundRequest{
		BookingId:         "e2e-lifecycle-booking",
		RefundReason:      "cancelledByDriver",
		OriginalAmount:    6000,
		ServiceFee:        600,
		DepartureDatetime: departure.Format(time.RFC3339),
		CancelledAt:       time.Now().UTC().Format(time.RFC3339),
	})
	require.NoError(t, err)
	assert.Equal(t, "completed", refundResp.Status)
	assert.Equal(t, int32(6600), refundResp.RefundAmount) // 6000 + 600

	// 6. Verifier le statut du paiement → refunded
	statusResp, err = grpcClient.GetPaymentStatus(ctx, &paymentpb.GetPaymentStatusRequest{
		PaymentId: "e2e-lifecycle-pay",
	})
	require.NoError(t, err)
	assert.Equal(t, "refunded", statusResp.Status)

	// 7. Verifier le refund via GetRefundStatus
	refundStatusResp, err := grpcClient.GetRefundStatus(ctx, &paymentpb.GetRefundStatusRequest{
		RefundId: refundResp.RefundId,
	})
	require.NoError(t, err)
	assert.Equal(t, "completed", refundStatusResp.Status)
	assert.Equal(t, int32(6600), refundStatusResp.AmountToPassenger)
	assert.Equal(t, int32(0), refundStatusResp.AmountToDriver)
}
