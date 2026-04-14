package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/Kpeewu/tissi-mah/services/booking-service/fixtures"
	"github.com/Kpeewu/tissi-mah/services/booking-service/internal/domain"
	"github.com/Kpeewu/tissi-mah/services/booking-service/internal/service"
	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/booking-service/internal/service/interfaces"
	bookingErrors "github.com/Kpeewu/tissi-mah/services/booking-service/pkg/errors"
	"github.com/Kpeewu/tissi-mah/services/booking-service/tests/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// --- Helpers ---

type testDeps struct {
	readRepo      *mocks.MockBookingRepositoryRead
	writeRepo     *mocks.MockBookingRepositoryWrite
	tripClient    *mocks.MockTripClient
	userClient    *mocks.MockUserClient
	paymentClient *mocks.MockPaymentClient
	svc           serviceInterfaces.BookingService
}

func newTestService() *testDeps {
	d := &testDeps{
		readRepo:      new(mocks.MockBookingRepositoryRead),
		writeRepo:     new(mocks.MockBookingRepositoryWrite),
		tripClient:    new(mocks.MockTripClient),
		userClient:    new(mocks.MockUserClient),
		paymentClient: new(mocks.MockPaymentClient),
	}
	logger := zap.NewNop()
	d.svc = service.NewBookingService(
		d.readRepo,
		d.writeRepo,
		d.tripClient,
		d.userClient,
		d.paymentClient,
		nil, // cache
		nil, // notifRedis
		10,  // serviceFeePercent
		logger,
	)
	return d
}

func (d *testDeps) assertExpectations(t *testing.T) {
	d.readRepo.AssertExpectations(t)
	d.writeRepo.AssertExpectations(t)
	d.tripClient.AssertExpectations(t)
	d.userClient.AssertExpectations(t)
	d.paymentClient.AssertExpectations(t)
}

// =============================================================================
// ApproveBooking
// =============================================================================

func TestApproveBooking(t *testing.T) {
	t.Run("succes - approuve un booking", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()

		d.readRepo.On("GetByID", mock.Anything, "booking-1").Return(&domain.Booking{BookingID: "booking-1", PassengerID: "passenger-1"}, nil)
		d.writeRepo.On("Approve", mock.Anything, "booking-1", "driver-1").Return(nil)

		err := d.svc.ApproveBooking(ctx, &serviceInterfaces.ApproveBookingInput{
			DriverID:  "driver-1",
			BookingID: "booking-1",
		})

		require.NoError(t, err)
		d.assertExpectations(t)
	})

	t.Run("erreur - input vide", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()

		err := d.svc.ApproveBooking(ctx, &serviceInterfaces.ApproveBookingInput{
			DriverID:  "",
			BookingID: "booking-1",
		})

		assert.ErrorIs(t, err, bookingErrors.ErrorInvalidInput)
	})
}

// =============================================================================
// RejectBooking
// =============================================================================

func TestRejectBooking(t *testing.T) {
	t.Run("succes - rejette un booking avec remboursement", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()

		booking := fixtures.NewTestBookingWithPayment(
			fixtures.WithBookingID("booking-reject"),
			fixtures.WithDriverID("driver-1"),
			fixtures.WithTripID("trip-1"),
		)

		// GetByID appele dans RejectBooking pour connaitre le tripID + places
		d.readRepo.On("GetByID", mock.Anything, "booking-reject").Return(booking, nil)
		d.writeRepo.On("Reject", mock.Anything, "booking-reject", "driver-1", "Pas disponible").Return(nil)
		d.tripClient.On("IncrementLegBookedSeats", mock.Anything, booking.TripID, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

		// requestRefundAsync appelle tripClient.GetTripDetails pour la date de depart
		d.tripClient.On("GetTripDetails", mock.Anything, booking.TripID).Return(nil, nil).Maybe()
		// fire-and-forget refund
		d.paymentClient.On("RequestRefund", mock.Anything, mock.MatchedBy(func(input interface{}) bool {
			return true
		})).Return(nil).Maybe()

		err := d.svc.RejectBooking(ctx, &serviceInterfaces.RejectBookingInput{
			DriverID:  "driver-1",
			BookingID: "booking-reject",
			Reason:    "Pas disponible",
		})

		require.NoError(t, err)
		d.readRepo.AssertExpectations(t)
		d.writeRepo.AssertExpectations(t)
	})

	t.Run("succes - rejette un booking cash sans remboursement", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()

		booking := fixtures.NewTestCashBooking(
			fixtures.WithBookingID("booking-cash"),
			fixtures.WithDriverID("driver-1"),
			fixtures.WithTripID("trip-2"),
		)

		d.readRepo.On("GetByID", mock.Anything, "booking-cash").Return(booking, nil)
		d.writeRepo.On("Reject", mock.Anything, "booking-cash", "driver-1", "").Return(nil)
		d.tripClient.On("IncrementLegBookedSeats", mock.Anything, booking.TripID, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

		err := d.svc.RejectBooking(ctx, &serviceInterfaces.RejectBookingInput{
			DriverID:  "driver-1",
			BookingID: "booking-cash",
		})

		require.NoError(t, err)
		// paymentClient ne doit PAS etre appele
		d.paymentClient.AssertNotCalled(t, "RequestRefund")
		d.readRepo.AssertExpectations(t)
		d.writeRepo.AssertExpectations(t)
	})

	t.Run("erreur - input vide", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()

		err := d.svc.RejectBooking(ctx, &serviceInterfaces.RejectBookingInput{
			DriverID:  "",
			BookingID: "booking-1",
		})

		assert.ErrorIs(t, err, bookingErrors.ErrorInvalidInput)
	})
}

// =============================================================================
// CancelBooking
// =============================================================================

func TestCancelBooking(t *testing.T) {
	t.Run("succes - annulation passager avec remboursement", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()

		booking := fixtures.NewTestBookingWithPayment(
			fixtures.WithBookingID("booking-cancel"),
			fixtures.WithDriverID("driver-1"),
			fixtures.WithPassengerID("passenger-1"),
			fixtures.WithTripID("trip-1"),
		)

		d.readRepo.On("GetByID", mock.Anything, "booking-cancel").Return(booking, nil)
		d.writeRepo.On("Cancel", mock.Anything, "booking-cancel", "passenger-1", "Changement de plan").Return(nil)
		d.tripClient.On("IncrementLegBookedSeats", mock.Anything, booking.TripID, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

		d.tripClient.On("GetTripDetails", mock.Anything, booking.TripID).Return(nil, nil).Maybe()
		d.paymentClient.On("RequestRefund", mock.Anything, mock.MatchedBy(func(input interface{}) bool {
			return true
		})).Return(nil).Maybe()

		err := d.svc.CancelBooking(ctx, &serviceInterfaces.CancelBookingInput{
			UserID:    "passenger-1",
			BookingID: "booking-cancel",
			Reason:    "Changement de plan",
		})

		require.NoError(t, err)
		d.readRepo.AssertExpectations(t)
		d.writeRepo.AssertExpectations(t)
	})

	t.Run("succes - annulation chauffeur avec remboursement", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()

		booking := fixtures.NewTestBookingWithPayment(
			fixtures.WithBookingID("booking-cancel-driver"),
			fixtures.WithDriverID("driver-1"),
			fixtures.WithPassengerID("passenger-1"),
			fixtures.WithTripID("trip-2"),
		)

		d.readRepo.On("GetByID", mock.Anything, "booking-cancel-driver").Return(booking, nil)
		d.writeRepo.On("Cancel", mock.Anything, "booking-cancel-driver", "driver-1", "Vehicule en panne").Return(nil)
		d.tripClient.On("IncrementLegBookedSeats", mock.Anything, booking.TripID, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

		d.tripClient.On("GetTripDetails", mock.Anything, booking.TripID).Return(nil, nil).Maybe()
		d.paymentClient.On("RequestRefund", mock.Anything, mock.MatchedBy(func(input interface{}) bool {
			return true
		})).Return(nil).Maybe()

		err := d.svc.CancelBooking(ctx, &serviceInterfaces.CancelBookingInput{
			UserID:    "driver-1",
			BookingID: "booking-cancel-driver",
			Reason:    "Vehicule en panne",
		})

		require.NoError(t, err)
		d.readRepo.AssertExpectations(t)
		d.writeRepo.AssertExpectations(t)
	})

	t.Run("succes - annulation sans paiement complete (pas de remboursement)", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()

		booking := fixtures.NewTestBooking(
			fixtures.WithBookingID("booking-nopay"),
			fixtures.WithDriverID("driver-1"),
			fixtures.WithTripID("trip-3"),
			fixtures.WithNoPaymentCompleted(),
		)

		d.readRepo.On("GetByID", mock.Anything, "booking-nopay").Return(booking, nil)
		d.writeRepo.On("Cancel", mock.Anything, "booking-nopay", "user-1", "").Return(nil)
		d.tripClient.On("IncrementLegBookedSeats", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

		err := d.svc.CancelBooking(ctx, &serviceInterfaces.CancelBookingInput{
			UserID:    "user-1",
			BookingID: "booking-nopay",
		})

		require.NoError(t, err)
		d.paymentClient.AssertNotCalled(t, "RequestRefund")
		d.readRepo.AssertExpectations(t)
		d.writeRepo.AssertExpectations(t)
	})

	t.Run("erreur - booking non trouve", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()

		d.readRepo.On("GetByID", mock.Anything, "unknown").Return(nil, bookingErrors.ErrorBookingNotFound)

		err := d.svc.CancelBooking(ctx, &serviceInterfaces.CancelBookingInput{
			UserID:    "user-1",
			BookingID: "unknown",
		})

		assert.ErrorIs(t, err, bookingErrors.ErrorBookingNotFound)
		d.readRepo.AssertExpectations(t)
	})

	t.Run("erreur - input vide", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()

		err := d.svc.CancelBooking(ctx, &serviceInterfaces.CancelBookingInput{
			UserID:    "",
			BookingID: "booking-1",
		})

		assert.ErrorIs(t, err, bookingErrors.ErrorInvalidInput)
	})
}

// =============================================================================
// ReportNoShow
// =============================================================================

func TestReportNoShow(t *testing.T) {
	t.Run("succes - no-show passager avec remboursement", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()

		booking := fixtures.NewTestBookingWithPayment(
			fixtures.WithBookingID("booking-noshow"),
			fixtures.WithDriverID("driver-1"),
			fixtures.WithTripID("trip-1"),
			fixtures.WithBookingStatus(domain.BookingStatusInProgress),
		)

		d.readRepo.On("GetByID", mock.Anything, "booking-noshow").Return(booking, nil)
		d.writeRepo.On("ReportNoShow", mock.Anything, "booking-noshow", "driver-1", "passenger", "Passager absent").Return(nil)
		d.tripClient.On("IncrementLegBookedSeats", mock.Anything, booking.TripID, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

		d.tripClient.On("GetTripDetails", mock.Anything, booking.TripID).Return(nil, nil).Maybe()
		d.paymentClient.On("RequestRefund", mock.Anything, mock.MatchedBy(func(input interface{}) bool {
			return true
		})).Return(nil).Maybe()

		err := d.svc.ReportNoShow(ctx, &serviceInterfaces.ReportNoShowInput{
			BookingID:   "booking-noshow",
			ReporterID:  "driver-1",
			NoShowType:  "passenger",
			Description: "Passager absent",
		})

		require.NoError(t, err)
		d.readRepo.AssertExpectations(t)
		d.writeRepo.AssertExpectations(t)
	})

	t.Run("succes - no-show chauffeur", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()

		booking := fixtures.NewTestBookingWithPayment(
			fixtures.WithBookingID("booking-noshow-driver"),
			fixtures.WithDriverID("driver-2"),
			fixtures.WithTripID("trip-2"),
		)

		d.readRepo.On("GetByID", mock.Anything, "booking-noshow-driver").Return(booking, nil)
		d.writeRepo.On("ReportNoShow", mock.Anything, "booking-noshow-driver", "passenger-1", "driver", "Chauffeur pas venu").Return(nil)
		d.tripClient.On("IncrementLegBookedSeats", mock.Anything, booking.TripID, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

		d.tripClient.On("GetTripDetails", mock.Anything, booking.TripID).Return(nil, nil).Maybe()
		d.paymentClient.On("RequestRefund", mock.Anything, mock.MatchedBy(func(input interface{}) bool {
			return true
		})).Return(nil).Maybe()

		err := d.svc.ReportNoShow(ctx, &serviceInterfaces.ReportNoShowInput{
			BookingID:   "booking-noshow-driver",
			ReporterID:  "passenger-1",
			NoShowType:  "driver",
			Description: "Chauffeur pas venu",
		})

		require.NoError(t, err)
		d.readRepo.AssertExpectations(t)
		d.writeRepo.AssertExpectations(t)
	})

	t.Run("succes - no-show booking cash (pas de remboursement)", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()

		booking := fixtures.NewTestCashBooking(
			fixtures.WithBookingID("booking-noshow-cash"),
			fixtures.WithTripID("trip-3"),
		)

		d.readRepo.On("GetByID", mock.Anything, "booking-noshow-cash").Return(booking, nil)
		d.writeRepo.On("ReportNoShow", mock.Anything, "booking-noshow-cash", "reporter-1", "passenger", "Absent").Return(nil)
		d.tripClient.On("IncrementLegBookedSeats", mock.Anything, booking.TripID, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

		err := d.svc.ReportNoShow(ctx, &serviceInterfaces.ReportNoShowInput{
			BookingID:   "booking-noshow-cash",
			ReporterID:  "reporter-1",
			NoShowType:  "passenger",
			Description: "Absent",
		})

		require.NoError(t, err)
		d.paymentClient.AssertNotCalled(t, "RequestRefund")
		d.readRepo.AssertExpectations(t)
		d.writeRepo.AssertExpectations(t)
	})

	t.Run("erreur - type no-show invalide", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()

		err := d.svc.ReportNoShow(ctx, &serviceInterfaces.ReportNoShowInput{
			BookingID:  "booking-1",
			ReporterID: "user-1",
			NoShowType: "invalid",
		})

		assert.ErrorIs(t, err, bookingErrors.ErrorInvalidInput)
	})

	t.Run("erreur - input vide", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()

		err := d.svc.ReportNoShow(ctx, &serviceInterfaces.ReportNoShowInput{
			BookingID:  "",
			ReporterID: "user-1",
			NoShowType: "passenger",
		})

		assert.ErrorIs(t, err, bookingErrors.ErrorInvalidInput)
	})
}

// =============================================================================
// ConfirmPayment
// =============================================================================

func TestConfirmPayment(t *testing.T) {
	t.Run("succes - confirme le paiement", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()

		d.writeRepo.On("ConfirmPayment", mock.Anything, "booking-pay", "txn-123").Return(nil)

		err := d.svc.ConfirmPayment(ctx, &serviceInterfaces.ConfirmPaymentInput{
			BookingID:     "booking-pay",
			TransactionID: "txn-123",
		})

		require.NoError(t, err)
		d.assertExpectations(t)
	})

	t.Run("erreur - input vide", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()

		err := d.svc.ConfirmPayment(ctx, &serviceInterfaces.ConfirmPaymentInput{
			BookingID:     "",
			TransactionID: "txn-123",
		})

		assert.ErrorIs(t, err, bookingErrors.ErrorInvalidInput)
	})
}

// =============================================================================
// StartBookingsForWaypoint / CompleteBookingsForWaypoint
// =============================================================================

func TestStartBookingsForWaypoint(t *testing.T) {
	t.Run("succes - demarre les bookings", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()

		d.writeRepo.On("StartBookingsForWaypoint", mock.Anything, "trip-1", "wp-1").Return([]string{"p1", "p2", "p3"}, nil)

		count, err := d.svc.StartBookingsForWaypoint(ctx, &serviceInterfaces.StartBookingsForWaypointInput{
			TripID:     "trip-1",
			WaypointID: "wp-1",
		})

		require.NoError(t, err)
		assert.Equal(t, 3, count)
		d.assertExpectations(t)
	})

	t.Run("erreur - input vide", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()

		_, err := d.svc.StartBookingsForWaypoint(ctx, &serviceInterfaces.StartBookingsForWaypointInput{
			TripID:     "",
			WaypointID: "wp-1",
		})

		assert.ErrorIs(t, err, bookingErrors.ErrorInvalidInput)
	})
}

func TestCompleteBookingsForWaypoint(t *testing.T) {
	t.Run("succes - complete les bookings", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()

		d.writeRepo.On("CompleteBookingsForWaypoint", mock.Anything, "trip-1", "wp-2").Return([]string{"p1", "p2"}, nil)

		count, err := d.svc.CompleteBookingsForWaypoint(ctx, &serviceInterfaces.CompleteBookingsForWaypointInput{
			TripID:     "trip-1",
			WaypointID: "wp-2",
		})

		require.NoError(t, err)
		assert.Equal(t, 2, count)
		d.assertExpectations(t)
	})
}

// =============================================================================
// shouldRequestRefund (teste indirectement via CancelBooking)
// =============================================================================

func TestShouldRequestRefund_Conditions(t *testing.T) {
	now := time.Now().UTC()

	t.Run("cash booking → pas de remboursement meme avec paiement complete", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()

		booking := fixtures.NewTestBooking(
			fixtures.WithBookingID("booking-cash-paid"),
			fixtures.WithPaymentMethod(domain.PaymentCash),
			fixtures.WithPaymentCompletedAt(now),
		)

		d.readRepo.On("GetByID", mock.Anything, "booking-cash-paid").Return(booking, nil)
		d.writeRepo.On("Cancel", mock.Anything, "booking-cash-paid", "user-1", "").Return(nil)
		d.tripClient.On("IncrementLegBookedSeats", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

		err := d.svc.CancelBooking(ctx, &serviceInterfaces.CancelBookingInput{
			UserID:    "user-1",
			BookingID: "booking-cash-paid",
		})

		require.NoError(t, err)
		d.paymentClient.AssertNotCalled(t, "RequestRefund")
	})

	t.Run("mobileMoney sans paiement complete → pas de remboursement", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()

		booking := fixtures.NewTestBooking(
			fixtures.WithBookingID("booking-nopay"),
			fixtures.WithPaymentMethod(domain.PaymentMobileMoney),
			fixtures.WithNoPaymentCompleted(),
		)

		d.readRepo.On("GetByID", mock.Anything, "booking-nopay").Return(booking, nil)
		d.writeRepo.On("Cancel", mock.Anything, "booking-nopay", "user-1", "").Return(nil)
		d.tripClient.On("IncrementLegBookedSeats", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

		err := d.svc.CancelBooking(ctx, &serviceInterfaces.CancelBookingInput{
			UserID:    "user-1",
			BookingID: "booking-nopay",
		})

		require.NoError(t, err)
		d.paymentClient.AssertNotCalled(t, "RequestRefund")
	})
}
