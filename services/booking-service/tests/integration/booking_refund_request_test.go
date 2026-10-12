package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Kpeewu/tissi-mah/services/booking-service/fixtures"
	"github.com/Kpeewu/tissi-mah/services/booking-service/internal/domain"
	bookingErrors "github.com/Kpeewu/tissi-mah/services/booking-service/pkg/errors"
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
		bookings, err := readRepo.GetBookingsPendingRefundRequest(ctx, updatedBefore, 5, 100, domain.NoShowContestWindow)
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

// =============================================================================
// No-show : remboursement bloqué pendant le délai de contestation
// =============================================================================

func TestBookingRepository_NoShowRefundWaitsForContestWindow(t *testing.T) {
	ctx := context.Background()
	writeRepo := newTestBookingWriteRepository()
	readRepo := newTestBookingReadRepository()
	now := time.Now().UTC()

	insertNoShow := func(t *testing.T, id string, reportedAt time.Time, opts ...fixtures.BookingOption) {
		t.Helper()
		opts = append([]fixtures.BookingOption{
			fixtures.WithBookingID(id),
			fixtures.WithPassengerID("pax-" + id),
			fixtures.WithDriverID("driver-" + id),
			fixtures.WithNoShowReported("driver", "pax-"+id, reportedAt),
			fixtures.WithRefundReason(domain.RefundReasonNoShowDriver),
		}, opts...)
		b := fixtures.NewTestBookingWithPayment(opts...)
		require.NoError(t, fixtures.InsertBookingWithHistory(ctx, testPool, b))
	}

	pendingIDs := func(t *testing.T) []string {
		t.Helper()
		bookings, err := readRepo.GetBookingsPendingRefundRequest(
			ctx, time.Now().UTC().Add(time.Minute), 5, 100, domain.NoShowContestWindow)
		require.NoError(t, err)
		ids := make([]string, 0, len(bookings))
		for _, b := range bookings {
			ids = append(ids, b.BookingID)
		}
		return ids
	}

	t.Run("selection par le worker", func(t *testing.T) {
		cleanupBookingTables(t, ctx)
		insertNoShow(t, "bk-ns-recent", now.Add(-time.Hour))
		insertNoShow(t, "bk-ns-expired", now.Add(-49*time.Hour))
		insertNoShow(t, "bk-ns-contested", now.Add(-49*time.Hour),
			fixtures.WithNoShowContested("driver-bk-ns-contested", now.Add(-48*time.Hour)))

		// Délai de contestation en cours ou contestation non tranchée : bloqué
		assert.Equal(t, []string{"bk-ns-expired"}, pendingIDs(t))
	})

	t.Run("contestation", func(t *testing.T) {
		cleanupBookingTables(t, ctx)
		insertNoShow(t, "bk-ns-open", now.Add(-time.Hour))
		insertNoShow(t, "bk-ns-closed", now.Add(-49*time.Hour))
		approved := fixtures.NewTestBookingWithPayment(
			fixtures.WithBookingID("bk-ns-approved"),
			fixtures.WithBookingStatus(domain.BookingStatusApproved),
		)
		require.NoError(t, fixtures.InsertBookingWithHistory(ctx, testPool, approved))

		require.NoError(t, writeRepo.ContestNoShow(ctx, "bk-ns-open", "driver-bk-ns-open", "J'étais là",
			domain.NoShowContestWindow))
		b, err := readRepo.GetByID(ctx, "bk-ns-open")
		require.NoError(t, err)
		require.NotNil(t, b.NoShowContestedAt)
		assert.Equal(t, "driver-bk-ns-open", *b.NoShowContestedBy)
		assert.Equal(t, "J'étais là", *b.NoShowContestReason)

		err = writeRepo.ContestNoShow(ctx, "bk-ns-open", "driver-bk-ns-open", "Encore", domain.NoShowContestWindow)
		assert.ErrorIs(t, err, bookingErrors.ErrorNoShowAlreadyContested)

		err = writeRepo.ContestNoShow(ctx, "bk-ns-closed", "driver-bk-ns-closed", "Trop tard",
			domain.NoShowContestWindow)
		assert.ErrorIs(t, err, bookingErrors.ErrorNoShowContestClosed)

		err = writeRepo.ContestNoShow(ctx, "bk-ns-approved", "x", "Pas un no-show", domain.NoShowContestWindow)
		assert.ErrorIs(t, err, bookingErrors.ErrorInvalidStatusTransition)

		// Contestée : le remboursement reste bloqué ; délai expiré sans contestation : transmis
		assert.Equal(t, []string{"bk-ns-closed"}, pendingIDs(t))
	})

	t.Run("remboursement deja transmis : contestation close", func(t *testing.T) {
		cleanupBookingTables(t, ctx)
		insertNoShow(t, "bk-ns-sent", now.Add(-time.Hour))
		require.NoError(t, writeRepo.MarkRefundRequested(ctx, "bk-ns-sent"))

		err := writeRepo.ContestNoShow(ctx, "bk-ns-sent", "driver-bk-ns-sent", "J'étais là", domain.NoShowContestWindow)
		assert.ErrorIs(t, err, bookingErrors.ErrorNoShowContestClosed)
	})
}
