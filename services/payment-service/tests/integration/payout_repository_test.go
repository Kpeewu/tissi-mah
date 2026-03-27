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
// PayoutRepositoryWrite — CreatePayout
// =============================================================================

func TestPayoutWriteRepository_CreatePayout(t *testing.T) {
	ctx := context.Background()
	writeRepo := newTestPayoutWriteRepository()
	readRepo := newTestPayoutReadRepository()

	t.Run("cree un payout avec succes", func(t *testing.T) {
		cleanupPaymentTables(t, ctx)

		payout := fixtures.NewTestPayout(
			fixtures.WithPayoutID("po-create-1"),
			fixtures.WithPayoutDriverID("driver-1"),
			fixtures.WithPayoutTripID("trip-1"),
			fixtures.WithPayoutNetAmount(4500),
		)

		err := writeRepo.CreatePayout(ctx, payout)
		require.NoError(t, err)

		result, err := readRepo.GetByID(ctx, "po-create-1")
		require.NoError(t, err)
		assert.Equal(t, "po-create-1", result.PayoutID)
		assert.Equal(t, "driver-1", result.DriverID)
		assert.Equal(t, 4500, result.NetAmount)
		assert.Equal(t, domain.PayoutStatusPending, result.Status)
	})
}

// =============================================================================
// PayoutRepositoryRead — GetByID
// =============================================================================

func TestPayoutReadRepository_GetByID(t *testing.T) {
	ctx := context.Background()
	readRepo := newTestPayoutReadRepository()

	t.Run("retourne un payout existant", func(t *testing.T) {
		cleanupPaymentTables(t, ctx)

		payout := fixtures.NewTestPayout(
			fixtures.WithPayoutID("po-read-1"),
			fixtures.WithPayoutStatus(domain.PayoutStatusCompleted),
		)
		require.NoError(t, fixtures.InsertPayout(ctx, testPool, payout))

		result, err := readRepo.GetByID(ctx, "po-read-1")

		require.NoError(t, err)
		assert.Equal(t, "po-read-1", result.PayoutID)
		assert.Equal(t, domain.PayoutStatusCompleted, result.Status)
	})

	t.Run("retourne ErrorPayoutNotFound si inexistant", func(t *testing.T) {
		cleanupPaymentTables(t, ctx)

		result, err := readRepo.GetByID(ctx, "po-inexistant")

		assert.ErrorIs(t, err, paymentErrors.ErrorPayoutNotFound)
		assert.Nil(t, result)
	})
}

// =============================================================================
// PayoutRepositoryRead — GetByTripID
// =============================================================================

func TestPayoutReadRepository_GetByTripID(t *testing.T) {
	ctx := context.Background()
	readRepo := newTestPayoutReadRepository()

	t.Run("retourne le payout par trip ID", func(t *testing.T) {
		cleanupPaymentTables(t, ctx)

		payout := fixtures.NewTestPayout(
			fixtures.WithPayoutTripID("trip-find-1"),
		)
		require.NoError(t, fixtures.InsertPayout(ctx, testPool, payout))

		result, err := readRepo.GetByTripID(ctx, "trip-find-1")

		require.NoError(t, err)
		assert.Equal(t, "trip-find-1", result.TripID)
	})
}

// =============================================================================
// PayoutRepositoryRead — GetDriverPayouts
// =============================================================================

func TestPayoutReadRepository_GetDriverPayouts(t *testing.T) {
	ctx := context.Background()
	readRepo := newTestPayoutReadRepository()

	t.Run("retourne les payouts d un chauffeur", func(t *testing.T) {
		cleanupPaymentTables(t, ctx)

		p1 := fixtures.NewTestPayout(fixtures.WithPayoutDriverID("driver-list-1"), fixtures.WithPayoutNetAmount(4500))
		p2 := fixtures.NewTestPayout(fixtures.WithPayoutDriverID("driver-list-1"), fixtures.WithPayoutNetAmount(3000))
		require.NoError(t, fixtures.InsertPayout(ctx, testPool, p1))
		require.NoError(t, fixtures.InsertPayout(ctx, testPool, p2))

		results, err := readRepo.GetDriverPayouts(ctx, "driver-list-1", 0)

		require.NoError(t, err)
		assert.Len(t, results, 2)
	})

	t.Run("retourne une liste vide si aucun payout", func(t *testing.T) {
		cleanupPaymentTables(t, ctx)

		results, err := readRepo.GetDriverPayouts(ctx, "driver-no-payouts", 0)

		require.NoError(t, err)
		assert.Empty(t, results)
	})
}

// =============================================================================
// PayoutRepositoryWrite — UpdatePayoutStatus / MarkPayoutFailed
// =============================================================================

func TestPayoutWriteRepository_UpdatePayoutStatus(t *testing.T) {
	ctx := context.Background()
	writeRepo := newTestPayoutWriteRepository()
	readRepo := newTestPayoutReadRepository()

	t.Run("met a jour le statut du payout", func(t *testing.T) {
		cleanupPaymentTables(t, ctx)

		payout := fixtures.NewTestPayout(
			fixtures.WithPayoutID("po-upd-1"),
			fixtures.WithPayoutStatus(domain.PayoutStatusPending),
		)
		require.NoError(t, fixtures.InsertPayout(ctx, testPool, payout))

		err := writeRepo.UpdatePayoutStatus(ctx, "po-upd-1", domain.PayoutStatusProcessing, "provider-ref-123")
		require.NoError(t, err)

		result, err := readRepo.GetByID(ctx, "po-upd-1")
		require.NoError(t, err)
		assert.Equal(t, domain.PayoutStatusProcessing, result.Status)
	})
}

func TestPayoutWriteRepository_MarkPayoutFailed(t *testing.T) {
	ctx := context.Background()
	writeRepo := newTestPayoutWriteRepository()
	readRepo := newTestPayoutReadRepository()

	t.Run("marque un payout comme echoue", func(t *testing.T) {
		cleanupPaymentTables(t, ctx)

		payout := fixtures.NewTestPayout(fixtures.WithPayoutID("po-fail-1"))
		require.NoError(t, fixtures.InsertPayout(ctx, testPool, payout))

		err := writeRepo.MarkPayoutFailed(ctx, "po-fail-1", "Solde insuffisant")
		require.NoError(t, err)

		result, err := readRepo.GetByID(ctx, "po-fail-1")
		require.NoError(t, err)
		assert.Equal(t, domain.PayoutStatusFailed, result.Status)
		require.NotNil(t, result.FailureReason)
		assert.Equal(t, "Solde insuffisant", *result.FailureReason)
	})
}
