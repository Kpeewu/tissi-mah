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
// GetByID
// =============================================================================

func TestPaymentReadRepository_GetByID(t *testing.T) {
	ctx := context.Background()
	repo := newTestPaymentReadRepository()

	t.Run("retourne un paiement existant", func(t *testing.T) {
		cleanupPaymentTables(t, ctx)

		payment := fixtures.NewTestPayment(
			fixtures.WithPaymentID("pay-read-1"),
			fixtures.WithPaymentBookingID("booking-read-1"),
			fixtures.WithPaymentAmount(5000),
			fixtures.WithPaymentStatus(domain.PaymentStatusPending),
		)
		require.NoError(t, fixtures.InsertPayment(ctx, testPool, payment))

		result, err := repo.GetByID(ctx, "pay-read-1")

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "pay-read-1", result.PaymentID)
		assert.Equal(t, "booking-read-1", result.BookingID)
		assert.Equal(t, 5000, result.Amount)
		assert.Equal(t, domain.PaymentStatusPending, result.Status)
		assert.Equal(t, domain.PaymentMobileMoney, result.PaymentMethod)
	})

	t.Run("retourne ErrorPaymentNotFound si inexistant", func(t *testing.T) {
		cleanupPaymentTables(t, ctx)

		result, err := repo.GetByID(ctx, "pay-inexistant")

		assert.ErrorIs(t, err, paymentErrors.ErrorPaymentNotFound)
		assert.Nil(t, result)
	})
}

// =============================================================================
// GetByBookingID
// =============================================================================

func TestPaymentReadRepository_GetByBookingID(t *testing.T) {
	ctx := context.Background()
	repo := newTestPaymentReadRepository()

	t.Run("retourne le paiement le plus recent pour un booking", func(t *testing.T) {
		cleanupPaymentTables(t, ctx)

		payment := fixtures.NewTestPayment(
			fixtures.WithPaymentBookingID("booking-find-1"),
			fixtures.WithPaymentAmount(3000),
		)
		require.NoError(t, fixtures.InsertPayment(ctx, testPool, payment))

		result, err := repo.GetByBookingID(ctx, "booking-find-1")

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "booking-find-1", result.BookingID)
		assert.Equal(t, 3000, result.Amount)
	})

	t.Run("retourne ErrorPaymentNotFound si aucun paiement", func(t *testing.T) {
		cleanupPaymentTables(t, ctx)

		result, err := repo.GetByBookingID(ctx, "booking-inexistant")

		assert.ErrorIs(t, err, paymentErrors.ErrorPaymentNotFound)
		assert.Nil(t, result)
	})
}

// =============================================================================
// GetByExternalTransactionID
// =============================================================================

func TestPaymentReadRepository_GetByExternalTransactionID(t *testing.T) {
	ctx := context.Background()
	repo := newTestPaymentReadRepository()

	t.Run("retourne le paiement par external ID", func(t *testing.T) {
		cleanupPaymentTables(t, ctx)

		payment := fixtures.NewTestPayment(
			fixtures.WithExternalTransactionID("ext-txn-42"),
		)
		require.NoError(t, fixtures.InsertPayment(ctx, testPool, payment))

		result, err := repo.GetByExternalTransactionID(ctx, "ext-txn-42")

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "ext-txn-42", result.ExternalTransactionID)
	})

	t.Run("retourne ErrorPaymentNotFound si inexistant", func(t *testing.T) {
		cleanupPaymentTables(t, ctx)

		result, err := repo.GetByExternalTransactionID(ctx, "ext-inexistant")

		assert.ErrorIs(t, err, paymentErrors.ErrorPaymentNotFound)
		assert.Nil(t, result)
	})
}
