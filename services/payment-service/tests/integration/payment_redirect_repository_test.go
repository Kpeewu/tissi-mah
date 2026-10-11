package integration

import (
	"context"
	"testing"
	"time"

	"github.com/Kpeewu/tissi-mah/services/payment-service/fixtures"
	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/domain"
	paymentErrors "github.com/Kpeewu/tissi-mah/services/payment-service/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Paiement avec redirection — colonnes payment_url / provider_status / expires_at
// =============================================================================

func TestPaymentRepository_CreatePaymentWithRedirectFields(t *testing.T) {
	ctx := context.Background()
	writeRepo := newTestPaymentWriteRepository()
	readRepo := newTestPaymentReadRepository()
	cleanupPaymentTables(t, ctx)

	expiresAt := time.Now().UTC().Add(10 * time.Minute).Truncate(time.Second)
	payment := fixtures.NewTestPayment(
		fixtures.WithPaymentID("pay-redirect-1"),
		fixtures.WithPaymentURL("https://process.fedapay.com/tok"),
		fixtures.WithPaymentExpiresAt(expiresAt),
	)
	require.NoError(t, writeRepo.CreatePayment(ctx, payment))

	result, err := readRepo.GetByID(ctx, "pay-redirect-1")
	require.NoError(t, err)
	assert.Equal(t, "https://process.fedapay.com/tok", result.PaymentURL)
	assert.Equal(t, "pending", result.ProviderStatus)
	assert.True(t, result.ExpiresAt.Equal(expiresAt))
}

func TestPaymentReadRepository_GetExpiredPendingPayments_UsesExpiresAt(t *testing.T) {
	ctx := context.Background()
	repo := newTestPaymentReadRepository()
	cleanupPaymentTables(t, ctx)

	now := time.Now().UTC()
	require.NoError(t, fixtures.InsertPayment(ctx, testPool, fixtures.NewTestPayment(
		fixtures.WithPaymentID("pay-expired"),
		fixtures.WithExternalTransactionID("1"),
		fixtures.WithPaymentExpiresAt(now.Add(-time.Minute)),
	)))
	require.NoError(t, fixtures.InsertPayment(ctx, testPool, fixtures.NewTestPayment(
		fixtures.WithPaymentID("pay-not-expired"),
		fixtures.WithExternalTransactionID("2"),
		fixtures.WithPaymentExpiresAt(now.Add(5*time.Minute)),
	)))
	require.NoError(t, fixtures.InsertPayment(ctx, testPool, fixtures.NewTestPayment(
		fixtures.WithPaymentID("pay-expired-held"),
		fixtures.WithExternalTransactionID("3"),
		fixtures.WithPaymentStatus(domain.PaymentStatusHeld),
		fixtures.WithPaymentExpiresAt(now.Add(-time.Minute)),
	)))

	payments, err := repo.GetExpiredPendingPayments(ctx, now)

	require.NoError(t, err)
	require.Len(t, payments, 1)
	assert.Equal(t, "pay-expired", payments[0].PaymentID)
}

func TestPaymentReadRepository_GetPendingByBookingID(t *testing.T) {
	ctx := context.Background()
	repo := newTestPaymentReadRepository()
	cleanupPaymentTables(t, ctx)

	require.NoError(t, fixtures.InsertPayment(ctx, testPool, fixtures.NewTestPayment(
		fixtures.WithPaymentID("pay-failed-old"),
		fixtures.WithPaymentBookingID("bk-pending"),
		fixtures.WithExternalTransactionID("10"),
		fixtures.WithPaymentStatus(domain.PaymentStatusFailed),
	)))
	require.NoError(t, fixtures.InsertPayment(ctx, testPool, fixtures.NewTestPayment(
		fixtures.WithPaymentID("pay-pending"),
		fixtures.WithPaymentBookingID("bk-pending"),
		fixtures.WithExternalTransactionID("11"),
	)))

	result, err := repo.GetPendingByBookingID(ctx, "bk-pending")
	require.NoError(t, err)
	assert.Equal(t, "pay-pending", result.PaymentID)

	_, err = repo.GetPendingByBookingID(ctx, "bk-unknown")
	assert.ErrorIs(t, err, paymentErrors.ErrorPaymentNotFound)
}

func TestPaymentWriteRepository_UpdateProviderStatusAndURL(t *testing.T) {
	ctx := context.Background()
	writeRepo := newTestPaymentWriteRepository()
	readRepo := newTestPaymentReadRepository()
	cleanupPaymentTables(t, ctx)

	require.NoError(t, fixtures.InsertPayment(ctx, testPool, fixtures.NewTestPayment(
		fixtures.WithPaymentID("pay-provider"),
	)))

	require.NoError(t, writeRepo.UpdateProviderStatus(ctx, "pay-provider", "declined"))
	require.NoError(t, writeRepo.UpdatePaymentURL(ctx, "pay-provider", "https://process.fedapay.com/new"))

	result, err := readRepo.GetByID(ctx, "pay-provider")
	require.NoError(t, err)
	assert.Equal(t, domain.PaymentStatusPending, result.Status) // declined n'est pas final
	assert.Equal(t, "declined", result.ProviderStatus)
	assert.Equal(t, "https://process.fedapay.com/new", result.PaymentURL)
}

func TestPaymentWriteRepository_MarkFailedPaymentHeld(t *testing.T) {
	ctx := context.Background()
	writeRepo := newTestPaymentWriteRepository()
	readRepo := newTestPaymentReadRepository()

	t.Run("failed → held (approbation tardive)", func(t *testing.T) {
		cleanupPaymentTables(t, ctx)
		require.NoError(t, fixtures.InsertPayment(ctx, testPool, fixtures.NewTestPayment(
			fixtures.WithPaymentID("pay-late"),
			fixtures.WithPaymentStatus(domain.PaymentStatusFailed),
		)))

		require.NoError(t, writeRepo.MarkFailedPaymentHeld(ctx, "pay-late"))

		result, err := readRepo.GetByID(ctx, "pay-late")
		require.NoError(t, err)
		assert.Equal(t, domain.PaymentStatusHeld, result.Status)
		assert.Equal(t, "approved", result.ProviderStatus)
		assert.NotNil(t, result.CompletedAt)
	})

	t.Run("refuse si le paiement n'est pas failed", func(t *testing.T) {
		cleanupPaymentTables(t, ctx)
		require.NoError(t, fixtures.InsertPayment(ctx, testPool, fixtures.NewTestPayment(
			fixtures.WithPaymentID("pay-pending-late"),
		)))

		err := writeRepo.MarkFailedPaymentHeld(ctx, "pay-pending-late")
		assert.ErrorIs(t, err, paymentErrors.ErrorPaymentAlreadyProcessed)
	})
}

// =============================================================================
// Rattachement des paiements held — colonne booking_synced_at
// =============================================================================

func TestPaymentRepository_UnsyncedHeldPayments(t *testing.T) {
	ctx := context.Background()
	writeRepo := newTestPaymentWriteRepository()
	readRepo := newTestPaymentReadRepository()
	cleanupPaymentTables(t, ctx)

	now := time.Now().UTC()
	insert := func(paymentID, bookingID, externalID string, status domain.PaymentStatus, completedAt time.Time) {
		require.NoError(t, fixtures.InsertPayment(ctx, testPool, fixtures.NewTestPayment(
			fixtures.WithPaymentID(paymentID),
			fixtures.WithPaymentBookingID(bookingID),
			fixtures.WithExternalTransactionID(externalID),
			fixtures.WithPaymentStatus(status),
			fixtures.WithPaymentCompletedAt(completedAt),
		)))
	}
	insert("pay-unsynced-old", "bk-us-1", "21", domain.PaymentStatusHeld, now.Add(-10*time.Minute))
	insert("pay-unsynced-older", "bk-us-2", "22", domain.PaymentStatusHeld, now.Add(-20*time.Minute))
	insert("pay-unsynced-recent", "bk-us-3", "23", domain.PaymentStatusHeld, now)
	insert("pay-synced", "bk-us-4", "24", domain.PaymentStatusHeld, now.Add(-10*time.Minute))
	insert("pay-released", "bk-us-5", "25", domain.PaymentStatusReleased, now.Add(-10*time.Minute))
	require.NoError(t, writeRepo.MarkBookingSynced(ctx, "pay-synced"))

	payments, err := readRepo.GetUnsyncedHeldPayments(ctx, now.Add(-2*time.Minute), 100)

	require.NoError(t, err)
	require.Len(t, payments, 2)
	assert.Equal(t, "pay-unsynced-older", payments[0].PaymentID) // plus ancien d'abord
	assert.Equal(t, "pay-unsynced-old", payments[1].PaymentID)

	t.Run("limite respectée", func(t *testing.T) {
		limited, err := readRepo.GetUnsyncedHeldPayments(ctx, now.Add(-2*time.Minute), 1)
		require.NoError(t, err)
		require.Len(t, limited, 1)
		assert.Equal(t, "pay-unsynced-older", limited[0].PaymentID)
	})

	t.Run("MarkBookingSynced idempotent", func(t *testing.T) {
		require.NoError(t, writeRepo.MarkBookingSynced(ctx, "pay-unsynced-old"))
		require.NoError(t, writeRepo.MarkBookingSynced(ctx, "pay-unsynced-old"))

		remaining, err := readRepo.GetUnsyncedHeldPayments(ctx, now.Add(-2*time.Minute), 100)
		require.NoError(t, err)
		require.Len(t, remaining, 1)
		assert.Equal(t, "pay-unsynced-older", remaining[0].PaymentID)
	})
}

func TestPaymentWriteRepository_MarkFailedPaymentHeld_ResetsBookingSync(t *testing.T) {
	ctx := context.Background()
	writeRepo := newTestPaymentWriteRepository()
	readRepo := newTestPaymentReadRepository()
	cleanupPaymentTables(t, ctx)

	require.NoError(t, fixtures.InsertPayment(ctx, testPool, fixtures.NewTestPayment(
		fixtures.WithPaymentID("pay-late-sync"),
		fixtures.WithPaymentStatus(domain.PaymentStatusFailed),
	)))
	_, err := testPool.Exec(ctx,
		`UPDATE payments SET booking_synced_at = NOW(), booking_sync_attempts = 5 WHERE payment_id = $1`,
		"pay-late-sync")
	require.NoError(t, err)

	require.NoError(t, writeRepo.MarkFailedPaymentHeld(ctx, "pay-late-sync"))

	payments, err := readRepo.GetUnsyncedHeldPayments(ctx, time.Now().UTC().Add(time.Minute), 100)
	require.NoError(t, err)
	require.Len(t, payments, 1)
	assert.Equal(t, "pay-late-sync", payments[0].PaymentID)
	assert.Equal(t, 0, payments[0].BookingSyncAttempts) // nouvelle approbation : compteur remis à zéro
}

func TestPaymentWriteRepository_IncrementBookingSyncAttempts(t *testing.T) {
	ctx := context.Background()
	writeRepo := newTestPaymentWriteRepository()
	readRepo := newTestPaymentReadRepository()
	cleanupPaymentTables(t, ctx)

	require.NoError(t, fixtures.InsertPayment(ctx, testPool, fixtures.NewTestPayment(
		fixtures.WithPaymentID("pay-attempts"),
		fixtures.WithPaymentStatus(domain.PaymentStatusHeld),
	)))

	attempts, err := writeRepo.IncrementBookingSyncAttempts(ctx, "pay-attempts")
	require.NoError(t, err)
	assert.Equal(t, 1, attempts)

	attempts, err = writeRepo.IncrementBookingSyncAttempts(ctx, "pay-attempts")
	require.NoError(t, err)
	assert.Equal(t, 2, attempts)

	result, err := readRepo.GetByID(ctx, "pay-attempts")
	require.NoError(t, err)
	assert.Equal(t, 2, result.BookingSyncAttempts)

	_, err = writeRepo.IncrementBookingSyncAttempts(ctx, "pay-unknown")
	assert.ErrorIs(t, err, paymentErrors.ErrorPaymentNotFound)
}
