package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/Kpeewu/tissi-mah/pkg/notification"
	"github.com/Kpeewu/tissi-mah/services/booking-service/fixtures"
	"github.com/Kpeewu/tissi-mah/services/booking-service/internal/client"
	"github.com/Kpeewu/tissi-mah/services/booking-service/internal/domain"
	"github.com/Kpeewu/tissi-mah/services/booking-service/internal/service"
	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/booking-service/internal/service/interfaces"
	bookingErrors "github.com/Kpeewu/tissi-mah/services/booking-service/pkg/errors"
	"github.com/Kpeewu/tissi-mah/services/booking-service/tests/mocks"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
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
	ratingClient  *mocks.MockRatingClient
	svc           serviceInterfaces.BookingService
}

func newTestService() *testDeps {
	d := &testDeps{
		readRepo:      new(mocks.MockBookingRepositoryRead),
		writeRepo:     new(mocks.MockBookingRepositoryWrite),
		tripClient:    new(mocks.MockTripClient),
		userClient:    new(mocks.MockUserClient),
		paymentClient: new(mocks.MockPaymentClient),
		ratingClient:  new(mocks.MockRatingClient),
	}
	logger := zap.NewNop()
	d.svc = service.NewBookingService(
		d.readRepo,
		d.writeRepo,
		d.tripClient,
		d.userClient,
		d.paymentClient,
		d.ratingClient,
		nil, // cache
		nil, // notifRedis
		10,  // serviceFeePercent
		logger,
	)
	return d
}

// newTestServiceWithNotif construit le service avec un Redis de notification (miniredis)
// pour vérifier les événements publiés.
func newTestServiceWithNotif(t *testing.T) (*testDeps, *redis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { assert.NoError(t, rdb.Close()) })

	d := newTestService()
	d.svc = service.NewBookingService(
		d.readRepo,
		d.writeRepo,
		d.tripClient,
		d.userClient,
		d.paymentClient,
		d.ratingClient,
		nil, // cache
		rdb,
		10, // serviceFeePercent
		zap.NewNop(),
	)
	return d, rdb
}

// publishedNotifications renvoie les couples "EVENT_TYPE:userID" publiés sur le stream de notification.
func publishedNotifications(t *testing.T, rdb *redis.Client) []string {
	t.Helper()
	msgs, err := rdb.XRange(context.Background(), notification.StreamName, "-", "+").Result()
	require.NoError(t, err)

	out := make([]string, 0, len(msgs))
	for _, msg := range msgs {
		data, ok := msg.Values["data"].(string)
		require.True(t, ok)
		var event notification.Event
		require.NoError(t, json.Unmarshal([]byte(data), &event))
		out = append(out, event.EventType+":"+event.UserID)
	}
	return out
}

func (d *testDeps) assertExpectations(t *testing.T) {
	d.readRepo.AssertExpectations(t)
	d.writeRepo.AssertExpectations(t)
	d.tripClient.AssertExpectations(t)
	d.userClient.AssertExpectations(t)
	d.paymentClient.AssertExpectations(t)
	d.ratingClient.AssertExpectations(t)
}

// =============================================================================
// ApproveBooking
// =============================================================================

// =============================================================================
// GetBookingDetails / GetBookingDetailsInternal
// =============================================================================

func TestGetBookingDetails(t *testing.T) {
	detailsBooking := &domain.Booking{
		BookingID: "booking-1", TripID: "trip-1", PassengerID: "passenger-1", DriverID: "driver-1",
	}

	for _, userID := range []string{"passenger-1", "driver-1"} {
		t.Run("succes - lecture par "+userID, func(t *testing.T) {
			d := newTestService()
			d.readRepo.On("GetByIDWithDetails", mock.Anything, "booking-1").Return(detailsBooking, nil, nil, nil)
			d.tripClient.On("GetTripDetails", mock.Anything, "trip-1").Return(&client.TripDetails{RoutePolyline: "poly"}, nil)

			result, err := d.svc.GetBookingDetails(context.Background(), &serviceInterfaces.GetBookingDetailsInput{
				BookingID: "booking-1",
				UserID:    userID,
			})

			require.NoError(t, err)
			assert.Equal(t, "booking-1", result.BookingID)
			assert.Equal(t, "poly", result.RoutePolyline)
		})
	}

	t.Run("erreur - un tiers ne peut pas lire la reservation", func(t *testing.T) {
		d := newTestService()
		d.readRepo.On("GetByIDWithDetails", mock.Anything, "booking-1").Return(detailsBooking, nil, nil, nil)

		_, err := d.svc.GetBookingDetails(context.Background(), &serviceInterfaces.GetBookingDetailsInput{
			BookingID: "booking-1",
			UserID:    "intrus",
		})

		assert.ErrorIs(t, err, bookingErrors.ErrorUnauthorized)
	})

	t.Run("erreur - identite vide refusee sans lecture", func(t *testing.T) {
		d := newTestService()

		_, err := d.svc.GetBookingDetails(context.Background(), &serviceInterfaces.GetBookingDetailsInput{
			BookingID: "booking-1",
		})

		assert.ErrorIs(t, err, bookingErrors.ErrorUnauthorized)
		d.readRepo.AssertNotCalled(t, "GetByIDWithDetails", mock.Anything, mock.Anything)
	})

	t.Run("succes - route interne sans identite ni enrichissement", func(t *testing.T) {
		d := newTestService()
		d.readRepo.On("GetByIDWithDetails", mock.Anything, "booking-1").Return(detailsBooking, nil, nil, nil)

		result, err := d.svc.GetBookingDetailsInternal(context.Background(), "booking-1")

		require.NoError(t, err)
		assert.Equal(t, "passenger-1", result.PassengerID)
		assert.Empty(t, result.RoutePolyline)
		d.tripClient.AssertNotCalled(t, "GetTripDetails", mock.Anything, mock.Anything)
	})

	t.Run("erreur - route interne avec ID vide", func(t *testing.T) {
		d := newTestService()

		_, err := d.svc.GetBookingDetailsInternal(context.Background(), "")

		assert.ErrorIs(t, err, bookingErrors.ErrorInvalidInput)
	})
}

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
	// Prise en charge prévue passée depuis plus que le délai d'attente
	pastPickup := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)

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

		d.expectDeparture(booking, pastPickup)
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
			fixtures.WithPassengerID("passenger-1"),
			fixtures.WithDriverID("driver-2"),
			fixtures.WithTripID("trip-2"),
		)

		d.readRepo.On("GetByID", mock.Anything, "booking-noshow-driver").Return(booking, nil)
		d.writeRepo.On("ReportNoShow", mock.Anything, "booking-noshow-driver", "passenger-1", "driver", "Chauffeur pas venu").Return(nil)
		d.tripClient.On("IncrementLegBookedSeats", mock.Anything, booking.TripID, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

		d.expectDeparture(booking, pastPickup)
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
			fixtures.WithDriverID("reporter-1"),
			fixtures.WithTripID("trip-3"),
		)

		d.readRepo.On("GetByID", mock.Anything, "booking-noshow-cash").Return(booking, nil)
		d.writeRepo.On("ReportNoShow", mock.Anything, "booking-noshow-cash", "reporter-1", "passenger", "Absent").Return(nil)
		d.expectDeparture(booking, pastPickup)
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

	unauthorizedCases := []struct {
		name       string
		reporterID string
		noShowType string
	}{
		{"erreur - un tiers ne peut pas signaler", "intrus", "driver"},
		{"erreur - le passager ne peut pas se declarer absent", "passenger-x", "passenger"},
		{"erreur - le chauffeur ne peut pas se declarer absent", "driver-x", "driver"},
	}
	for _, tc := range unauthorizedCases {
		t.Run(tc.name, func(t *testing.T) {
			d := newTestService()
			ctx := context.Background()

			booking := fixtures.NewTestBookingWithPayment(
				fixtures.WithBookingID("booking-noshow-unauth"),
				fixtures.WithPassengerID("passenger-x"),
				fixtures.WithDriverID("driver-x"),
				fixtures.WithBookingStatus(domain.BookingStatusInProgress),
			)
			d.readRepo.On("GetByID", mock.Anything, "booking-noshow-unauth").Return(booking, nil)

			err := d.svc.ReportNoShow(ctx, &serviceInterfaces.ReportNoShowInput{
				BookingID:  "booking-noshow-unauth",
				ReporterID: tc.reporterID,
				NoShowType: tc.noShowType,
			})

			assert.ErrorIs(t, err, bookingErrors.ErrorUnauthorized)
			d.writeRepo.AssertNotCalled(t, "ReportNoShow",
				mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		})
	}

	t.Run("erreur - signalement avant la fin du delai d'attente", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()

		booking := fixtures.NewTestBookingWithPayment(
			fixtures.WithBookingID("booking-noshow-early"),
			fixtures.WithPassengerID("passenger-1"),
			fixtures.WithDriverID("driver-1"),
		)
		d.readRepo.On("GetByID", mock.Anything, "booking-noshow-early").Return(booking, nil)
		// Prise en charge prévue il y a 5 min : le délai d'attente n'est pas écoulé
		d.expectDeparture(booking, time.Now().UTC().Add(-5*time.Minute).Format(time.RFC3339))

		err := d.svc.ReportNoShow(ctx, &serviceInterfaces.ReportNoShowInput{
			BookingID:  "booking-noshow-early",
			ReporterID: "passenger-1",
			NoShowType: "driver",
		})

		assert.ErrorIs(t, err, bookingErrors.ErrorNoShowTooEarly)
		d.writeRepo.AssertNotCalled(t, "ReportNoShow",
			mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("erreur - signalement apres la fin de la fenetre de declaration", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()

		booking := fixtures.NewTestBookingWithPayment(
			fixtures.WithBookingID("booking-noshow-late"),
			fixtures.WithPassengerID("passenger-1"),
			fixtures.WithDriverID("driver-1"),
		)
		d.readRepo.On("GetByID", mock.Anything, "booking-noshow-late").Return(booking, nil)
		// Prise en charge prévue il y a 4 h : la fenêtre de 3 h est close
		d.expectDeparture(booking, time.Now().UTC().Add(-4*time.Hour).Format(time.RFC3339))

		err := d.svc.ReportNoShow(ctx, &serviceInterfaces.ReportNoShowInput{
			BookingID:  "booking-noshow-late",
			ReporterID: "driver-1",
			NoShowType: "passenger",
		})

		assert.ErrorIs(t, err, bookingErrors.ErrorNoShowReportClosed)
		d.writeRepo.AssertNotCalled(t, "ReportNoShow",
			mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("erreur - heure de prise en charge inconnue", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()

		booking := fixtures.NewTestBookingWithPayment(
			fixtures.WithBookingID("booking-noshow-unknown"),
			fixtures.WithPassengerID("passenger-1"),
			fixtures.WithDriverID("driver-1"),
		)
		d.readRepo.On("GetByID", mock.Anything, "booking-noshow-unknown").Return(booking, nil)
		d.tripClient.On("GetTripDetails", mock.Anything, booking.TripID).
			Return(nil, errors.New("trips-service unavailable"))

		err := d.svc.ReportNoShow(ctx, &serviceInterfaces.ReportNoShowInput{
			BookingID:  "booking-noshow-unknown",
			ReporterID: "passenger-1",
			NoShowType: "driver",
		})

		assert.ErrorIs(t, err, bookingErrors.ErrorDataRetrievalFailed)
		d.writeRepo.AssertNotCalled(t, "ReportNoShow",
			mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
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
	t.Run("succes - paymentPending sans auto-approbation → pendingApproval", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()

		booking := fixtures.NewTestBooking(
			fixtures.WithBookingID("booking-pay"),
			fixtures.WithTripID("trip-pay"),
			fixtures.WithBookingStatus(domain.BookingStatusPaymentPending),
		)
		d.readRepo.On("GetByID", mock.Anything, "booking-pay").Return(booking, nil)
		d.tripClient.On("GetTripDetails", mock.Anything, "trip-pay").Return(&client.TripDetails{TripID: "trip-pay", Status: "scheduled"}, nil)
		d.writeRepo.On("ConfirmPayment", mock.Anything, "booking-pay", "txn-123",
			domain.BookingStatusPaymentPending, domain.BookingStatusPendingApproval).Return(nil)

		status, err := d.svc.ConfirmPayment(ctx, &serviceInterfaces.ConfirmPaymentInput{
			BookingID:     "booking-pay",
			TransactionID: "txn-123",
		})

		require.NoError(t, err)
		assert.Equal(t, string(domain.BookingStatusPendingApproval), status)
		d.tripClient.AssertNotCalled(t, "IncrementLegBookedSeats", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		d.assertExpectations(t)
	})

	t.Run("succes - trajet en auto-approbation → approved", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()

		booking := fixtures.NewTestBooking(
			fixtures.WithBookingID("booking-auto"),
			fixtures.WithTripID("trip-auto"),
			fixtures.WithBookingStatus(domain.BookingStatusPaymentPending),
		)
		d.readRepo.On("GetByID", mock.Anything, "booking-auto").Return(booking, nil)
		d.tripClient.On("GetTripDetails", mock.Anything, "trip-auto").Return(&client.TripDetails{TripID: "trip-auto", Status: "scheduled", AutoApproveEnabled: true}, nil)
		d.writeRepo.On("ConfirmPayment", mock.Anything, "booking-auto", "txn-auto",
			domain.BookingStatusPaymentPending, domain.BookingStatusApproved).Return(nil)

		status, err := d.svc.ConfirmPayment(ctx, &serviceInterfaces.ConfirmPaymentInput{
			BookingID:     "booking-auto",
			TransactionID: "txn-auto",
		})

		require.NoError(t, err)
		assert.Equal(t, string(domain.BookingStatusApproved), status)
		d.assertExpectations(t)
	})

	t.Run("notifications - auto-approbation → conducteur ET passager notifiés", func(t *testing.T) {
		d, rdb := newTestServiceWithNotif(t)
		ctx := context.Background()

		booking := fixtures.NewTestBooking(
			fixtures.WithBookingID("booking-auto-notif"),
			fixtures.WithTripID("trip-auto-notif"),
			fixtures.WithDriverID("driver-1"),
			fixtures.WithPassengerID("passenger-1"),
			fixtures.WithBookingStatus(domain.BookingStatusPaymentPending),
		)
		d.readRepo.On("GetByID", mock.Anything, "booking-auto-notif").Return(booking, nil)
		d.tripClient.On("GetTripDetails", mock.Anything, "trip-auto-notif").Return(
			&client.TripDetails{TripID: "trip-auto-notif", Status: "scheduled", AutoApproveEnabled: true}, nil)
		d.writeRepo.On("ConfirmPayment", mock.Anything, "booking-auto-notif", "txn-auto-notif",
			domain.BookingStatusPaymentPending, domain.BookingStatusApproved).Return(nil)

		_, err := d.svc.ConfirmPayment(ctx, &serviceInterfaces.ConfirmPaymentInput{
			BookingID:     "booking-auto-notif",
			TransactionID: "txn-auto-notif",
		})

		require.NoError(t, err)
		assert.ElementsMatch(t, []string{
			notification.NewBookingRequest + ":driver-1",
			notification.BookingConfirmed + ":passenger-1",
		}, publishedNotifications(t, rdb))
		d.assertExpectations(t)
	})

	t.Run("notifications - sans auto-approbation → seul le conducteur est notifié", func(t *testing.T) {
		d, rdb := newTestServiceWithNotif(t)
		ctx := context.Background()

		booking := fixtures.NewTestBooking(
			fixtures.WithBookingID("booking-manual-notif"),
			fixtures.WithTripID("trip-manual-notif"),
			fixtures.WithDriverID("driver-2"),
			fixtures.WithPassengerID("passenger-2"),
			fixtures.WithBookingStatus(domain.BookingStatusPaymentPending),
		)
		d.readRepo.On("GetByID", mock.Anything, "booking-manual-notif").Return(booking, nil)
		d.tripClient.On("GetTripDetails", mock.Anything, "trip-manual-notif").Return(
			&client.TripDetails{TripID: "trip-manual-notif", Status: "scheduled"}, nil)
		d.writeRepo.On("ConfirmPayment", mock.Anything, "booking-manual-notif", "txn-manual-notif",
			domain.BookingStatusPaymentPending, domain.BookingStatusPendingApproval).Return(nil)

		_, err := d.svc.ConfirmPayment(ctx, &serviceInterfaces.ConfirmPaymentInput{
			BookingID:     "booking-manual-notif",
			TransactionID: "txn-manual-notif",
		})

		require.NoError(t, err)
		assert.Equal(t, []string{notification.NewBookingRequest + ":driver-2"}, publishedNotifications(t, rdb))
		d.assertExpectations(t)
	})

	t.Run("succes - trips-service indisponible → pendingApproval par défaut", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()

		booking := fixtures.NewTestBooking(
			fixtures.WithBookingID("booking-notrip"),
			fixtures.WithTripID("trip-down"),
			fixtures.WithBookingStatus(domain.BookingStatusPaymentPending),
		)
		d.readRepo.On("GetByID", mock.Anything, "booking-notrip").Return(booking, nil)
		d.tripClient.On("GetTripDetails", mock.Anything, "trip-down").Return(nil, errors.New("unavailable"))
		d.writeRepo.On("ConfirmPayment", mock.Anything, "booking-notrip", "txn-1",
			domain.BookingStatusPaymentPending, domain.BookingStatusPendingApproval).Return(nil)

		status, err := d.svc.ConfirmPayment(ctx, &serviceInterfaces.ConfirmPaymentInput{
			BookingID:     "booking-notrip",
			TransactionID: "txn-1",
		})

		require.NoError(t, err)
		assert.Equal(t, string(domain.BookingStatusPendingApproval), status)
	})

	t.Run("succes - réactivation d'une réservation paymentFailed avec places disponibles", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()

		booking := fixtures.NewTestBooking(
			fixtures.WithBookingID("booking-late"),
			fixtures.WithTripID("trip-late"),
			fixtures.WithBookingStatus(domain.BookingStatusPaymentFailed),
			fixtures.WithNoPaymentCompleted(),
		)
		d.readRepo.On("GetByID", mock.Anything, "booking-late").Return(booking, nil)
		d.readRepo.On("HasActiveBooking", mock.Anything, booking.PassengerID, "trip-late").Return(false, nil)
		d.tripClient.On("GetTripDetails", mock.Anything, "trip-late").Return(&client.TripDetails{TripID: "trip-late", Status: "scheduled", AvailableSeats: 2}, nil)
		d.writeRepo.On("ConfirmPayment", mock.Anything, "booking-late", "txn-late",
			domain.BookingStatusPaymentFailed, domain.BookingStatusPendingApproval).Return(nil)
		d.tripClient.On("IncrementLegBookedSeats", mock.Anything, "trip-late",
			int(booking.PickupSequencerOrder), int(booking.DropoffSequencerOrder), int(booking.SeatsBooked)).Return(nil)

		status, err := d.svc.ConfirmPayment(ctx, &serviceInterfaces.ConfirmPaymentInput{
			BookingID:     "booking-late",
			TransactionID: "txn-late",
		})

		require.NoError(t, err)
		assert.Equal(t, string(domain.BookingStatusPendingApproval), status)
		d.assertExpectations(t)
	})

	t.Run("erreur - réactivation impossible, trajet complet", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()

		booking := fixtures.NewTestBooking(
			fixtures.WithBookingID("booking-full"),
			fixtures.WithTripID("trip-full"),
			fixtures.WithBookingStatus(domain.BookingStatusPaymentFailed),
			fixtures.WithSeatsBooked(2),
			fixtures.WithNoPaymentCompleted(),
		)
		d.readRepo.On("GetByID", mock.Anything, "booking-full").Return(booking, nil)
		d.readRepo.On("HasActiveBooking", mock.Anything, booking.PassengerID, "trip-full").Return(false, nil)
		d.tripClient.On("GetTripDetails", mock.Anything, "trip-full").Return(&client.TripDetails{TripID: "trip-full", Status: "scheduled", AvailableSeats: 1}, nil)

		_, err := d.svc.ConfirmPayment(ctx, &serviceInterfaces.ConfirmPaymentInput{
			BookingID:     "booking-full",
			TransactionID: "txn-full",
		})

		assert.ErrorIs(t, err, bookingErrors.ErrorNoSeatsAvailable)
		d.writeRepo.AssertNotCalled(t, "ConfirmPayment", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("erreur - réactivation impossible, le passager a déjà une autre réservation active", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()

		booking := fixtures.NewTestBooking(
			fixtures.WithBookingID("booking-dup"),
			fixtures.WithTripID("trip-dup"),
			fixtures.WithBookingStatus(domain.BookingStatusPaymentFailed),
			fixtures.WithNoPaymentCompleted(),
		)
		d.readRepo.On("GetByID", mock.Anything, "booking-dup").Return(booking, nil)
		d.readRepo.On("HasActiveBooking", mock.Anything, booking.PassengerID, "trip-dup").Return(true, nil)
		d.tripClient.On("GetTripDetails", mock.Anything, "trip-dup").Return(&client.TripDetails{TripID: "trip-dup", Status: "scheduled", AvailableSeats: 3}, nil)

		_, err := d.svc.ConfirmPayment(ctx, &serviceInterfaces.ConfirmPaymentInput{
			BookingID:     "booking-dup",
			TransactionID: "txn-dup",
		})

		assert.ErrorIs(t, err, bookingErrors.ErrorNoSeatsAvailable)
	})

	t.Run("erreur - réservation déjà confirmée", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()

		booking := fixtures.NewTestBooking(
			fixtures.WithBookingID("booking-done"),
			fixtures.WithBookingStatus(domain.BookingStatusPendingApproval),
			fixtures.WithPaymentCompletedAt(time.Now()),
		)
		d.readRepo.On("GetByID", mock.Anything, "booking-done").Return(booking, nil)

		_, err := d.svc.ConfirmPayment(ctx, &serviceInterfaces.ConfirmPaymentInput{
			BookingID:     "booking-done",
			TransactionID: "txn-done",
		})

		assert.ErrorIs(t, err, bookingErrors.ErrorPaymentAlreadyConfirmed)
	})

	t.Run("erreur - input vide", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()

		_, err := d.svc.ConfirmPayment(ctx, &serviceInterfaces.ConfirmPaymentInput{
			BookingID:     "",
			TransactionID: "txn-123",
		})

		assert.ErrorIs(t, err, bookingErrors.ErrorInvalidInput)
	})
}

// =============================================================================
// CancelBooking — paiement en ligne en cours
// =============================================================================

func TestCancelBooking_PaymentPending(t *testing.T) {
	t.Run("supprime la collecte FedaPay avant de libérer les places", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()

		booking := fixtures.NewTestBooking(
			fixtures.WithBookingID("booking-pp"),
			fixtures.WithPassengerID("passenger-pp"),
			fixtures.WithBookingStatus(domain.BookingStatusPaymentPending),
			fixtures.WithNoPaymentCompleted(),
		)
		d.readRepo.On("GetByID", mock.Anything, "booking-pp").Return(booking, nil)
		d.paymentClient.On("CancelPaymentForBooking", mock.Anything, "booking-pp", mock.Anything).Return("failed", nil)
		d.writeRepo.On("Cancel", mock.Anything, "booking-pp", "passenger-pp", "").Return(nil)
		d.tripClient.On("IncrementLegBookedSeats", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

		err := d.svc.CancelBooking(ctx, &serviceInterfaces.CancelBookingInput{
			UserID:    "passenger-pp",
			BookingID: "booking-pp",
		})

		require.NoError(t, err)
		d.paymentClient.AssertNotCalled(t, "RequestRefund", mock.Anything, mock.Anything)
		d.paymentClient.AssertExpectations(t)
		d.writeRepo.AssertExpectations(t)
	})

	t.Run("refuse l'annulation si la collecte n'a pas pu être supprimée", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()

		booking := fixtures.NewTestBooking(
			fixtures.WithBookingID("booking-pp-err"),
			fixtures.WithPassengerID("passenger-pp"),
			fixtures.WithBookingStatus(domain.BookingStatusPaymentPending),
		)
		d.readRepo.On("GetByID", mock.Anything, "booking-pp-err").Return(booking, nil)
		d.paymentClient.On("CancelPaymentForBooking", mock.Anything, "booking-pp-err", mock.Anything).Return("", errors.New("unavailable"))

		err := d.svc.CancelBooking(ctx, &serviceInterfaces.CancelBookingInput{
			UserID:    "passenger-pp",
			BookingID: "booking-pp-err",
		})

		assert.ErrorIs(t, err, bookingErrors.ErrorInternalServer)
		d.writeRepo.AssertNotCalled(t, "Cancel", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("refuse si l'utilisateur n'est ni passager ni conducteur", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()

		booking := fixtures.NewTestBooking(
			fixtures.WithBookingID("booking-pp-other"),
			fixtures.WithBookingStatus(domain.BookingStatusPaymentPending),
		)
		d.readRepo.On("GetByID", mock.Anything, "booking-pp-other").Return(booking, nil)

		err := d.svc.CancelBooking(ctx, &serviceInterfaces.CancelBookingInput{
			UserID:    "intruder",
			BookingID: "booking-pp-other",
		})

		assert.ErrorIs(t, err, bookingErrors.ErrorUnauthorized)
		d.paymentClient.AssertNotCalled(t, "CancelPaymentForBooking", mock.Anything, mock.Anything, mock.Anything)
	})
}

// =============================================================================
// CancelBookingsForTrip / CancelBookingsForWaypoint — paiements
// =============================================================================

// waitForAsyncCall attend qu'un appel fire-and-forget (goroutine) atteigne le mock.
func waitForAsyncCall(t *testing.T, called <-chan string) string {
	t.Helper()
	select {
	case id := <-called:
		return id
	case <-time.After(2 * time.Second):
		t.Fatal("appel asynchrone au payment-service non reçu")
		return ""
	}
}

// bulkCancelFixtures retourne une réservation paymentPending (collecte en cours), une réservation
// approuvée payée et une réservation cash : seules les deux premières sollicitent payment-service.
func bulkCancelFixtures(tripID string) (pending, paid, cash *domain.Booking) {
	pending = fixtures.NewTestBooking(
		fixtures.WithBookingID("bk-bulk-pending"),
		fixtures.WithTripID(tripID),
		fixtures.WithBookingStatus(domain.BookingStatusPaymentPending),
		fixtures.WithNoPaymentCompleted(),
	)
	paid = fixtures.NewTestBookingWithPayment(
		fixtures.WithBookingID("bk-bulk-paid"),
		fixtures.WithTripID(tripID),
		fixtures.WithBookingStatus(domain.BookingStatusApproved),
	)
	cash = fixtures.NewTestCashBooking(
		fixtures.WithBookingID("bk-bulk-cash"),
		fixtures.WithTripID(tripID),
		fixtures.WithBookingStatus(domain.BookingStatusPendingApproval),
	)
	return pending, paid, cash
}

// withRefundDue simule la réservation relue après la transition : statut final et motif du
// remboursement dû posé par le repository.
func withRefundDue(b *domain.Booking, status domain.BookingStatus, reason string) *domain.Booking {
	cp := *b
	now := time.Now().UTC()
	cp.Status = status
	cp.CancelledAt = &now
	cp.RefundReason = &reason
	return &cp
}

func TestCancelBookingsForTrip_Payments(t *testing.T) {
	t.Run("supprime la collecte des paymentPending et rembourse les réservations payées", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()

		pending, paid, cash := bulkCancelFixtures("trip-bulk")
		d.writeRepo.On("CancelBookingsForTrip", mock.Anything, "trip-bulk").
			Return([]*domain.Booking{pending, paid, cash}, nil)
		d.tripClient.On("GetTripDetails", mock.Anything, "trip-bulk").Return(nil, nil).Maybe()
		d.readRepo.On("GetByID", mock.Anything, "bk-bulk-paid").
			Return(withRefundDue(paid, domain.BookingStatusCancelled, domain.RefundReasonTripCancelled), nil)
		d.writeRepo.On("MarkRefundRequested", mock.Anything, "bk-bulk-paid").Return(nil)

		cancelled := make(chan string, 1)
		refunded := make(chan string, 1)
		d.paymentClient.On("CancelPaymentForBooking", mock.Anything, "bk-bulk-pending", "Trajet annulé par le chauffeur").
			Return("failed", nil).
			Run(func(args mock.Arguments) { cancelled <- args.String(1) })
		d.paymentClient.On("RequestRefund", mock.Anything, mock.MatchedBy(func(input *client.RefundInput) bool {
			return input.BookingID == "bk-bulk-paid" && input.RefundReason == "tripCancelled" &&
				input.OriginalAmount == paid.Subtotal && input.ServiceFee == paid.ServiceFee
		})).
			Return(nil).
			Run(func(args mock.Arguments) {
				if input, ok := args.Get(1).(*client.RefundInput); ok {
					refunded <- input.BookingID
				}
			})

		count, err := d.svc.CancelBookingsForTrip(ctx, &serviceInterfaces.CancelBookingsForTripInput{TripID: "trip-bulk"})

		require.NoError(t, err)
		assert.Equal(t, 3, count)
		assert.Equal(t, "bk-bulk-pending", waitForAsyncCall(t, cancelled))
		assert.Equal(t, "bk-bulk-paid", waitForAsyncCall(t, refunded))
		d.paymentClient.AssertNumberOfCalls(t, "CancelPaymentForBooking", 1)
		d.paymentClient.AssertNumberOfCalls(t, "RequestRefund", 1)
	})

	t.Run("l'échec de la suppression de la collecte ne bloque pas l'annulation", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()

		pending, _, _ := bulkCancelFixtures("trip-bulk-err")
		d.writeRepo.On("CancelBookingsForTrip", mock.Anything, "trip-bulk-err").
			Return([]*domain.Booking{pending}, nil)

		cancelled := make(chan string, 1)
		d.paymentClient.On("CancelPaymentForBooking", mock.Anything, "bk-bulk-pending", mock.Anything).
			Return("", errors.New("unavailable")).
			Run(func(args mock.Arguments) { cancelled <- args.String(1) })

		count, err := d.svc.CancelBookingsForTrip(ctx, &serviceInterfaces.CancelBookingsForTripInput{TripID: "trip-bulk-err"})

		require.NoError(t, err)
		assert.Equal(t, 1, count)
		waitForAsyncCall(t, cancelled)
		d.paymentClient.AssertNotCalled(t, "RequestRefund", mock.Anything, mock.Anything)
	})
}

func TestCancelBookingsForWaypoint_Payments(t *testing.T) {
	t.Run("supprime la collecte des paymentPending et rembourse les réservations payées", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()

		pending, paid, cash := bulkCancelFixtures("trip-wp")
		d.writeRepo.On("CancelBookingsForWaypoint", mock.Anything, "trip-wp", "wp-1").
			Return([]*domain.Booking{pending, paid, cash}, nil)
		d.tripClient.On("GetTripDetails", mock.Anything, "trip-wp").Return(nil, nil).Maybe()
		d.readRepo.On("GetByID", mock.Anything, "bk-bulk-paid").
			Return(withRefundDue(paid, domain.BookingStatusCancelled, domain.RefundReasonWaypointCancelled), nil)
		d.writeRepo.On("MarkRefundRequested", mock.Anything, "bk-bulk-paid").Return(nil)

		cancelled := make(chan string, 1)
		refunded := make(chan string, 1)
		d.paymentClient.On("CancelPaymentForBooking", mock.Anything, "bk-bulk-pending", "Arrêt supprimé par le chauffeur").
			Return("failed", nil).
			Run(func(args mock.Arguments) { cancelled <- args.String(1) })
		d.paymentClient.On("RequestRefund", mock.Anything, mock.MatchedBy(func(input *client.RefundInput) bool {
			return input.BookingID == "bk-bulk-paid" && input.RefundReason == "waypointCancelled"
		})).
			Return(nil).
			Run(func(args mock.Arguments) {
				if input, ok := args.Get(1).(*client.RefundInput); ok {
					refunded <- input.BookingID
				}
			})

		count, err := d.svc.CancelBookingsForWaypoint(ctx, &serviceInterfaces.CancelBookingsForWaypointInput{
			TripID:     "trip-wp",
			WaypointID: "wp-1",
		})

		require.NoError(t, err)
		assert.Equal(t, 3, count)
		assert.Equal(t, "bk-bulk-pending", waitForAsyncCall(t, cancelled))
		assert.Equal(t, "bk-bulk-paid", waitForAsyncCall(t, refunded))
		d.paymentClient.AssertNumberOfCalls(t, "CancelPaymentForBooking", 1)
		d.paymentClient.AssertNumberOfCalls(t, "RequestRefund", 1)
	})
}

// =============================================================================
// SweepStalePaymentPendingBookings
// =============================================================================

func TestSweepStalePaymentPendingBookings(t *testing.T) {
	t.Run("libère sans paiement, confirme si held, ignore si pending", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()
		impl := service.AsImpl(d.svc)
		require.NotNil(t, impl)

		noPayment := fixtures.NewTestBooking(fixtures.WithBookingID("bk-nopay"), fixtures.WithBookingStatus(domain.BookingStatusPaymentPending))
		held := fixtures.NewTestBooking(fixtures.WithBookingID("bk-held"), fixtures.WithTripID("trip-held"), fixtures.WithBookingStatus(domain.BookingStatusPaymentPending))

		d.readRepo.On("GetStalePaymentPendingBookingIDs", mock.Anything, mock.Anything, mock.Anything).
			Return([]string{"bk-nopay", "bk-held", "bk-pending"}, nil)

		// Aucun paiement → FailPayment
		d.paymentClient.On("GetPaymentStatusByBooking", mock.Anything, "bk-nopay").Return("", nil)
		d.readRepo.On("GetByID", mock.Anything, "bk-nopay").Return(noPayment, nil)
		d.writeRepo.On("FailPayment", mock.Anything, "bk-nopay", "paymentTimeout").Return(nil)
		d.tripClient.On("IncrementLegBookedSeats", mock.Anything, noPayment.TripID, mock.Anything, mock.Anything, -int(noPayment.SeatsBooked)).Return(nil)

		// Paiement held → ConfirmPayment
		d.paymentClient.On("GetPaymentStatusByBooking", mock.Anything, "bk-held").Return("held", nil)
		d.readRepo.On("GetByID", mock.Anything, "bk-held").Return(held, nil)
		d.tripClient.On("GetTripDetails", mock.Anything, "trip-held").Return(&client.TripDetails{TripID: "trip-held", Status: "scheduled"}, nil)
		d.writeRepo.On("ConfirmPayment", mock.Anything, "bk-held", mock.Anything,
			domain.BookingStatusPaymentPending, domain.BookingStatusPendingApproval).Return(nil)

		// Paiement pending → rien
		d.paymentClient.On("GetPaymentStatusByBooking", mock.Anything, "bk-pending").Return("pending", nil)

		impl.SweepStalePaymentPendingBookings(ctx, 12*time.Minute)

		d.writeRepo.AssertExpectations(t)
		d.paymentClient.AssertExpectations(t)
		d.writeRepo.AssertNotCalled(t, "FailPayment", mock.Anything, "bk-pending", mock.Anything)
	})

	t.Run("paiement remboursé (rattachement abandonné) → places libérées", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()
		impl := service.AsImpl(d.svc)
		require.NotNil(t, impl)

		refunded := fixtures.NewTestBooking(
			fixtures.WithBookingID("bk-refunded"),
			fixtures.WithBookingStatus(domain.BookingStatusPaymentPending),
		)

		d.readRepo.On("GetStalePaymentPendingBookingIDs", mock.Anything, mock.Anything, mock.Anything).
			Return([]string{"bk-refunded"}, nil)
		d.paymentClient.On("GetPaymentStatusByBooking", mock.Anything, "bk-refunded").Return("refunded", nil)
		d.readRepo.On("GetByID", mock.Anything, "bk-refunded").Return(refunded, nil)
		d.writeRepo.On("FailPayment", mock.Anything, "bk-refunded", "paymentTimeout").Return(nil)
		d.tripClient.On("IncrementLegBookedSeats", mock.Anything, refunded.TripID,
			mock.Anything, mock.Anything, -int(refunded.SeatsBooked)).Return(nil)

		impl.SweepStalePaymentPendingBookings(ctx, 12*time.Minute)

		d.writeRepo.AssertExpectations(t)
		d.writeRepo.AssertNotCalled(t, "ConfirmPayment",
			mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
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

// =============================================================================
// Vue support : ListBookingsAdmin / GetBookingDetailAdmin
// =============================================================================

func TestListBookingsAdmin(t *testing.T) {
	t.Run("succès : filtres + pagination + enrichissement noms (dédupliqué)", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()

		rows := []*domain.RawAdminBookingPreview{
			{BookingID: "b1", PassengerID: "p1", DriverID: "d1", Status: domain.BookingStatusApproved, TotalAmount: 1500, CreatedAt: time.Now(), DepartureDatetime: time.Now()},
			{BookingID: "b2", PassengerID: "p2", DriverID: "d1", Status: domain.BookingStatusCompleted, TotalAmount: 2000, CreatedAt: time.Now(), DepartureDatetime: time.Now()},
		}
		// Le filtre attendu correspond à l'input mappé.
		expectedFilter := domain.BookingAdminFilter{Status: "approved"}
		d.readRepo.On("ListBookingsAdmin", ctx, expectedFilter, 0, 20).Return(rows, nil)
		d.readRepo.On("CountBookingsAdmin", ctx, expectedFilter).Return(2, nil)

		// 3 userIDs uniques (p1, p2, d1) → 3 appels seulement, pas 4.
		d.userClient.On("GetPassengerInfo", ctx, "p1").Return("Passenger One", true, nil).Once()
		d.userClient.On("GetPassengerInfo", ctx, "p2").Return("Passenger Two", false, nil).Once()
		d.userClient.On("GetPassengerInfo", ctx, "d1").Return("Driver One", true, nil).Once()

		res, err := d.svc.ListBookingsAdmin(ctx, &serviceInterfaces.ListBookingsAdminInput{Status: "approved"})
		require.NoError(t, err)
		assert.Equal(t, 2, res.Total)
		require.Len(t, res.Bookings, 2)
		assert.Equal(t, "Passenger One", res.Bookings[0].PassengerName)
		assert.Equal(t, "Driver One", res.Bookings[0].DriverName)
		assert.Equal(t, "Driver One", res.Bookings[1].DriverName) // dédupliqué
		d.userClient.AssertExpectations(t)
		d.readRepo.AssertExpectations(t)
	})

	t.Run("PageSize plafonné à 100 et défaut 20", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()
		d.readRepo.On("ListBookingsAdmin", ctx, mock.Anything, 0, 100).Return([]*domain.RawAdminBookingPreview{}, nil)
		d.readRepo.On("CountBookingsAdmin", ctx, mock.Anything).Return(0, nil)

		_, err := d.svc.ListBookingsAdmin(ctx, &serviceInterfaces.ListBookingsAdminInput{PageSize: 9999})
		require.NoError(t, err)
		d.readRepo.AssertExpectations(t)
	})

	t.Run("DateFrom invalide → ErrorInvalidInput", func(t *testing.T) {
		d := newTestService()
		_, err := d.svc.ListBookingsAdmin(context.Background(), &serviceInterfaces.ListBookingsAdminInput{DateFrom: "pas-une-date"})
		assert.ErrorIs(t, err, bookingErrors.ErrorInvalidInput)
	})

	t.Run("enrichissement best-effort : nom vide si user-service échoue", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()
		rows := []*domain.RawAdminBookingPreview{
			{BookingID: "b1", PassengerID: "p1", DriverID: "d1", Status: domain.BookingStatusApproved},
		}
		d.readRepo.On("ListBookingsAdmin", ctx, mock.Anything, 0, 20).Return(rows, nil)
		d.readRepo.On("CountBookingsAdmin", ctx, mock.Anything).Return(1, nil)
		d.userClient.On("GetPassengerInfo", ctx, "p1").Return("", false, assert.AnError)
		d.userClient.On("GetPassengerInfo", ctx, "d1").Return("Driver One", true, nil)

		res, err := d.svc.ListBookingsAdmin(ctx, &serviceInterfaces.ListBookingsAdminInput{})
		require.NoError(t, err)
		assert.Equal(t, "", res.Bookings[0].PassengerName)
		assert.Equal(t, "Driver One", res.Bookings[0].DriverName)
	})
}

func TestGetBookingDetailAdmin(t *testing.T) {
	t.Run("succès : détail sans contrôle d'appartenance + noms", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()
		booking := &domain.Booking{BookingID: "b1", PassengerID: "p1", DriverID: "d1", TripID: "t1", Status: domain.BookingStatusApproved}
		d.readRepo.On("GetByIDWithDetails", ctx, "b1").Return(booking, []*domain.Segment{}, []*domain.StatusHistoryEntry{}, nil)
		d.userClient.On("GetPassengerInfo", ctx, "p1").Return("Passenger One", true, nil)
		d.userClient.On("GetPassengerInfo", ctx, "d1").Return("Driver One", true, nil)
		d.tripClient.On("GetTripDetails", ctx, "t1").Return(&client.TripDetails{TripID: "t1", RoutePolyline: "abc123"}, nil)

		res, err := d.svc.GetBookingDetailAdmin(ctx, "b1")
		require.NoError(t, err)
		assert.Equal(t, "b1", res.Booking.BookingID)
		assert.Equal(t, "Passenger One", res.PassengerName)
		assert.Equal(t, "Driver One", res.DriverName)
		assert.Equal(t, "abc123", res.Booking.RoutePolyline)
	})

	t.Run("polyline best-effort : trips-service en erreur → polyline vide, pas d'échec", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()
		booking := &domain.Booking{BookingID: "b1", PassengerID: "p1", DriverID: "d1", TripID: "t1", Status: domain.BookingStatusApproved}
		d.readRepo.On("GetByIDWithDetails", ctx, "b1").Return(booking, []*domain.Segment{}, []*domain.StatusHistoryEntry{}, nil)
		d.userClient.On("GetPassengerInfo", ctx, "p1").Return("Passenger One", true, nil)
		d.userClient.On("GetPassengerInfo", ctx, "d1").Return("Driver One", true, nil)
		d.tripClient.On("GetTripDetails", ctx, "t1").Return((*client.TripDetails)(nil), assert.AnError)

		res, err := d.svc.GetBookingDetailAdmin(ctx, "b1")
		require.NoError(t, err)
		assert.Empty(t, res.Booking.RoutePolyline)
	})

	t.Run("historique enrichi des noms d'auteurs (seed passager/conducteur, vide pour system)", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()
		booking := &domain.Booking{BookingID: "b1", PassengerID: "p1", DriverID: "d1", TripID: "t1", Status: domain.BookingStatusApproved}
		history := []*domain.StatusHistoryEntry{
			{HistoryID: "h1", PreviousStatus: "pendingApproval", NewStatus: "approved", ChangedBy: "d1", ChangedByType: "driver"},
			{HistoryID: "h2", PreviousStatus: "approved", NewStatus: "inProgress", ChangedBy: "system", ChangedByType: "system"},
			{HistoryID: "h3", PreviousStatus: "created", NewStatus: "pendingApproval", ChangedBy: "p1", ChangedByType: "passenger"},
		}
		d.readRepo.On("GetByIDWithDetails", ctx, "b1").Return(booking, []*domain.Segment{}, history, nil)
		// Seed couvre p1 et d1 → aucun appel user-service supplémentaire pour l'historique.
		d.userClient.On("GetPassengerInfo", ctx, "p1").Return("Passenger One", true, nil).Once()
		d.userClient.On("GetPassengerInfo", ctx, "d1").Return("Driver One", true, nil).Once()
		d.tripClient.On("GetTripDetails", ctx, "t1").Return(&client.TripDetails{TripID: "t1"}, nil)

		res, err := d.svc.GetBookingDetailAdmin(ctx, "b1")
		require.NoError(t, err)
		require.Len(t, res.Booking.History, 3)
		assert.Equal(t, "Driver One", res.Booking.History[0].ChangedByName)
		assert.Empty(t, res.Booking.History[1].ChangedByName)
		assert.Equal(t, "Passenger One", res.Booking.History[2].ChangedByName)
		d.userClient.AssertExpectations(t)
	})

	t.Run("bookingID vide → ErrorInvalidInput", func(t *testing.T) {
		d := newTestService()
		_, err := d.svc.GetBookingDetailAdmin(context.Background(), "")
		assert.ErrorIs(t, err, bookingErrors.ErrorInvalidInput)
	})

	t.Run("introuvable → erreur propagée", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()
		d.readRepo.On("GetByIDWithDetails", ctx, "x").Return((*domain.Booking)(nil), []*domain.Segment(nil), []*domain.StatusHistoryEntry(nil), bookingErrors.ErrorBookingNotFound)
		_, err := d.svc.GetBookingDetailAdmin(ctx, "x")
		assert.ErrorIs(t, err, bookingErrors.ErrorBookingNotFound)
	})
}

// =============================================================================
// GetActivePassengerSummariesForTrip
// =============================================================================

func TestGetActivePassengerSummariesForTrip(t *testing.T) {
	t.Run("succes - le chauffeur du trajet voit ses passagers", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()

		d.tripClient.On("GetTripDetails", mock.Anything, "trip-sum").
			Return(&client.TripDetails{TripID: "trip-sum", DriverID: "driver-sum"}, nil)
		d.readRepo.On("GetActivePassengerSummariesForTrip", mock.Anything, "trip-sum").
			Return([]*domain.RawPassengerSummary{}, nil)

		results, err := d.svc.GetActivePassengerSummariesForTrip(ctx, "trip-sum", "driver-sum")

		require.NoError(t, err)
		assert.Empty(t, results)
		d.readRepo.AssertExpectations(t)
	})

	t.Run("erreur - un autre utilisateur ne peut pas lire les passagers", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()

		d.tripClient.On("GetTripDetails", mock.Anything, "trip-sum").
			Return(&client.TripDetails{TripID: "trip-sum", DriverID: "driver-sum"}, nil)

		_, err := d.svc.GetActivePassengerSummariesForTrip(ctx, "trip-sum", "intrus")

		assert.ErrorIs(t, err, bookingErrors.ErrorUnauthorized)
		d.readRepo.AssertNotCalled(t, "GetActivePassengerSummariesForTrip", mock.Anything, mock.Anything)
	})

	t.Run("erreur - trajet introuvable", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()

		d.tripClient.On("GetTripDetails", mock.Anything, "trip-absent").Return(nil, nil)

		_, err := d.svc.GetActivePassengerSummariesForTrip(ctx, "trip-absent", "driver-sum")

		assert.ErrorIs(t, err, bookingErrors.ErrorTripNotFound)
	})
}
