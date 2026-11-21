package integration

import (
	"context"
	"testing"

	"github.com/Kpeewu/tissi-mah/services/payment-service/fixtures"
	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/domain"
	paymentErrors "github.com/Kpeewu/tissi-mah/services/payment-service/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// RefundRepositoryWrite — CreateRefund
// =============================================================================

func TestRefundWriteRepository_CreateRefund(t *testing.T) {
	ctx := context.Background()
	writeRepo := newTestRefundWriteRepository()
	readRepo := newTestRefundReadRepository()

	t.Run("cree un refund avec succes", func(t *testing.T) {
		cleanupPaymentTables(t, ctx)

		// Inserer un paiement parent
		payment := fixtures.NewTestPayment(fixtures.WithPaymentID("pay-for-refund"))
		require.NoError(t, fixtures.InsertPayment(ctx, testPool, payment))

		refund := fixtures.NewTestRefund(
			fixtures.WithRefundID("ref-create-1"),
			fixtures.WithRefundPaymentID("pay-for-refund"),
			fixtures.WithRefundBookingID("booking-ref-1"),
			fixtures.WithRefundAmount(5500),
			fixtures.WithRefundStatus(domain.RefundStatusCompleted),
		)

		err := writeRepo.CreateRefund(ctx, refund)
		require.NoError(t, err)

		// Verifier en lecture
		result, err := readRepo.GetByID(ctx, "ref-create-1")
		require.NoError(t, err)
		assert.Equal(t, "ref-create-1", result.RefundID)
		assert.Equal(t, 5500, result.RefundAmount)
		assert.Equal(t, domain.RefundStatusCompleted, result.Status)
	})

	t.Run("un seul refund par paiement : le second renvoie ErrorRefundAlreadyProcessed", func(t *testing.T) {
		cleanupPaymentTables(t, ctx)

		payment := fixtures.NewTestPayment(fixtures.WithPaymentID("pay-unique-refund"))
		require.NoError(t, fixtures.InsertPayment(ctx, testPool, payment))

		first := fixtures.NewTestRefund(fixtures.WithRefundPaymentID("pay-unique-refund"))
		require.NoError(t, writeRepo.CreateRefund(ctx, first))

		second := fixtures.NewTestRefund(fixtures.WithRefundPaymentID("pay-unique-refund"))
		err := writeRepo.CreateRefund(ctx, second)

		assert.ErrorIs(t, err, paymentErrors.ErrorRefundAlreadyProcessed)
	})
}

// TestRefundWriteRepository_CreateRefund_AllReasons vérifie que chaque motif du domaine existe
// dans l'enum SQL refund_reason : une constante ajoutée sans migration ALTER TYPE fait échouer
// l'INSERT (cas waypointCancelled).
func TestRefundWriteRepository_CreateRefund_AllReasons(t *testing.T) {
	ctx := context.Background()
	writeRepo := newTestRefundWriteRepository()
	readRepo := newTestRefundReadRepository()

	cleanupPaymentTables(t, ctx)

	for _, reason := range domain.RefundReasons() {
		t.Run(string(reason), func(t *testing.T) {
			// Un paiement par motif : un paiement n'a qu'un remboursement
			paymentID := "pay-reason-" + string(reason)
			payment := fixtures.NewTestPayment(
				fixtures.WithPaymentID(paymentID),
				fixtures.WithPaymentBookingID("bk-reason-"+string(reason)),
			)
			require.NoError(t, fixtures.InsertPayment(ctx, testPool, payment))

			refundID := "ref-reason-" + string(reason)
			refund := fixtures.NewTestRefund(
				fixtures.WithRefundID(refundID),
				fixtures.WithRefundPaymentID(paymentID),
				fixtures.WithRefundReason(reason),
			)

			require.NoError(t, writeRepo.CreateRefund(ctx, refund), "motif absent de l'enum refund_reason ?")

			result, err := readRepo.GetByID(ctx, refundID)
			require.NoError(t, err)
			assert.Equal(t, reason, result.RefundReason)
		})
	}
}

// =============================================================================
// RefundRepositoryRead — GetByID / GetByBookingID / GetByPaymentID
// =============================================================================

func TestRefundReadRepository_GetByID(t *testing.T) {
	ctx := context.Background()
	readRepo := newTestRefundReadRepository()

	t.Run("retourne un refund existant", func(t *testing.T) {
		cleanupPaymentTables(t, ctx)

		payment := fixtures.NewTestPayment(fixtures.WithPaymentID("pay-ref-read"))
		require.NoError(t, fixtures.InsertPayment(ctx, testPool, payment))

		refund := fixtures.NewTestRefund(
			fixtures.WithRefundID("ref-read-1"),
			fixtures.WithRefundPaymentID("pay-ref-read"),
			fixtures.WithRefundReason(domain.RefundReasonCancelledByDriver),
		)
		require.NoError(t, fixtures.InsertRefund(ctx, testPool, refund))

		result, err := readRepo.GetByID(ctx, "ref-read-1")

		require.NoError(t, err)
		assert.Equal(t, "ref-read-1", result.RefundID)
		assert.Equal(t, domain.RefundReasonCancelledByDriver, result.RefundReason)
	})

	t.Run("retourne ErrorRefundNotFound si inexistant", func(t *testing.T) {
		cleanupPaymentTables(t, ctx)

		result, err := readRepo.GetByID(ctx, "ref-inexistant")

		assert.ErrorIs(t, err, paymentErrors.ErrorRefundNotFound)
		assert.Nil(t, result)
	})
}

func TestRefundReadRepository_GetByBookingID(t *testing.T) {
	ctx := context.Background()
	readRepo := newTestRefundReadRepository()

	t.Run("retourne le refund par booking ID", func(t *testing.T) {
		cleanupPaymentTables(t, ctx)

		payment := fixtures.NewTestPayment(fixtures.WithPaymentID("pay-ref-booking"))
		require.NoError(t, fixtures.InsertPayment(ctx, testPool, payment))

		refund := fixtures.NewTestRefund(
			fixtures.WithRefundPaymentID("pay-ref-booking"),
			fixtures.WithRefundBookingID("booking-ref-find"),
		)
		require.NoError(t, fixtures.InsertRefund(ctx, testPool, refund))

		result, err := readRepo.GetByBookingID(ctx, "booking-ref-find")

		require.NoError(t, err)
		assert.Equal(t, "booking-ref-find", result.BookingID)
	})
}

// =============================================================================
// RefundRepositoryWrite — UpdateRefundStatus
// =============================================================================

func TestRefundWriteRepository_UpdateRefundStatus(t *testing.T) {
	ctx := context.Background()
	writeRepo := newTestRefundWriteRepository()
	readRepo := newTestRefundReadRepository()

	t.Run("met a jour le statut du refund", func(t *testing.T) {
		cleanupPaymentTables(t, ctx)

		payment := fixtures.NewTestPayment(fixtures.WithPaymentID("pay-ref-upd"))
		require.NoError(t, fixtures.InsertPayment(ctx, testPool, payment))

		refund := fixtures.NewTestRefund(
			fixtures.WithRefundID("ref-upd-1"),
			fixtures.WithRefundPaymentID("pay-ref-upd"),
			fixtures.WithRefundStatus(domain.RefundStatusPending),
		)
		require.NoError(t, fixtures.InsertRefund(ctx, testPool, refund))

		err := writeRepo.UpdateRefundStatus(ctx, "ref-upd-1", domain.RefundStatusCompleted)
		require.NoError(t, err)

		result, err := readRepo.GetByID(ctx, "ref-upd-1")
		require.NoError(t, err)
		assert.Equal(t, domain.RefundStatusCompleted, result.Status)
	})
}
