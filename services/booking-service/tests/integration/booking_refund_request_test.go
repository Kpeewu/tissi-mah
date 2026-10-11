package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Kpeewu/tissi-mah/services/booking-service/fixtures"
	"github.com/Kpeewu/tissi-mah/services/booking-service/internal/domain"
)

// =============================================================================
// Remboursement dû : refund_reason posé avec la transition
// =============================================================================

func TestBookingWriteRepository_TransitionsRecordRefundReason(t *testing.T) {
	ctx := context.Background()
	writeRepo := newTestBookingWriteRepository()
	readRepo := newTestBookingReadRepository()

	// insert insère une réservation payée en ligne (ou non) dans le statut donné
	insert := func(t *testing.T, id string, status domain.BookingStatus, paid bool, opts ...fixtures.BookingOption) {
		t.Helper()
		opts = append([]fixtures.BookingOption{
			fixtures.WithBookingID(id),
			fixtures.WithPassengerID("pax-" + id),
			fixtures.WithDriverID("driver-" + id),
			fixtures.WithBookingStatus(status),
		}, opts...)
		var b *domain.Booking
		if paid {
			b = fixtures.NewTestBookingWithPayment(opts...)
		} else {
			b = fixtures.NewTestBooking(opts...)
		}
		require.NoError(t, fixtures.InsertBookingWithHistory(ctx, testPool, b))
	}

	// refundReason relit le motif enregistré ("" si NULL)
	refundReason := func(t *testing.T, id string) string {
		t.Helper()
		b, err := readRepo.GetByID(ctx, id)
		require.NoError(t, err)
		assert.Nil(t, b.RefundRequestedAt)
		if b.RefundReason == nil {
			return ""
		}
		return *b.RefundReason
	}

	t.Run("annulation passager / chauffeur", func(t *testing.T) {
		cleanupBookingTables(t, ctx)
		insert(t, "bk-rr-pax", domain.BookingStatusApproved, true)
		insert(t, "bk-rr-drv", domain.BookingStatusApproved, true)

		require.NoError(t, writeRepo.Cancel(ctx, "bk-rr-pax", "pax-bk-rr-pax", "Changement de plan"))
		require.NoError(t, writeRepo.Cancel(ctx, "bk-rr-drv", "driver-bk-rr-drv", "Imprévu"))

		assert.Equal(t, domain.RefundReasonCancelledByPassenger, refundReason(t, "bk-rr-pax"))
		assert.Equal(t, domain.RefundReasonCancelledByDriver, refundReason(t, "bk-rr-drv"))
	})

	t.Run("rien à rembourser : cash ou paiement non abouti", func(t *testing.T) {
		cleanupBookingTables(t, ctx)
		insert(t, "bk-rr-cash", domain.BookingStatusApproved, false, fixtures.WithPaymentMethod(domain.PaymentCash))
		insert(t, "bk-rr-unpaid", domain.BookingStatusPendingApproval, false)

		require.NoError(t, writeRepo.Cancel(ctx, "bk-rr-cash", "pax-bk-rr-cash", ""))
		require.NoError(t, writeRepo.Reject(ctx, "bk-rr-unpaid", "driver-bk-rr-unpaid", ""))

		assert.Empty(t, refundReason(t, "bk-rr-cash"))
		assert.Empty(t, refundReason(t, "bk-rr-unpaid"))
	})

	t.Run("rejet et no-show", func(t *testing.T) {
		cleanupBookingTables(t, ctx)
		insert(t, "bk-rr-reject", domain.BookingStatusPendingApproval, true)
		insert(t, "bk-rr-noshow-d", domain.BookingStatusApproved, true)
		insert(t, "bk-rr-noshow-p", domain.BookingStatusInProgress, true)

		require.NoError(t, writeRepo.Reject(ctx, "bk-rr-reject", "driver-bk-rr-reject", "Complet"))
		require.NoError(t, writeRepo.ReportNoShow(ctx, "bk-rr-noshow-d", "pax-bk-rr-noshow-d", "driver", ""))
		require.NoError(t, writeRepo.ReportNoShow(ctx, "bk-rr-noshow-p", "driver-bk-rr-noshow-p", "passenger", ""))

		assert.Equal(t, domain.RefundReasonBookingRejected, refundReason(t, "bk-rr-reject"))
		assert.Equal(t, domain.RefundReasonNoShowDriver, refundReason(t, "bk-rr-noshow-d"))
		assert.Equal(t, domain.RefundReasonNoShowPassenger, refundReason(t, "bk-rr-noshow-p"))
	})

	t.Run("annulations groupées trajet / arrêt", func(t *testing.T) {
		cleanupBookingTables(t, ctx)
		insert(t, "bk-rr-trip", domain.BookingStatusApproved, true, fixtures.WithTripID("trip-rr"))
		insert(t, "bk-rr-trip-pending", domain.BookingStatusPaymentPending, false,
			fixtures.WithTripID("trip-rr"), fixtures.WithNoPaymentCompleted())
		insert(t, "bk-rr-wp", domain.BookingStatusApproved, true,
			fixtures.WithTripID("trip-rr-wp"), fixtures.WithPickupWaypointID("wp-rr"))

		_, err := writeRepo.CancelBookingsForTrip(ctx, "trip-rr")
		require.NoError(t, err)
		_, err = writeRepo.CancelBookingsForWaypoint(ctx, "trip-rr-wp", "wp-rr")
		require.NoError(t, err)

		assert.Equal(t, domain.RefundReasonTripCancelled, refundReason(t, "bk-rr-trip"))
		assert.Empty(t, refundReason(t, "bk-rr-trip-pending"))
		assert.Equal(t, domain.RefundReasonWaypointCancelled, refundReason(t, "bk-rr-wp"))
	})
}

// =============================================================================
// Worker de rejeu : sélection, acquittement, refus comptés
// =============================================================================

func TestBookingRepository_RefundRequestLifecycle(t *testing.T) {
	ctx := context.Background()
	writeRepo := newTestBookingWriteRepository()
	readRepo := newTestBookingReadRepository()

	pendingIDs := func(t *testing.T, updatedBefore time.Time) []string {
		t.Helper()
		bookings, err := readRepo.GetBookingsPendingRefundRequest(ctx, updatedBefore, 5, 100)
		require.NoError(t, err)
		ids := make([]string, 0, len(bookings))
		for _, b := range bookings {
			ids = append(ids, b.BookingID)
			require.NotNil(t, b.RefundReason)
		}
		return ids
	}

	cleanupBookingTables(t, ctx)
	for _, id := range []string{"bk-life-1", "bk-life-2", "bk-life-3"} {
		b := fixtures.NewTestBookingWithPayment(
			fixtures.WithBookingID(id),
			fixtures.WithTripID("trip-life"),
			fixtures.WithBookingStatus(domain.BookingStatusApproved),
		)
		require.NoError(t, fixtures.InsertBookingWithHistory(ctx, testPool, b))
	}
	_, err := writeRepo.CancelBookingsForTrip(ctx, "trip-life")
	require.NoError(t, err)

	// Délai de grâce : rien n'est sélectionné tant que la transition est trop récente
	assert.Empty(t, pendingIDs(t, time.Now().UTC().Add(-time.Minute)))

	future := time.Now().UTC().Add(time.Minute)
	assert.ElementsMatch(t, []string{"bk-life-1", "bk-life-2", "bk-life-3"}, pendingIDs(t, future))

	// Acquittée : n'est plus rejouée ; un second marquage est sans effet
	require.NoError(t, writeRepo.MarkRefundRequested(ctx, "bk-life-1"))
	require.NoError(t, writeRepo.MarkRefundRequested(ctx, "bk-life-1"))
	b1, err := readRepo.GetByID(ctx, "bk-life-1")
	require.NoError(t, err)
	assert.NotNil(t, b1.RefundRequestedAt)

	// Refusée 5 fois : abandonnée
	for i := 1; i <= 5; i++ {
		attempts, err := writeRepo.IncrementRefundRequestAttempts(ctx, "bk-life-2")
		require.NoError(t, err)
		assert.Equal(t, i, attempts)
	}

	assert.Equal(t, []string{"bk-life-3"}, pendingIDs(t, time.Now().UTC().Add(time.Minute)))
}
