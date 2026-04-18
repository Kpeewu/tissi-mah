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

func TestPayoutReadRepository_IsTripReadyForPayout(t *testing.T) {
	ctx := context.Background()
	readRepo := newTestPayoutReadRepository()

	t.Run("aucun paiement - retourne false", func(t *testing.T) {
		cleanupPaymentTables(t, ctx)

		ready, err := readRepo.IsTripReadyForPayout(ctx, "trip-empty")

		require.NoError(t, err)
		assert.False(t, ready)
	})

	t.Run("released uniquement - retourne true", func(t *testing.T) {
		cleanupPaymentTables(t, ctx)

		p := fixtures.NewTestPayment(
			fixtures.WithPaymentTripID("trip-ready-1"),
			fixtures.WithPaymentStatus(domain.PaymentStatusReleased),
		)
		require.NoError(t, fixtures.InsertPayment(ctx, testPool, p))

		ready, err := readRepo.IsTripReadyForPayout(ctx, "trip-ready-1")

		require.NoError(t, err)
		assert.True(t, ready)
	})

	t.Run("paiement pending présent - retourne false", func(t *testing.T) {
		cleanupPaymentTables(t, ctx)

		released := fixtures.NewTestPayment(
			fixtures.WithPaymentTripID("trip-pending-1"),
			fixtures.WithPaymentStatus(domain.PaymentStatusReleased),
		)
		pending := fixtures.NewTestPayment(
			fixtures.WithPaymentTripID("trip-pending-1"),
			fixtures.WithPaymentStatus(domain.PaymentStatusPending),
		)
		require.NoError(t, fixtures.InsertPayment(ctx, testPool, released))
		require.NoError(t, fixtures.InsertPayment(ctx, testPool, pending))

		ready, err := readRepo.IsTripReadyForPayout(ctx, "trip-pending-1")

		require.NoError(t, err)
		assert.False(t, ready)
	})

	t.Run("paiement held présent - retourne false", func(t *testing.T) {
		cleanupPaymentTables(t, ctx)

		released := fixtures.NewTestPayment(
			fixtures.WithPaymentTripID("trip-held-1"),
			fixtures.WithPaymentStatus(domain.PaymentStatusReleased),
		)
		held := fixtures.NewTestPayment(
			fixtures.WithPaymentTripID("trip-held-1"),
			fixtures.WithPaymentStatus(domain.PaymentStatusHeld),
		)
		require.NoError(t, fixtures.InsertPayment(ctx, testPool, released))
		require.NoError(t, fixtures.InsertPayment(ctx, testPool, held))

		ready, err := readRepo.IsTripReadyForPayout(ctx, "trip-held-1")

		require.NoError(t, err)
		assert.False(t, ready)
	})

	t.Run("payout scheduled existant - retourne false", func(t *testing.T) {
		cleanupPaymentTables(t, ctx)

		released := fixtures.NewTestPayment(
			fixtures.WithPaymentTripID("trip-payout-1"),
			fixtures.WithPaymentStatus(domain.PaymentStatusReleased),
		)
		require.NoError(t, fixtures.InsertPayment(ctx, testPool, released))

		payout := fixtures.NewTestPayout(
			fixtures.WithPayoutTripID("trip-payout-1"),
			fixtures.WithPayoutStatus(domain.PayoutStatusScheduled),
		)
		require.NoError(t, fixtures.InsertPayout(ctx, testPool, payout))

		ready, err := readRepo.IsTripReadyForPayout(ctx, "trip-payout-1")

		require.NoError(t, err)
		assert.False(t, ready)
	})

	t.Run("payout failed ignoré - retourne true", func(t *testing.T) {
		cleanupPaymentTables(t, ctx)

		released := fixtures.NewTestPayment(
			fixtures.WithPaymentTripID("trip-failed-1"),
			fixtures.WithPaymentStatus(domain.PaymentStatusReleased),
		)
		require.NoError(t, fixtures.InsertPayment(ctx, testPool, released))

		payout := fixtures.NewTestPayout(
			fixtures.WithPayoutTripID("trip-failed-1"),
			fixtures.WithPayoutStatus(domain.PayoutStatusFailed),
		)
		require.NoError(t, fixtures.InsertPayout(ctx, testPool, payout))

		ready, err := readRepo.IsTripReadyForPayout(ctx, "trip-failed-1")

		require.NoError(t, err)
		assert.True(t, ready)
	})

	t.Run("payout cancelled ignoré - retourne true", func(t *testing.T) {
		cleanupPaymentTables(t, ctx)

		released := fixtures.NewTestPayment(
			fixtures.WithPaymentTripID("trip-cancel-1"),
			fixtures.WithPaymentStatus(domain.PaymentStatusReleased),
		)
		require.NoError(t, fixtures.InsertPayment(ctx, testPool, released))

		payout := fixtures.NewTestPayout(
			fixtures.WithPayoutTripID("trip-cancel-1"),
			fixtures.WithPayoutStatus(domain.PayoutStatusCancelled),
		)
		require.NoError(t, fixtures.InsertPayout(ctx, testPool, payout))

		ready, err := readRepo.IsTripReadyForPayout(ctx, "trip-cancel-1")

		require.NoError(t, err)
		assert.True(t, ready)
	})
}

func TestPayoutReadRepository_GetByProviderReference(t *testing.T) {
	ctx := context.Background()
	readRepo := newTestPayoutReadRepository()

	t.Run("retourne le payout correspondant", func(t *testing.T) {
		cleanupPaymentTables(t, ctx)

		payout := fixtures.NewTestPayout(fixtures.WithPayoutID("po-ref-1"))
		payout.PaymentProviderReference = "fedapay-ref-123"
		require.NoError(t, fixtures.InsertPayout(ctx, testPool, payout))

		result, err := readRepo.GetByProviderReference(ctx, "fedapay-ref-123")

		require.NoError(t, err)
		assert.Equal(t, "po-ref-1", result.PayoutID)
	})

	t.Run("plusieurs payouts même référence - retourne le plus récent", func(t *testing.T) {
		cleanupPaymentTables(t, ctx)

		p1 := fixtures.NewTestPayout(fixtures.WithPayoutID("po-ref-old"))
		p1.PaymentProviderReference = "fedapay-multi"
		require.NoError(t, fixtures.InsertPayout(ctx, testPool, p1))

		p2 := fixtures.NewTestPayout(fixtures.WithPayoutID("po-ref-new"))
		p2.PaymentProviderReference = "fedapay-multi"
		require.NoError(t, fixtures.InsertPayout(ctx, testPool, p2))

		result, err := readRepo.GetByProviderReference(ctx, "fedapay-multi")

		require.NoError(t, err)
		assert.Equal(t, "po-ref-new", result.PayoutID)
	})

	t.Run("référence inexistante - retourne ErrorPayoutNotFound", func(t *testing.T) {
		cleanupPaymentTables(t, ctx)

		result, err := readRepo.GetByProviderReference(ctx, "fedapay-not-exist")

		assert.ErrorIs(t, err, paymentErrors.ErrorPayoutNotFound)
		assert.Nil(t, result)
	})
}
