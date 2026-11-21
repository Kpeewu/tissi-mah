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
	ctx := ctxWithUID("e2e-test-user")

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
	ctx := ctxWithUID("e2e-test-user")

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

		resp, err := grpcClient.GetBookingDetails(ctxWithUID("e2e-pax-1"), &bookingpb.GetBookingDetailsRequest{
			BookingId: "e2e-bk-details-1",
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

		_, err := grpcClient.GetBookingDetails(ctxWithUID("e2e-pax-1"), &bookingpb.GetBookingDetailsRequest{
			BookingId: "e2e-bk-inexistant",
		})

		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.NotFound, st.Code())
	})

	t.Run("erreur - un tiers ne peut pas lire la reservation, meme avec UserId vide", func(t *testing.T) {
		cleanupBookingTables(t, ctx)

		booking := fixtures.NewTestBooking(
			fixtures.WithBookingID("e2e-bk-details-tiers"),
			fixtures.WithPassengerID("e2e-pax-1"),
			fixtures.WithDriverID("e2e-driver-1"),
		)
		require.NoError(t, fixtures.InsertBooking(ctx, testPool, booking))

		_, err := grpcClient.GetBookingDetails(ctxWithUID("e2e-intrus"), &bookingpb.GetBookingDetailsRequest{
			BookingId: "e2e-bk-details-tiers",
			UserId:    "",
		})

		require.Error(t, err)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.PermissionDenied, st.Code())
	})

	t.Run("erreur - appel sans identite refuse, meme avec UserId du passager", func(t *testing.T) {
		cleanupBookingTables(t, ctx)

		booking := fixtures.NewTestBooking(fixtures.WithBookingID("e2e-bk-details-internal"))
		require.NoError(t, fixtures.InsertBooking(ctx, testPool, booking))

		_, err := grpcClient.GetBookingDetails(context.Background(), &bookingpb.GetBookingDetailsRequest{
			BookingId: "e2e-bk-details-internal",
			UserId:    booking.PassengerID,
		})

		require.Error(t, err)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.Unauthenticated, st.Code())
	})

	t.Run("succes - route interne sans identite", func(t *testing.T) {
		resp, err := grpcClient.GetBookingDetailsInternal(context.Background(), &bookingpb.GetBookingDetailsRequest{
			BookingId: "e2e-bk-details-internal",
		})

		require.NoError(t, err)
		require.NotNil(t, resp.Booking)
		assert.Equal(t, "e2e-bk-details-internal", resp.Booking.BookingId)
	})

	t.Run("erreur - utilisateur inconnu de user-service", func(t *testing.T) {
		_, err := grpcClient.GetBookingDetails(ctxWithUID(unknownFirebaseUID), &bookingpb.GetBookingDetailsRequest{
			BookingId: "e2e-bk-details-internal",
		})

		require.Error(t, err)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.PermissionDenied, st.Code())
	})
}

// =============================================================================
// ApproveBooking E2E
// =============================================================================

func TestE2E_ApproveBooking(t *testing.T) {
	ctx := ctxWithUID("e2e-test-user")

	t.Run("succes - approuve un booking pendingApproval", func(t *testing.T) {
		cleanupBookingTables(t, ctx)

		booking := fixtures.NewTestBooking(
			fixtures.WithBookingID("e2e-bk-approve"),
			fixtures.WithDriverID("e2e-driver-approve"),
			fixtures.WithBookingStatus(domain.BookingStatusPendingApproval),
		)
		require.NoError(t, fixtures.InsertBookingWithHistory(ctx, testPool, booking))

		resp, err := grpcClient.ApproveBooking(ctxWithUID("e2e-driver-approve"), &bookingpb.ApproveBookingRequest{
			BookingId: "e2e-bk-approve",
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.True(t, resp.Success)

		// Verifier le statut en DB
		detailResp, err := grpcClient.GetBookingDetails(ctxWithUID("e2e-driver-approve"), &bookingpb.GetBookingDetailsRequest{
			BookingId: "e2e-bk-approve",
		})
		require.NoError(t, err)
		assert.Equal(t, "approved", detailResp.Booking.Status)
		assert.NotEmpty(t, detailResp.Booking.ApprovedAt)
	})

	t.Run("erreur - DriverId du body ignore, l'appelant n'est pas le chauffeur", func(t *testing.T) {
		cleanupBookingTables(t, ctx)

		booking := fixtures.NewTestBooking(
			fixtures.WithBookingID("e2e-bk-approve-usurp"),
			fixtures.WithPassengerID("e2e-pax-usurp"),
			fixtures.WithDriverID("e2e-driver-approve"),
			fixtures.WithBookingStatus(domain.BookingStatusPendingApproval),
		)
		require.NoError(t, fixtures.InsertBookingWithHistory(ctx, testPool, booking))

		_, err := grpcClient.ApproveBooking(ctxWithUID("e2e-pax-usurp"), &bookingpb.ApproveBookingRequest{
			BookingId: "e2e-bk-approve-usurp",
			DriverId:  "e2e-driver-approve",
		})

		require.Error(t, err)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.PermissionDenied, st.Code())
	})

	t.Run("erreur - input invalide", func(t *testing.T) {
		_, err := grpcClient.ApproveBooking(ctxWithUID("e2e-driver-approve"), &bookingpb.ApproveBookingRequest{
			BookingId: "",
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
	ctx := ctxWithUID("e2e-test-user")

	t.Run("succes - rejette un booking pendingApproval", func(t *testing.T) {
		cleanupBookingTables(t, ctx)

		booking := fixtures.NewTestBooking(
			fixtures.WithBookingID("e2e-bk-reject"),
			fixtures.WithDriverID("e2e-driver-reject"),
			fixtures.WithBookingStatus(domain.BookingStatusPendingApproval),
		)
		require.NoError(t, fixtures.InsertBookingWithHistory(ctx, testPool, booking))

		resp, err := grpcClient.RejectBooking(ctxWithUID("e2e-driver-reject"), &bookingpb.RejectBookingRequest{
			BookingId: "e2e-bk-reject",
			Reason:    "Pas disponible",
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.True(t, resp.Success)

		// Verifier le statut en DB
		detailResp, err := grpcClient.GetBookingDetails(ctxWithUID("e2e-driver-reject"), &bookingpb.GetBookingDetailsRequest{
			BookingId: "e2e-bk-reject",
		})
		require.NoError(t, err)
		assert.Equal(t, "rejected", detailResp.Booking.Status)
		assert.NotEmpty(t, detailResp.Booking.RejectedAt)
	})

	t.Run("erreur - DriverId du body ignore, l'appelant n'est pas le chauffeur", func(t *testing.T) {
		cleanupBookingTables(t, ctx)

		booking := fixtures.NewTestBooking(
			fixtures.WithBookingID("e2e-bk-reject-usurp"),
			fixtures.WithDriverID("e2e-driver-reject"),
			fixtures.WithBookingStatus(domain.BookingStatusPendingApproval),
		)
		require.NoError(t, fixtures.InsertBookingWithHistory(ctx, testPool, booking))

		_, err := grpcClient.RejectBooking(ctxWithUID("e2e-intrus"), &bookingpb.RejectBookingRequest{
			BookingId: "e2e-bk-reject-usurp",
			DriverId:  "e2e-driver-reject",
			Reason:    "usurpation",
		})

		require.Error(t, err)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.PermissionDenied, st.Code())
	})
}

// =============================================================================
// CancelBooking E2E
// =============================================================================

func TestE2E_CancelBooking(t *testing.T) {
	ctx := ctxWithUID("e2e-test-user")

	t.Run("succes - annule un booking approved", func(t *testing.T) {
		cleanupBookingTables(t, ctx)

		booking := fixtures.NewTestBooking(
			fixtures.WithBookingID("e2e-bk-cancel"),
			fixtures.WithPassengerID("e2e-pax-cancel"),
			fixtures.WithBookingStatus(domain.BookingStatusApproved),
		)
		require.NoError(t, fixtures.InsertBookingWithHistory(ctx, testPool, booking))

		resp, err := grpcClient.CancelBooking(ctxWithUID("e2e-pax-cancel"), &bookingpb.CancelBookingRequest{
			BookingId: "e2e-bk-cancel",
			Reason:    "Changement de plan",
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.True(t, resp.Success)

		// Verifier le statut en DB
		detailResp, err := grpcClient.GetBookingDetails(ctxWithUID("e2e-pax-cancel"), &bookingpb.GetBookingDetailsRequest{
			BookingId: "e2e-bk-cancel",
		})
		require.NoError(t, err)
		assert.Equal(t, "cancelled", detailResp.Booking.Status)
		assert.NotEmpty(t, detailResp.Booking.CancelledAt)
		assert.Equal(t, "e2e-pax-cancel", detailResp.Booking.CancellerId)
	})

	t.Run("securite - passager avec UserId du chauffeur : annulation imputee au passager", func(t *testing.T) {
		cleanupBookingTables(t, ctx)

		booking := fixtures.NewTestBookingWithPayment(
			fixtures.WithBookingID("e2e-bk-cancel-usurp"),
			fixtures.WithPassengerID("e2e-pax-usurp"),
			fixtures.WithDriverID("e2e-driver-usurp"),
			fixtures.WithBookingStatus(domain.BookingStatusApproved),
		)
		require.NoError(t, fixtures.InsertBookingWithHistory(ctx, testPool, booking))

		_, err := grpcClient.CancelBooking(ctxWithUID("e2e-pax-usurp"), &bookingpb.CancelBookingRequest{
			BookingId: "e2e-bk-cancel-usurp",
			UserId:    "e2e-driver-usurp",
			Reason:    "Je me fais passer pour le chauffeur",
		})
		require.NoError(t, err)

		var cancellerID, refundReason string
		require.NoError(t, testPool.QueryRow(ctx,
			`SELECT canceller_id, refund_reason FROM bookings WHERE booking_id = $1`, "e2e-bk-cancel-usurp",
		).Scan(&cancellerID, &refundReason))
		assert.Equal(t, "e2e-pax-usurp", cancellerID)
		assert.Equal(t, domain.RefundReasonCancelledByPassenger, refundReason)
	})

	t.Run("erreur - un tiers ne peut pas annuler", func(t *testing.T) {
		cleanupBookingTables(t, ctx)

		booking := fixtures.NewTestBooking(
			fixtures.WithBookingID("e2e-bk-cancel-tiers"),
			fixtures.WithBookingStatus(domain.BookingStatusApproved),
		)
		require.NoError(t, fixtures.InsertBookingWithHistory(ctx, testPool, booking))

		_, err := grpcClient.CancelBooking(ctxWithUID("e2e-intrus"), &bookingpb.CancelBookingRequest{
			BookingId: "e2e-bk-cancel-tiers",
			UserId:    booking.PassengerID,
			Reason:    "test",
		})

		require.Error(t, err)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.PermissionDenied, st.Code())
	})

	t.Run("erreur - booking inexistant", func(t *testing.T) {
		cleanupBookingTables(t, ctx)

		_, err := grpcClient.CancelBooking(ctxWithUID("pax-1"), &bookingpb.CancelBookingRequest{
			BookingId: "e2e-bk-inexistant",
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
	ctx := ctxWithUID("e2e-test-user")

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
		detailResp, err := grpcClient.GetBookingDetails(ctxWithUID(booking.PassengerID), &bookingpb.GetBookingDetailsRequest{
			BookingId: "e2e-bk-confirm",
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
// ContestNoShow E2E
// =============================================================================

func TestE2E_ContestNoShow(t *testing.T) {
	ctx := ctxWithUID("e2e-test-user")
	cleanupBookingTables(t, ctx)

	booking := fixtures.NewTestBookingWithPayment(
		fixtures.WithBookingID("e2e-bk-contest"),
		fixtures.WithPassengerID("e2e-passenger-contest"),
		fixtures.WithDriverID("e2e-driver-contest"),
		fixtures.WithNoShowReported("passenger", "e2e-driver-contest", time.Now().UTC().Add(-time.Hour)),
		fixtures.WithRefundReason(domain.RefundReasonNoShowPassenger),
	)
	require.NoError(t, fixtures.InsertBookingWithHistory(ctx, testPool, booking))

	contest := func(uid string) error {
		_, err := grpcClient.ContestNoShow(ctxWithUID(uid), &bookingpb.ContestNoShowRequest{
			BookingId: "e2e-bk-contest",
			Reason:    "J'attendais au point de rendez-vous",
		})
		return err
	}

	t.Run("erreur - le declarant ne peut pas contester", func(t *testing.T) {
		st, _ := status.FromError(contest("e2e-driver-contest"))
		assert.Equal(t, codes.PermissionDenied, st.Code())
	})

	t.Run("succes - le passager accuse conteste", func(t *testing.T) {
		require.NoError(t, contest("e2e-passenger-contest"))

		var contestedBy string
		require.NoError(t, testPool.QueryRow(ctx,
			`SELECT no_show_contested_by FROM bookings WHERE booking_id = 'e2e-bk-contest'`,
		).Scan(&contestedBy))
		assert.Equal(t, "e2e-passenger-contest", contestedBy)
	})

	t.Run("erreur - seconde contestation", func(t *testing.T) {
		st, _ := status.FromError(contest("e2e-passenger-contest"))
		assert.Equal(t, codes.FailedPrecondition, st.Code())
	})
}

// =============================================================================
// ResolveNoShowContest E2E (support)
// =============================================================================

func TestE2E_ResolveNoShowContest(t *testing.T) {
	ctx := ctxWithUID("e2e-test-user")
	cleanupBookingTables(t, ctx)

	now := time.Now().UTC()
	booking := fixtures.NewTestBookingWithPayment(
		fixtures.WithBookingID("e2e-bk-resolve"),
		fixtures.WithPassengerID("e2e-passenger-resolve"),
		fixtures.WithDriverID("e2e-driver-resolve"),
		fixtures.WithNoShowReported("driver", "e2e-passenger-resolve", now.Add(-2*time.Hour)),
		fixtures.WithRefundReason(domain.RefundReasonNoShowDriver),
		fixtures.WithNoShowContested("e2e-driver-resolve", now.Add(-time.Hour)),
	)
	require.NoError(t, fixtures.InsertBookingWithHistory(ctx, testPool, booking))

	req := &bookingpb.ResolveNoShowContestRequest{
		BookingId:    "e2e-bk-resolve",
		RefundReason: domain.RefundReasonNoShowPassenger,
		Note:         "Chauffeur présent au point de rendez-vous",
	}

	t.Run("erreur - un utilisateur ne peut pas trancher", func(t *testing.T) {
		_, err := grpcClient.ResolveNoShowContest(ctxWithUID("e2e-driver-resolve"), req)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.Unauthenticated, st.Code())
	})

	t.Run("succes - le support tranche", func(t *testing.T) {
		resp, err := grpcClient.ResolveNoShowContest(ctxWithSupportUID("e2e-support-1"), req)
		require.NoError(t, err)
		assert.True(t, resp.Success)

		detail, err := grpcClient.GetBookingDetailAdmin(ctxWithSupportUID("e2e-support-1"),
			&bookingpb.GetBookingDetailAdminRequest{BookingId: "e2e-bk-resolve"})
		require.NoError(t, err)
		assert.Equal(t, domain.RefundReasonNoShowPassenger, detail.Booking.NoShowResolution)
		assert.Equal(t, "e2e-driver-resolve", detail.Booking.NoShowContestedBy)
		assert.Equal(t, "e2e-support-1", detail.NoShowResolvedBy)
	})

	t.Run("erreur - decision deja prise", func(t *testing.T) {
		_, err := grpcClient.ResolveNoShowContest(ctxWithSupportUID("e2e-support-2"), req)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.FailedPrecondition, st.Code())
	})
}

// =============================================================================
// ReportNoShow E2E
// =============================================================================

func TestE2E_ReportNoShow(t *testing.T) {
	ctx := ctxWithUID("e2e-test-user")

	t.Run("succes - signale un no-show passager", func(t *testing.T) {
		cleanupBookingTables(t, ctx)

		booking := fixtures.NewTestBooking(
			fixtures.WithBookingID("e2e-bk-noshow"),
			fixtures.WithDriverID("e2e-driver-noshow"),
			fixtures.WithTripID(noShowTripID),
			fixtures.WithPickupWaypointID(noShowWaypointID),
			fixtures.WithBookingStatus(domain.BookingStatusInProgress),
		)
		require.NoError(t, fixtures.InsertBookingWithHistory(ctx, testPool, booking))

		resp, err := grpcClient.ReportNoShow(ctxWithUID("e2e-driver-noshow"), &bookingpb.ReportNoShowRequest{
			BookingId:   "e2e-bk-noshow",
			NoShowType:  "passenger",
			Description: "Passager absent au point de ramassage",
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.True(t, resp.Success)

		// Verifier le statut en DB
		detailResp, err := grpcClient.GetBookingDetails(ctxWithUID("e2e-driver-noshow"), &bookingpb.GetBookingDetailsRequest{
			BookingId: "e2e-bk-noshow",
		})
		require.NoError(t, err)
		assert.Equal(t, "noShow", detailResp.Booking.Status)
		assert.Equal(t, "passenger", detailResp.Booking.NoShowType)
		assert.Equal(t, "e2e-driver-noshow", detailResp.Booking.NoShowReportedBy)
	})

	t.Run("erreur - chauffeur absent declare alors que le passager est a bord", func(t *testing.T) {
		cleanupBookingTables(t, ctx)

		booking := fixtures.NewTestBooking(
			fixtures.WithBookingID("e2e-bk-noshow-aboard"),
			fixtures.WithPassengerID("e2e-passenger-aboard"),
			fixtures.WithTripID(noShowTripID),
			fixtures.WithPickupWaypointID(noShowWaypointID),
			fixtures.WithBookingStatus(domain.BookingStatusInProgress),
		)
		require.NoError(t, fixtures.InsertBookingWithHistory(ctx, testPool, booking))

		_, err := grpcClient.ReportNoShow(ctxWithUID("e2e-passenger-aboard"), &bookingpb.ReportNoShowRequest{
			BookingId:  "e2e-bk-noshow-aboard",
			NoShowType: "driver",
		})

		require.Error(t, err)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.FailedPrecondition, st.Code())

		detailResp, err := grpcClient.GetBookingDetails(ctxWithUID("e2e-passenger-aboard"),
			&bookingpb.GetBookingDetailsRequest{BookingId: "e2e-bk-noshow-aboard"})
		require.NoError(t, err)
		assert.Equal(t, "inProgress", detailResp.Booking.Status)
	})

	t.Run("succes - chauffeur absent avant la prise en charge", func(t *testing.T) {
		cleanupBookingTables(t, ctx)

		booking := fixtures.NewTestBooking(
			fixtures.WithBookingID("e2e-bk-noshow-driver"),
			fixtures.WithPassengerID("e2e-passenger-waiting"),
			fixtures.WithTripID(noShowTripID),
			fixtures.WithPickupWaypointID(noShowWaypointID),
			fixtures.WithBookingStatus(domain.BookingStatusApproved),
		)
		require.NoError(t, fixtures.InsertBookingWithHistory(ctx, testPool, booking))

		resp, err := grpcClient.ReportNoShow(ctxWithUID("e2e-passenger-waiting"), &bookingpb.ReportNoShowRequest{
			BookingId:  "e2e-bk-noshow-driver",
			NoShowType: "driver",
		})

		require.NoError(t, err)
		assert.True(t, resp.Success)
	})

	t.Run("erreur - type no-show invalide", func(t *testing.T) {
		_, err := grpcClient.ReportNoShow(ctxWithUID("driver-1"), &bookingpb.ReportNoShowRequest{
			BookingId:   "e2e-bk-1",
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
	ctx := ctxWithUID("e2e-test-user")
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
	detailResp, err := grpcClient.GetBookingDetails(ctxWithUID("e2e-lifecycle-pax"), &bookingpb.GetBookingDetailsRequest{
		BookingId: "e2e-lifecycle-bk",
	})
	require.NoError(t, err)
	assert.Equal(t, "pendingApproval", detailResp.Booking.Status)
	assert.NotEmpty(t, detailResp.Booking.PaymentCompletedAt)

	// 4. Approuver le booking → approved
	approveResp, err := grpcClient.ApproveBooking(ctxWithUID("e2e-lifecycle-driver"), &bookingpb.ApproveBookingRequest{
		BookingId: "e2e-lifecycle-bk",
	})
	require.NoError(t, err)
	assert.True(t, approveResp.Success)

	// 5. Verifier le statut → approved
	detailResp, err = grpcClient.GetBookingDetails(ctxWithUID("e2e-lifecycle-driver"), &bookingpb.GetBookingDetailsRequest{
		BookingId: "e2e-lifecycle-bk",
	})
	require.NoError(t, err)
	assert.Equal(t, "approved", detailResp.Booking.Status)
	assert.NotEmpty(t, detailResp.Booking.ApprovedAt)

	// 6. Annuler le booking → cancelled
	cancelResp, err := grpcClient.CancelBooking(ctxWithUID("e2e-lifecycle-pax"), &bookingpb.CancelBookingRequest{
		BookingId: "e2e-lifecycle-bk",
		Reason:    "Je ne peux plus voyager",
	})
	require.NoError(t, err)
	assert.True(t, cancelResp.Success)

	// 7. Verifier le statut final → cancelled avec tous les timestamps
	detailResp, err = grpcClient.GetBookingDetails(ctxWithUID("e2e-lifecycle-pax"), &bookingpb.GetBookingDetailsRequest{
		BookingId: "e2e-lifecycle-bk",
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
	_, err = grpcClient.ApproveBooking(ctxWithUID("e2e-lifecycle-driver"), &bookingpb.ApproveBookingRequest{
		BookingId: "e2e-lifecycle-bk",
	})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.FailedPrecondition, st.Code())
}

// =============================================================================
// StartBookingsForWaypoint E2E
// =============================================================================

func TestE2E_StartBookingsForWaypoint(t *testing.T) {
	ctx := ctxWithUID("e2e-test-user")

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
		detailResp, err := grpcClient.GetBookingDetails(ctxWithUID(booking.PassengerID), &bookingpb.GetBookingDetailsRequest{
			BookingId: "e2e-bk-start",
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
	ctx := ctxWithUID("e2e-test-user")

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
		detailResp, err := grpcClient.GetBookingDetails(ctxWithUID(booking.PassengerID), &bookingpb.GetBookingDetailsRequest{
			BookingId: "e2e-bk-complete",
		})
		require.NoError(t, err)
		assert.Equal(t, "completed", detailResp.Booking.Status)
		assert.NotEmpty(t, detailResp.Booking.CompletedAt)
	})
}

// =============================================================================
// Suppression de compte E2E (appels internes auth-service, sans metadata)
// =============================================================================

func TestE2E_AccountDeletionInternalCalls(t *testing.T) {
	ctx := context.Background()
	cleanupBookingTables(t, ctx)

	booking := fixtures.NewTestBooking(
		fixtures.WithBookingID("e2e-bk-deletion"),
		fixtures.WithPassengerID("e2e-pax-deletion"),
		fixtures.WithBookingStatus(domain.BookingStatusCompleted),
	)
	require.NoError(t, fixtures.InsertBooking(ctx, testPool, booking))

	eligibility, err := grpcClient.CheckDeletionEligibility(ctx, &bookingpb.CheckDeletionEligibilityRequest{
		UserId: "e2e-pax-deletion",
	})
	require.NoError(t, err)
	assert.True(t, eligibility.CanDelete)

	ids, err := grpcClient.GetPassengerBookingIDs(ctx, &bookingpb.GetPassengerBookingIDsRequest{
		UserId: "e2e-pax-deletion",
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"e2e-bk-deletion"}, ids.BookingIds)

	anonymized, err := grpcClient.AnonymizeUserData(ctx, &bookingpb.AnonymizeUserDataRequest{
		UserId: "e2e-pax-deletion",
	})
	require.NoError(t, err)
	assert.True(t, anonymized.Success)
}
