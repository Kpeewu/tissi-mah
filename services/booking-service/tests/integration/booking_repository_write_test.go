package integration

import (
	"context"
	"testing"
	"time"

	"github.com/Kpeewu/tissi-mah/services/booking-service/fixtures"
	"github.com/Kpeewu/tissi-mah/services/booking-service/internal/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Create
// =============================================================================

func TestBookingWriteRepository_Create(t *testing.T) {
	ctx := context.Background()
	writeRepo := newTestBookingWriteRepository()
	readRepo := newTestBookingReadRepository()

	t.Run("cree un booking avec segments et historique", func(t *testing.T) {
		cleanupBookingTables(t, ctx)

		booking := fixtures.NewTestBooking(
			fixtures.WithBookingID("bk-create-1"),
			fixtures.WithPassengerID("pax-create"),
			fixtures.WithTripID("trip-create"),
			fixtures.WithBookingStatus(domain.BookingStatusPendingApproval),
		)

		now := time.Now().UTC()
		segment := &domain.Segment{
			SegmentID:          uuid.New().String(),
			BookingID:          "bk-create-1",
			PickupWaypointID:   booking.PickupWaypointID,
			DropoffWaypointID:  booking.DropoffWaypointID,
			PickupLocationName: "Lome",
			PickupCity:         "Lome",
			PickupScheduledAt:  &now,
			DropoffLocationName: "Kpalime",
			DropoffCity:         "Kpalime",
			DropoffScheduledAt:  &now,
		}

		history := &domain.StatusHistoryEntry{
			HistoryID:     uuid.New().String(),
			BookingID:     "bk-create-1",
			PreviousStatus: "",
			NewStatus:      string(domain.BookingStatusPendingApproval),
			ChangedBy:      "pax-create",
			ChangedByType:  "passenger",
		}

		err := writeRepo.Create(ctx, booking, []*domain.Segment{segment}, history)
		require.NoError(t, err)

		// Verifier en lecture
		result, err := readRepo.GetByID(ctx, "bk-create-1")
		require.NoError(t, err)
		assert.Equal(t, "bk-create-1", result.BookingID)
		assert.Equal(t, "pax-create", result.PassengerID)
		assert.Equal(t, domain.BookingStatusPendingApproval, result.Status)
	})
}

// =============================================================================
// Approve
// =============================================================================

func TestBookingWriteRepository_Approve(t *testing.T) {
	ctx := context.Background()
	writeRepo := newTestBookingWriteRepository()
	readRepo := newTestBookingReadRepository()

	t.Run("approuve un booking pendingApproval", func(t *testing.T) {
		cleanupBookingTables(t, ctx)

		booking := fixtures.NewTestBooking(
			fixtures.WithBookingID("bk-approve-1"),
			fixtures.WithDriverID("driver-approve"),
			fixtures.WithBookingStatus(domain.BookingStatusPendingApproval),
		)
		require.NoError(t, fixtures.InsertBookingWithHistory(ctx, testPool, booking))

		err := writeRepo.Approve(ctx, "bk-approve-1", "driver-approve")
		require.NoError(t, err)

		result, err := readRepo.GetByID(ctx, "bk-approve-1")
		require.NoError(t, err)
		assert.Equal(t, domain.BookingStatusApproved, result.Status)
		assert.NotNil(t, result.ApprovedAt)
	})
}

// =============================================================================
// Reject
// =============================================================================

func TestBookingWriteRepository_Reject(t *testing.T) {
	ctx := context.Background()
	writeRepo := newTestBookingWriteRepository()
	readRepo := newTestBookingReadRepository()

	t.Run("rejette un booking pendingApproval", func(t *testing.T) {
		cleanupBookingTables(t, ctx)

		booking := fixtures.NewTestBooking(
			fixtures.WithBookingID("bk-reject-1"),
			fixtures.WithDriverID("driver-reject"),
			fixtures.WithBookingStatus(domain.BookingStatusPendingApproval),
		)
		require.NoError(t, fixtures.InsertBookingWithHistory(ctx, testPool, booking))

		err := writeRepo.Reject(ctx, "bk-reject-1", "driver-reject", "Pas disponible")
		require.NoError(t, err)

		result, err := readRepo.GetByID(ctx, "bk-reject-1")
		require.NoError(t, err)
		assert.Equal(t, domain.BookingStatusRejected, result.Status)
		assert.NotNil(t, result.RejectedAt)
	})
}

// =============================================================================
// Cancel
// =============================================================================

func TestBookingWriteRepository_Cancel(t *testing.T) {
	ctx := context.Background()
	writeRepo := newTestBookingWriteRepository()
	readRepo := newTestBookingReadRepository()

	t.Run("annule un booking", func(t *testing.T) {
		cleanupBookingTables(t, ctx)

		booking := fixtures.NewTestBooking(
			fixtures.WithBookingID("bk-cancel-1"),
			fixtures.WithPassengerID("pax-cancel-1"),
			fixtures.WithBookingStatus(domain.BookingStatusApproved),
		)
		require.NoError(t, fixtures.InsertBookingWithHistory(ctx, testPool, booking))

		err := writeRepo.Cancel(ctx, "bk-cancel-1", "pax-cancel-1", "Changement de plan")
		require.NoError(t, err)

		result, err := readRepo.GetByID(ctx, "bk-cancel-1")
		require.NoError(t, err)
		assert.Equal(t, domain.BookingStatusCancelled, result.Status)
		assert.NotNil(t, result.CancelledAt)
		require.NotNil(t, result.CancellerID)
		assert.Equal(t, "pax-cancel-1", *result.CancellerID)
	})
}

// =============================================================================
// ConfirmPayment
// =============================================================================

func TestBookingWriteRepository_ConfirmPayment(t *testing.T) {
	ctx := context.Background()
	writeRepo := newTestBookingWriteRepository()
	readRepo := newTestBookingReadRepository()

	t.Run("confirme le paiement d un booking", func(t *testing.T) {
		cleanupBookingTables(t, ctx)

		booking := fixtures.NewTestBooking(
			fixtures.WithBookingID("bk-confirm-1"),
			fixtures.WithBookingStatus(domain.BookingStatusPaymentPending),
		)
		require.NoError(t, fixtures.InsertBookingWithHistory(ctx, testPool, booking))

		err := writeRepo.ConfirmPayment(ctx, "bk-confirm-1", "txn-abc123")
		require.NoError(t, err)

		result, err := readRepo.GetByID(ctx, "bk-confirm-1")
		require.NoError(t, err)
		assert.Equal(t, domain.BookingStatusPendingApproval, result.Status)
		assert.NotNil(t, result.PaymentCompletedAt)
	})
}

// =============================================================================
// MarkPaymentReleased
// =============================================================================

func TestBookingWriteRepository_MarkPaymentReleased(t *testing.T) {
	ctx := context.Background()
	writeRepo := newTestBookingWriteRepository()
	readRepo := newTestBookingReadRepository()

	t.Run("marque le paiement comme libere", func(t *testing.T) {
		cleanupBookingTables(t, ctx)

		booking := fixtures.NewTestBooking(
			fixtures.WithBookingID("bk-release-1"),
			fixtures.WithBookingStatus(domain.BookingStatusCompleted),
		)
		require.NoError(t, fixtures.InsertBooking(ctx, testPool, booking))

		err := writeRepo.MarkPaymentReleased(ctx, "bk-release-1")
		require.NoError(t, err)

		result, err := readRepo.GetByID(ctx, "bk-release-1")
		require.NoError(t, err)
		assert.NotNil(t, result.PaymentReleasedAt)
	})
}

// =============================================================================
// ReportNoShow
// =============================================================================

func TestBookingWriteRepository_ReportNoShow(t *testing.T) {
	ctx := context.Background()
	writeRepo := newTestBookingWriteRepository()
	readRepo := newTestBookingReadRepository()

	t.Run("signale un no-show passager", func(t *testing.T) {
		cleanupBookingTables(t, ctx)

		booking := fixtures.NewTestBooking(
			fixtures.WithBookingID("bk-noshow-1"),
			fixtures.WithBookingStatus(domain.BookingStatusInProgress),
		)
		require.NoError(t, fixtures.InsertBookingWithHistory(ctx, testPool, booking))

		err := writeRepo.ReportNoShow(ctx, "bk-noshow-1", "driver-1", "passenger", "Passager absent au point de ramassage")
		require.NoError(t, err)

		result, err := readRepo.GetByID(ctx, "bk-noshow-1")
		require.NoError(t, err)
		assert.Equal(t, domain.BookingStatusNoShow, result.Status)
		require.NotNil(t, result.NoShowType)
		assert.Equal(t, "passenger", *result.NoShowType)
		require.NotNil(t, result.NoShowReportedBy)
		assert.Equal(t, "driver-1", *result.NoShowReportedBy)
	})
}
