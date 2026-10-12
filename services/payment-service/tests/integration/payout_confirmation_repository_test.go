package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Kpeewu/tissi-mah/services/payment-service/fixtures"
	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/domain"
	paymentErrors "github.com/Kpeewu/tissi-mah/services/payment-service/pkg/errors"
)

// seedProcessingRefund crée un paiement remboursé et son refund processing.
func seedProcessingRefund(ctx context.Context, t *testing.T, paymentID, refundID string) {
	t.Helper()
	require.NoError(t, newTestPaymentWriteRepository().CreatePayment(ctx, fixtures.NewTestPayment(
		fixtures.WithPaymentID(paymentID), fixtures.WithPaymentStatus(domain.PaymentStatusRefunded),
	)))
	require.NoError(t, newTestRefundWriteRepository().CreateRefund(ctx, fixtures.NewTestRefund(
		fixtures.WithRefundID(refundID), fixtures.WithRefundPaymentID(paymentID),
		fixtures.WithRefundStatus(domain.RefundStatusProcessing),
	)))
}

func TestRefundPayoutConfirmationRepository(t *testing.T) {
	ctx := context.Background()
	readRepo := newTestRefundReadRepository()
	writeRepo := newTestRefundWriteRepository()

	t.Run("référence FedaPay enregistrée puis retrouvée, confirmation unique", func(t *testing.T) {
		cleanupPaymentTables(t, ctx)
		seedProcessingRefund(ctx, t, "pay-rc-1", "ref-rc-1")

		require.NoError(t, writeRepo.SetRefundProviderReference(ctx, "ref-rc-1", "777"))
		found, err := readRepo.GetByProviderReference(ctx, "777")
		require.NoError(t, err)
		assert.Equal(t, "ref-rc-1", found.RefundID)

		require.NoError(t, writeRepo.MarkRefundCompleted(ctx, "ref-rc-1", "777"))
		assert.ErrorIs(t, writeRepo.MarkRefundCompleted(ctx, "ref-rc-1", "777"), paymentErrors.ErrorRefundAlreadyProcessed)
		assert.ErrorIs(t, writeRepo.SetRefundProviderReference(ctx, "ref-rc-1", "778"), paymentErrors.ErrorRefundAlreadyProcessed)

		refund, err := readRepo.GetByID(ctx, "ref-rc-1")
		require.NoError(t, err)
		assert.Equal(t, domain.RefundStatusCompleted, refund.Status)
		assert.NotNil(t, refund.CompletedAt)
	})

	t.Run("échec FedaPay : processing → pending, retry_count incrémenté une seule fois", func(t *testing.T) {
		cleanupPaymentTables(t, ctx)
		seedProcessingRefund(ctx, t, "pay-rc-2", "ref-rc-2")

		require.NoError(t, writeRepo.MarkRefundPayoutFailed(ctx, "ref-rc-2", "fedapay payout failed: x"))
		assert.ErrorIs(t, writeRepo.MarkRefundPayoutFailed(ctx, "ref-rc-2", "again"), paymentErrors.ErrorRefundAlreadyProcessed)

		refund, err := readRepo.GetByID(ctx, "ref-rc-2")
		require.NoError(t, err)
		assert.Equal(t, domain.RefundStatusPending, refund.Status)
		assert.Equal(t, int16(1), refund.RetryCount)
		require.NotNil(t, refund.FailureReason)
		assert.Equal(t, "fedapay payout failed: x", *refund.FailureReason)
	})

	t.Run("réconciliation : seulement les processing avec référence, avant le délai", func(t *testing.T) {
		cleanupPaymentTables(t, ctx)
		seedProcessingRefund(ctx, t, "pay-rc-3", "ref-with-ref")
		seedProcessingRefund(ctx, t, "pay-rc-4", "ref-without-ref")
		seedProcessingRefund(ctx, t, "pay-rc-5", "ref-completed")
		require.NoError(t, writeRepo.SetRefundProviderReference(ctx, "ref-with-ref", "801"))
		require.NoError(t, writeRepo.SetRefundProviderReference(ctx, "ref-completed", "802"))
		require.NoError(t, writeRepo.MarkRefundCompleted(ctx, "ref-completed", "802"))

		refunds, err := readRepo.GetProcessingRefundsToReconcile(ctx, time.Now().UTC().Add(time.Minute), 50)
		require.NoError(t, err)
		require.Len(t, refunds, 1)
		assert.Equal(t, "ref-with-ref", refunds[0].RefundID)

		recent, err := readRepo.GetProcessingRefundsToReconcile(ctx, time.Now().UTC().Add(-time.Hour), 50)
		require.NoError(t, err)
		assert.Empty(t, recent, "un virement démarré récemment attend le webhook")
	})
}

func TestDriverPayoutConfirmationRepository(t *testing.T) {
	ctx := context.Background()
	paymentWriteRepo := newTestPaymentWriteRepository()
	payoutReadRepo := newTestPayoutReadRepository()
	payoutWriteRepo := newTestPayoutWriteRepository()

	seedPayout := func(t *testing.T, payoutID, tripID, providerRef string, status domain.PayoutStatus) {
		t.Helper()
		payout := fixtures.NewTestPayout(
			fixtures.WithPayoutID(payoutID), fixtures.WithPayoutTripID(tripID), fixtures.WithPayoutStatus(status),
		)
		payout.PaymentProviderReference = providerRef
		require.NoError(t, payoutWriteRepo.CreatePayout(ctx, payout))
	}
	paymentStatus := func(t *testing.T, paymentID string) string {
		t.Helper()
		var status string
		require.NoError(t, testPool.QueryRow(ctx, `SELECT status FROM payments WHERE payment_id = $1`, paymentID).Scan(&status))
		return status
	}

	t.Run("confirmation unique", func(t *testing.T) {
		cleanupPaymentTables(t, ctx)
		seedPayout(t, "po-c-1", "trip-c-1", "901", domain.PayoutStatusProcessing)

		require.NoError(t, payoutWriteRepo.ConfirmPayoutCompleted(ctx, "po-c-1"))
		assert.ErrorIs(t, payoutWriteRepo.ConfirmPayoutCompleted(ctx, "po-c-1"), paymentErrors.ErrorPayoutAlreadyProcessed)

		payout, err := payoutReadRepo.GetByID(ctx, "po-c-1")
		require.NoError(t, err)
		assert.Equal(t, domain.PayoutStatusCompleted, payout.Status)
		assert.NotNil(t, payout.CompletedAt)
	})

	t.Run("échec : payout failed et paiements du trajet remis en released, atomiquement", func(t *testing.T) {
		cleanupPaymentTables(t, ctx)
		seedPayout(t, "po-c-2", "trip-c-2", "902", domain.PayoutStatusProcessing)
		for _, p := range []struct{ id, trip string }{{"pay-c-1", "trip-c-2"}, {"pay-c-2", "trip-c-2"}, {"pay-c-other", "trip-c-other"}} {
			require.NoError(t, paymentWriteRepo.CreatePayment(ctx, fixtures.NewTestPayment(
				fixtures.WithPaymentID(p.id), fixtures.WithPaymentTripID(p.trip), fixtures.WithPaymentStatus(domain.PaymentStatusPaidOut),
			)))
		}

		paymentIDs, err := payoutWriteRepo.FailProcessingPayout(ctx, "po-c-2", "trip-c-2", "fedapay payout failed: x")
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"pay-c-1", "pay-c-2"}, paymentIDs)
		assert.Equal(t, "released", paymentStatus(t, "pay-c-1"))
		assert.Equal(t, "paidOut", paymentStatus(t, "pay-c-other"))

		payout, err := payoutReadRepo.GetByID(ctx, "po-c-2")
		require.NoError(t, err)
		assert.Equal(t, domain.PayoutStatusFailed, payout.Status)

		_, err = payoutWriteRepo.FailProcessingPayout(ctx, "po-c-2", "trip-c-2", "again")
		assert.ErrorIs(t, err, paymentErrors.ErrorPayoutAlreadyProcessed)
	})

	t.Run("payout déjà completed : l'échec ne touche pas aux paiements", func(t *testing.T) {
		cleanupPaymentTables(t, ctx)
		seedPayout(t, "po-c-3", "trip-c-3", "903", domain.PayoutStatusCompleted)
		require.NoError(t, paymentWriteRepo.CreatePayment(ctx, fixtures.NewTestPayment(
			fixtures.WithPaymentID("pay-c-3"), fixtures.WithPaymentTripID("trip-c-3"), fixtures.WithPaymentStatus(domain.PaymentStatusPaidOut),
		)))

		_, err := payoutWriteRepo.FailProcessingPayout(ctx, "po-c-3", "trip-c-3", "late failure")

		assert.ErrorIs(t, err, paymentErrors.ErrorPayoutAlreadyProcessed)
		assert.Equal(t, "paidOut", paymentStatus(t, "pay-c-3"), "transaction annulée")
	})

	t.Run("réconciliation : seulement les processing avec référence", func(t *testing.T) {
		cleanupPaymentTables(t, ctx)
		seedPayout(t, "po-r-1", "trip-r-1", "911", domain.PayoutStatusProcessing)
		seedPayout(t, "po-r-2", "trip-r-2", "", domain.PayoutStatusProcessing)
		seedPayout(t, "po-r-3", "trip-r-3", "913", domain.PayoutStatusCompleted)

		payouts, err := payoutReadRepo.GetProcessingPayoutsToReconcile(ctx, time.Now().UTC().Add(time.Minute), 50)
		require.NoError(t, err)
		require.Len(t, payouts, 1)
		assert.Equal(t, "po-r-1", payouts[0].PayoutID)
		assert.Equal(t, "911", payouts[0].PaymentProviderReference)
	})

	t.Run("reprise automatique plafonnée à 3 payouts échoués", func(t *testing.T) {
		cleanupPaymentTables(t, ctx)
		require.NoError(t, paymentWriteRepo.CreatePayment(ctx, fixtures.NewTestPayment(
			fixtures.WithPaymentID("pay-cap"), fixtures.WithPaymentTripID("trip-cap"), fixtures.WithPaymentStatus(domain.PaymentStatusReleased),
		)))
		seedPayout(t, "po-cap-1", "trip-cap", "921", domain.PayoutStatusFailed)
		seedPayout(t, "po-cap-2", "trip-cap", "922", domain.PayoutStatusFailed)

		tripIDs, err := payoutReadRepo.GetTripsReadyForPayout(ctx)
		require.NoError(t, err)
		assert.Contains(t, tripIDs, "trip-cap", "2 échecs : encore repris automatiquement")

		seedPayout(t, "po-cap-3", "trip-cap", "923", domain.PayoutStatusFailed)
		tripIDs, err = payoutReadRepo.GetTripsReadyForPayout(ctx)
		require.NoError(t, err)
		assert.NotContains(t, tripIDs, "trip-cap", "3 échecs : relance par le support")

		ready, err := payoutReadRepo.IsTripReadyForPayout(ctx, "trip-cap")
		require.NoError(t, err)
		assert.True(t, ready, "le support peut toujours relancer manuellement")
	})
}
