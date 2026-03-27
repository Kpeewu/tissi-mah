package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/Kpeewu/tissi-mah/services/booking-service/fixtures"
	"github.com/Kpeewu/tissi-mah/services/booking-service/internal/domain"
	bookingpb "github.com/Kpeewu/tissi-mah/services/booking-service/proto/gen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// =============================================================================
// Health E2E
// =============================================================================

func TestE2E_Health(t *testing.T) {
	ctx := context.Background()

	resp, err := grpcClient.Health(ctx, &bookingpb.HealthRequest{})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "ok", resp.Status)
	assert.Equal(t, "1.0.0", resp.Version)
	assert.Greater(t, resp.Timestamp, int64(0))
}

// =============================================================================
// GetBookingDetails E2E
// =============================================================================

func TestE2E_GetBookingDetails(t *testing.T) {
	ctx := context.Background()

	t.Run("succes - retourne les details d un booking existant", func(t *testing.T) {
		cleanupBookingTables(t, ctx)

		booking := fixtures.NewTestBooking(
			fixtures.WithBookingID("e2e-bk-details-1"),
			fixtures.WithPassengerID("e2e-pax-1"),
			fixtures.WithDriverID("e2e-driver-1"),
			fixtures.WithTotalAmount(5500),
			fixtures.WithBookingStatus(domain.BookingStatusPendingApproval),
		)
		require.NoError(t, fixtures.InsertBooking(ctx, testPool, booking))

		resp, err := grpcClient.GetBookingDetails(ctx, &bookingpb.GetBookingDetailsRequest{
			BookingId: "e2e-bk-details-1",
			UserId:    "e2e-pax-1",
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		require.NotNil(t, resp.Booking)
		assert.Equal(t, "e2e-bk-details-1", resp.Booking.BookingId)
		assert.Equal(t, "e2e-pax-1", resp.Booking.PassengerId)
		assert.Equal(t, "e2e-driver-1", resp.Booking.DriverId)
		assert.Equal(t, int32(5500), resp.Booking.TotalAmount)
		assert.Equal(t, "pendingApproval", resp.Booking.Status)
	})

	t.Run("erreur - booking inexistant", func(t *testing.T) {
		cleanupBookingTables(t, ctx)

		_, err := grpcClient.GetBookingDetails(ctx, &bookingpb.GetBookingDetailsRequest{
			BookingId: "e2e-bk-inexistant",
			UserId:    "e2e-pax-1",
		})

		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.NotFound, st.Code())
	})
}

// =============================================================================
// ApproveBooking E2E
// =============================================================================

func TestE2E_ApproveBooking(t *testing.T) {
	ctx := context.Background()

	t.Run("succes - approuve un booking pendingApproval", func(t *testing.T) {
		cleanupBookingTables(t, ctx)

		booking := fixtures.NewTestBooking(
			fixtures.WithBookingID("e2e-bk-approve"),
			fixtures.WithDriverID("e2e-driver-approve"),
			fixtures.WithBookingStatus(domain.BookingStatusPendingApproval),
		)
		require.NoError(t, fixtures.InsertBookingWithHistory(ctx, testPool, booking))

		resp, err := grpcClient.ApproveBooking(ctx, &bookingpb.ApproveBookingRequest{
			BookingId: "e2e-bk-approve",
			DriverId:  "e2e-driver-approve",
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.True(t, resp.Success)

		// Verifier le statut en DB
		detailResp, err := grpcClient.GetBookingDetails(ctx, &bookingpb.GetBookingDetailsRequest{
			BookingId: "e2e-bk-approve",
			UserId:    "e2e-driver-approve",
		})
		require.NoError(t, err)
		assert.Equal(t, "approved", detailResp.Booking.Status)
		assert.NotEmpty(t, detailResp.Booking.ApprovedAt)
	})

	t.Run("erreur - input invalide", func(t *testing.T) {
		_, err := grpcClient.ApproveBooking(ctx, &bookingpb.ApproveBookingRequest{
			BookingId: "",
			DriverId:  "",
		})

		require.Error(t, err)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})
}

// =============================================================================
// RejectBooking E2E
// =============================================================================

func TestE2E_RejectBooking(t *testing.T) {
	ctx := context.Background()

	t.Run("succes - rejette un booking pendingApproval", func(t *testing.T) {
		cleanupBookingTables(t, ctx)

		booking := fixtures.NewTestBooking(
			fixtures.WithBookingID("e2e-bk-reject"),
			fixtures.WithDriverID("e2e-driver-reject"),
			fixtures.WithBookingStatus(domain.BookingStatusPendingApproval),
		)
		require.NoError(t, fixtures.InsertBookingWithHistory(ctx, testPool, booking))

		resp, err := grpcClient.RejectBooking(ctx, &bookingpb.RejectBookingRequest{
			BookingId: "e2e-bk-reject",
			DriverId:  "e2e-driver-reject",
			Reason:    "Pas disponible",
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.True(t, resp.Success)

		// Verifier le statut en DB
		detailResp, err := grpcClient.GetBookingDetails(ctx, &bookingpb.GetBookingDetailsRequest{
			BookingId: "e2e-bk-reject",
			UserId:    "e2e-driver-reject",
		})
		require.NoError(t, err)
		assert.Equal(t, "rejected", detailResp.Booking.Status)
		assert.NotEmpty(t, detailResp.Booking.RejectedAt)
	})
}

// =============================================================================
// CancelBooking E2E
// =============================================================================

func TestE2E_CancelBooking(t *testing.T) {
	ctx := context.Background()

	t.Run("succes - annule un booking approved", func(t *testing.T) {
		cleanupBookingTables(t, ctx)

		booking := fixtures.NewTestBooking(
			fixtures.WithBookingID("e2e-bk-cancel"),
			fixtures.WithPassengerID("e2e-pax-cancel"),
			fixtures.WithBookingStatus(domain.BookingStatusApproved),
		)
		require.NoError(t, fixtures.InsertBookingWithHistory(ctx, testPool, booking))

		resp, err := grpcClient.CancelBooking(ctx, &bookingpb.CancelBookingRequest{
			BookingId: "e2e-bk-cancel",
			UserId:    "e2e-pax-cancel",
			Reason:    "Changement de plan",
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.True(t, resp.Success)

		// Verifier le statut en DB
		detailResp, err := grpcClient.GetBookingDetails(ctx, &bookingpb.GetBookingDetailsRequest{
			BookingId: "e2e-bk-cancel",
			UserId:    "e2e-pax-cancel",
		})
		require.NoError(t, err)
		assert.Equal(t, "cancelled", detailResp.Booking.Status)
		assert.NotEmpty(t, detailResp.Booking.CancelledAt)
		assert.Equal(t, "e2e-pax-cancel", detailResp.Booking.CancellerId)
	})

	t.Run("erreur - booking inexistant", func(t *testing.T) {
		cleanupBookingTables(t, ctx)

		_, err := grpcClient.CancelBooking(ctx, &bookingpb.CancelBookingRequest{
			BookingId: "e2e-bk-inexistant",
			UserId:    "pax-1",
			Reason:    "test",
		})

		require.Error(t, err)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.NotFound, st.Code())
	})
}

// =============================================================================
// ConfirmPayment E2E
// =============================================================================

func TestE2E_ConfirmPayment(t *testing.T) {
	ctx := context.Background()

	t.Run("succes - confirme le paiement d un booking", func(t *testing.T) {
		cleanupBookingTables(t, ctx)

		booking := fixtures.NewTestBooking(
			fixtures.WithBookingID("e2e-bk-confirm"),
			fixtures.WithBookingStatus(domain.BookingStatusPaymentPending),
		)
		require.NoError(t, fixtures.InsertBookingWithHistory(ctx, testPool, booking))

		resp, err := grpcClient.ConfirmPayment(ctx, &bookingpb.ConfirmPaymentRequest{
			BookingId:     "e2e-bk-confirm",
			TransactionId: "txn-e2e-123",
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.True(t, resp.Success)
		assert.Equal(t, "pendingApproval", resp.Status)

		// Verifier le statut en DB
		detailResp, err := grpcClient.GetBookingDetails(ctx, &bookingpb.GetBookingDetailsRequest{
			BookingId: "e2e-bk-confirm",
			UserId:    booking.PassengerID,
		})
		require.NoError(t, err)
		assert.Equal(t, "pendingApproval", detailResp.Booking.Status)
		assert.NotEmpty(t, detailResp.Booking.PaymentCompletedAt)
	})

	t.Run("erreur - input invalide", func(t *testing.T) {
		_, err := grpcClient.ConfirmPayment(ctx, &bookingpb.ConfirmPaymentRequest{
			BookingId:     "",
			TransactionId: "",
		})

		require.Error(t, err)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})
}

// =============================================================================
// ReportNoShow E2E
// =============================================================================

func TestE2E_ReportNoShow(t *testing.T) {
	ctx := context.Background()

	t.Run("succes - signale un no-show passager", func(t *testing.T) {
		cleanupBookingTables(t, ctx)

		booking := fixtures.NewTestBooking(
			fixtures.WithBookingID("e2e-bk-noshow"),
			fixtures.WithDriverID("e2e-driver-noshow"),
			fixtures.WithBookingStatus(domain.BookingStatusInProgress),
		)
		require.NoError(t, fixtures.InsertBookingWithHistory(ctx, testPool, booking))

		resp, err := grpcClient.ReportNoShow(ctx, &bookingpb.ReportNoShowRequest{
			BookingId:   "e2e-bk-noshow",
			ReporterId:  "e2e-driver-noshow",
			NoShowType:  "passenger",
			Description: "Passager absent au point de ramassage",
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.True(t, resp.Success)

		// Verifier le statut en DB
		detailResp, err := grpcClient.GetBookingDetails(ctx, &bookingpb.GetBookingDetailsRequest{
			BookingId: "e2e-bk-noshow",
			UserId:    "e2e-driver-noshow",
		})
		require.NoError(t, err)
		assert.Equal(t, "noShow", detailResp.Booking.Status)
		assert.Equal(t, "passenger", detailResp.Booking.NoShowType)
		assert.Equal(t, "e2e-driver-noshow", detailResp.Booking.NoShowReportedBy)
	})

	t.Run("erreur - type no-show invalide", func(t *testing.T) {
		_, err := grpcClient.ReportNoShow(ctx, &bookingpb.ReportNoShowRequest{
			BookingId:   "e2e-bk-1",
			ReporterId:  "driver-1",
			NoShowType:  "invalid",
			Description: "test",
		})

		require.Error(t, err)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})
}

// =============================================================================
// Scenario complet E2E : booking lifecycle
// =============================================================================

func TestE2E_BookingLifecycle(t *testing.T) {
	ctx := context.Background()
	cleanupBookingTables(t, ctx)

	// 1. Inserer un booking paymentPending (simule un booking cree avec paiement mobile)
	booking := fixtures.NewTestBooking(
		fixtures.WithBookingID("e2e-lifecycle-bk"),
		fixtures.WithPassengerID("e2e-lifecycle-pax"),
		fixtures.WithDriverID("e2e-lifecycle-driver"),
		fixtures.WithTripID("e2e-lifecycle-trip"),
		fixtures.WithBookingStatus(domain.BookingStatusPaymentPending),
		fixtures.WithPaymentMethod(domain.PaymentMobileMoney),
		fixtures.WithTotalAmount(5500),
	)
	require.NoError(t, fixtures.InsertBookingWithHistory(ctx, testPool, booking))

	// 2. Confirmer le paiement → pendingApproval
	confirmResp, err := grpcClient.ConfirmPayment(ctx, &bookingpb.ConfirmPaymentRequest{
		BookingId:     "e2e-lifecycle-bk",
		TransactionId: "txn-lifecycle-001",
	})
	require.NoError(t, err)
	assert.True(t, confirmResp.Success)
	assert.Equal(t, "pendingApproval", confirmResp.Status)

	// 3. Verifier le statut → pendingApproval
	detailResp, err := grpcClient.GetBookingDetails(ctx, &bookingpb.GetBookingDetailsRequest{
		BookingId: "e2e-lifecycle-bk",
		UserId:    "e2e-lifecycle-pax",
	})
	require.NoError(t, err)
	assert.Equal(t, "pendingApproval", detailResp.Booking.Status)
	assert.NotEmpty(t, detailResp.Booking.PaymentCompletedAt)

	// 4. Approuver le booking → approved
	approveResp, err := grpcClient.ApproveBooking(ctx, &bookingpb.ApproveBookingRequest{
		BookingId: "e2e-lifecycle-bk",
		DriverId:  "e2e-lifecycle-driver",
	})
	require.NoError(t, err)
	assert.True(t, approveResp.Success)

	// 5. Verifier le statut → approved
	detailResp, err = grpcClient.GetBookingDetails(ctx, &bookingpb.GetBookingDetailsRequest{
		BookingId: "e2e-lifecycle-bk",
		UserId:    "e2e-lifecycle-driver",
	})
	require.NoError(t, err)
	assert.Equal(t, "approved", detailResp.Booking.Status)
	assert.NotEmpty(t, detailResp.Booking.ApprovedAt)

	// 6. Annuler le booking → cancelled
	cancelResp, err := grpcClient.CancelBooking(ctx, &bookingpb.CancelBookingRequest{
		BookingId: "e2e-lifecycle-bk",
		UserId:    "e2e-lifecycle-pax",
		Reason:    "Je ne peux plus voyager",
	})
	require.NoError(t, err)
	assert.True(t, cancelResp.Success)

	// 7. Verifier le statut final → cancelled avec tous les timestamps
	detailResp, err = grpcClient.GetBookingDetails(ctx, &bookingpb.GetBookingDetailsRequest{
		BookingId: "e2e-lifecycle-bk",
		UserId:    "e2e-lifecycle-pax",
	})
	require.NoError(t, err)
	assert.Equal(t, "cancelled", detailResp.Booking.Status)
	assert.NotEmpty(t, detailResp.Booking.PaymentCompletedAt)
	assert.NotEmpty(t, detailResp.Booking.ApprovedAt)
	assert.NotEmpty(t, detailResp.Booking.CancelledAt)
	assert.Equal(t, "e2e-lifecycle-pax", detailResp.Booking.CancellerId)

	// 8. Verifier l'historique des statuts
	assert.GreaterOrEqual(t, len(detailResp.Booking.History), 1)

	// 9. Verifier qu'un booking annule ne peut plus etre approuve
	_, err = grpcClient.ApproveBooking(ctx, &bookingpb.ApproveBookingRequest{
		BookingId: "e2e-lifecycle-bk",
		DriverId:  "e2e-lifecycle-driver",
	})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.FailedPrecondition, st.Code())
}

// =============================================================================
// StartBookingsForWaypoint E2E
// =============================================================================

func TestE2E_StartBookingsForWaypoint(t *testing.T) {
	ctx := context.Background()

	t.Run("succes - demarre les bookings d un waypoint", func(t *testing.T) {
		cleanupBookingTables(t, ctx)

		now := time.Now().UTC()
		booking := fixtures.NewTestBooking(
			fixtures.WithBookingID("e2e-bk-start"),
			fixtures.WithTripID("e2e-trip-start"),
			fixtures.WithPickupWaypointID("e2e-wp-pickup"),
			fixtures.WithBookingStatus(domain.BookingStatusApproved),
			fixtures.WithApprovedAt(now),
		)
		require.NoError(t, fixtures.InsertBookingWithHistory(ctx, testPool, booking))

		resp, err := grpcClient.StartBookingsForWaypoint(ctx, &bookingpb.StartBookingsForWaypointRequest{
			TripId:     "e2e-trip-start",
			WaypointId: "e2e-wp-pickup",
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.True(t, resp.Success)
		assert.Equal(t, int32(1), resp.BookingsCount)

		// Verifier le statut → inProgress
		detailResp, err := grpcClient.GetBookingDetails(ctx, &bookingpb.GetBookingDetailsRequest{
			BookingId: "e2e-bk-start",
			UserId:    booking.PassengerID,
		})
		require.NoError(t, err)
		assert.Equal(t, "inProgress", detailResp.Booking.Status)
	})

	t.Run("erreur - input invalide", func(t *testing.T) {
		_, err := grpcClient.StartBookingsForWaypoint(ctx, &bookingpb.StartBookingsForWaypointRequest{
			TripId:     "",
			WaypointId: "",
		})

		require.Error(t, err)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})
}

// =============================================================================
// CompleteBookingsForWaypoint E2E
// =============================================================================

func TestE2E_CompleteBookingsForWaypoint(t *testing.T) {
	ctx := context.Background()

	t.Run("succes - complete les bookings d un waypoint", func(t *testing.T) {
		cleanupBookingTables(t, ctx)

		booking := fixtures.NewTestBooking(
			fixtures.WithBookingID("e2e-bk-complete"),
			fixtures.WithTripID("e2e-trip-complete"),
			fixtures.WithDropoffWaypointID("e2e-wp-dropoff"),
			fixtures.WithBookingStatus(domain.BookingStatusInProgress),
		)
		require.NoError(t, fixtures.InsertBookingWithHistory(ctx, testPool, booking))

		resp, err := grpcClient.CompleteBookingsForWaypoint(ctx, &bookingpb.CompleteBookingsForWaypointRequest{
			TripId:     "e2e-trip-complete",
			WaypointId: "e2e-wp-dropoff",
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.True(t, resp.Success)
		assert.Equal(t, int32(1), resp.BookingsCount)

		// Verifier le statut → completed
		detailResp, err := grpcClient.GetBookingDetails(ctx, &bookingpb.GetBookingDetailsRequest{
			BookingId: "e2e-bk-complete",
			UserId:    booking.PassengerID,
		})
		require.NoError(t, err)
		assert.Equal(t, "completed", detailResp.Booking.Status)
		assert.NotEmpty(t, detailResp.Booking.CompletedAt)
	})
}
