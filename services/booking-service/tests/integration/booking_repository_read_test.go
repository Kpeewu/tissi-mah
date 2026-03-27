package integration

import (
	"context"
	"testing"
	"time"

	"github.com/Kpeewu/tissi-mah/services/booking-service/fixtures"
	"github.com/Kpeewu/tissi-mah/services/booking-service/internal/domain"
	bookingErrors "github.com/Kpeewu/tissi-mah/services/booking-service/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// GetByID
// =============================================================================

func TestBookingReadRepository_GetByID(t *testing.T) {
	ctx := context.Background()
	repo := newTestBookingReadRepository()

	t.Run("retourne un booking existant", func(t *testing.T) {
		cleanupBookingTables(t, ctx)

		booking := fixtures.NewTestBooking(
			fixtures.WithBookingID("bk-read-1"),
			fixtures.WithPassengerID("pax-1"),
			fixtures.WithTotalAmount(5500),
			fixtures.WithBookingStatus(domain.BookingStatusPendingApproval),
		)
		require.NoError(t, fixtures.InsertBooking(ctx, testPool, booking))

		result, err := repo.GetByID(ctx, "bk-read-1")

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "bk-read-1", result.BookingID)
		assert.Equal(t, "pax-1", result.PassengerID)
		assert.Equal(t, 5500, result.TotalAmount)
		assert.Equal(t, domain.BookingStatusPendingApproval, result.Status)
	})

	t.Run("retourne ErrorBookingNotFound si inexistant", func(t *testing.T) {
		cleanupBookingTables(t, ctx)

		result, err := repo.GetByID(ctx, "bk-inexistant")

		assert.ErrorIs(t, err, bookingErrors.ErrorBookingNotFound)
		assert.Nil(t, result)
	})
}

// =============================================================================
// HasActiveBooking
// =============================================================================

func TestBookingReadRepository_HasActiveBooking(t *testing.T) {
	ctx := context.Background()
	repo := newTestBookingReadRepository()

	t.Run("retourne true si booking actif existe", func(t *testing.T) {
		cleanupBookingTables(t, ctx)

		booking := fixtures.NewTestBooking(
			fixtures.WithPassengerID("pax-active"),
			fixtures.WithTripID("trip-active"),
			fixtures.WithBookingStatus(domain.BookingStatusApproved),
		)
		require.NoError(t, fixtures.InsertBooking(ctx, testPool, booking))

		exists, err := repo.HasActiveBooking(ctx, "pax-active", "trip-active")

		require.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("retourne false si aucun booking actif", func(t *testing.T) {
		cleanupBookingTables(t, ctx)

		exists, err := repo.HasActiveBooking(ctx, "pax-none", "trip-none")

		require.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("retourne false si booking annule", func(t *testing.T) {
		cleanupBookingTables(t, ctx)

		booking := fixtures.NewTestBooking(
			fixtures.WithPassengerID("pax-cancelled"),
			fixtures.WithTripID("trip-cancelled"),
			fixtures.WithBookingStatus(domain.BookingStatusCancelled),
			fixtures.WithCancellerID("pax-cancelled"),
			fixtures.WithCancellationReason("test"),
		)
		require.NoError(t, fixtures.InsertBooking(ctx, testPool, booking))

		exists, err := repo.HasActiveBooking(ctx, "pax-cancelled", "trip-cancelled")

		require.NoError(t, err)
		assert.False(t, exists)
	})
}

// =============================================================================
// GetCompletedBookingsPendingRelease
// =============================================================================

func TestBookingReadRepository_GetCompletedBookingsPendingRelease(t *testing.T) {
	ctx := context.Background()
	repo := newTestBookingReadRepository()

	t.Run("retourne les bookings completes non-cash en attente de release", func(t *testing.T) {
		cleanupBookingTables(t, ctx)

		now := time.Now().UTC()
		twoHoursAgo := now.Add(-2 * time.Hour)

		booking := fixtures.NewTestBooking(
			fixtures.WithBookingID("bk-pending-release"),
			fixtures.WithBookingStatus(domain.BookingStatusCompleted),
			fixtures.WithPaymentMethod(domain.PaymentMobileMoney),
			fixtures.WithPaymentCompletedAt(twoHoursAgo),
			fixtures.WithCompletedAt(twoHoursAgo),
		)
		require.NoError(t, fixtures.InsertBooking(ctx, testPool, booking))

		results, err := repo.GetCompletedBookingsPendingRelease(ctx, now.Add(-1*time.Hour))

		require.NoError(t, err)
		assert.Len(t, results, 1)
		assert.Equal(t, "bk-pending-release", results[0].BookingID)
	})

	t.Run("exclut les bookings cash", func(t *testing.T) {
		cleanupBookingTables(t, ctx)

		now := time.Now().UTC()
		twoHoursAgo := now.Add(-2 * time.Hour)

		booking := fixtures.NewTestCashBooking(
			fixtures.WithBookingStatus(domain.BookingStatusCompleted),
			fixtures.WithCompletedAt(twoHoursAgo),
		)
		require.NoError(t, fixtures.InsertBooking(ctx, testPool, booking))

		results, err := repo.GetCompletedBookingsPendingRelease(ctx, now.Add(-1*time.Hour))

		require.NoError(t, err)
		assert.Empty(t, results)
	})

	t.Run("exclut les bookings deja released", func(t *testing.T) {
		cleanupBookingTables(t, ctx)

		now := time.Now().UTC()
		twoHoursAgo := now.Add(-2 * time.Hour)

		booking := fixtures.NewTestBooking(
			fixtures.WithBookingStatus(domain.BookingStatusCompleted),
			fixtures.WithPaymentMethod(domain.PaymentMobileMoney),
			fixtures.WithPaymentCompletedAt(twoHoursAgo),
			fixtures.WithCompletedAt(twoHoursAgo),
		)
		require.NoError(t, fixtures.InsertBooking(ctx, testPool, booking))

		// Marquer comme released
		_, err := testPool.Exec(ctx, "UPDATE bookings SET payment_released_at = NOW() WHERE booking_id = $1", booking.BookingID)
		require.NoError(t, err)

		results, err := repo.GetCompletedBookingsPendingRelease(ctx, now.Add(-1*time.Hour))

		require.NoError(t, err)
		assert.Empty(t, results)
	})
}

// =============================================================================
// GetActiveBookingsSeatsForTrip
// =============================================================================

func TestBookingReadRepository_GetActiveBookingsSeatsForTrip(t *testing.T) {
	ctx := context.Background()
	repo := newTestBookingReadRepository()

	t.Run("retourne le total des places reservees actives", func(t *testing.T) {
		cleanupBookingTables(t, ctx)

		b1 := fixtures.NewTestBooking(
			fixtures.WithTripID("trip-seats"),
			fixtures.WithSeatsBooked(2),
			fixtures.WithSubtotal(10000),
			fixtures.WithServiceFee(1000),
			fixtures.WithTotalAmount(11000),
			fixtures.WithBookingStatus(domain.BookingStatusApproved),
		)
		b2 := fixtures.NewTestBooking(
			fixtures.WithTripID("trip-seats"),
			fixtures.WithSeatsBooked(1),
			fixtures.WithBookingStatus(domain.BookingStatusPendingApproval),
		)
		require.NoError(t, fixtures.InsertBooking(ctx, testPool, b1))
		require.NoError(t, fixtures.InsertBooking(ctx, testPool, b2))

		seats, err := repo.GetActiveBookingsSeatsForTrip(ctx, "trip-seats")

		require.NoError(t, err)
		assert.Equal(t, 3, seats) // 2 + 1
	})

	t.Run("retourne 0 si aucun booking actif", func(t *testing.T) {
		cleanupBookingTables(t, ctx)

		seats, err := repo.GetActiveBookingsSeatsForTrip(ctx, "trip-no-bookings")

		require.NoError(t, err)
		assert.Equal(t, 0, seats)
	})
}
