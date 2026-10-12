package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/Kpeewu/tissi-mah/services/booking-service/fixtures"
	"github.com/Kpeewu/tissi-mah/services/booking-service/internal/client"
	"github.com/Kpeewu/tissi-mah/services/booking-service/internal/domain"
	"github.com/Kpeewu/tissi-mah/services/booking-service/internal/service"
)

// refundDueBooking construit une réservation annulée dont le remboursement n'a pas été acquitté.
func refundDueBooking(bookingID, reason string) *domain.Booking {
	b := fixtures.NewTestBookingWithPayment(
		fixtures.WithBookingID(bookingID),
		fixtures.WithTripID("trip-refund"),
	)
	return withRefundDue(b, domain.BookingStatusCancelled, reason)
}

// expectPendingRefunds configure le lot retourné au worker.
func (d *testDeps) expectPendingRefunds(bookings ...*domain.Booking) {
	d.readRepo.On("GetBookingsPendingRefundRequest", mock.Anything, mock.Anything, 5, 100).Return(bookings, nil)
}

// expectDeparture configure la date de départ du waypoint de pickup.
func (d *testDeps) expectDeparture(b *domain.Booking, departure string) {
	d.tripClient.On("GetTripDetails", mock.Anything, b.TripID).Return(&client.TripDetails{
		TripID: b.TripID,
		Waypoints: []client.TripWaypoint{
			{WaypointID: b.PickupWaypointID, ScheduledPickupDatetime: departure},
		},
	}, nil)
}

func TestProcessRefundRequests(t *testing.T) {
	departure := time.Now().UTC().Add(48 * time.Hour).Format(time.RFC3339)

	t.Run("succès : demande envoyée avec le motif enregistré puis acquittée", func(t *testing.T) {
		d := newTestService()
		b := refundDueBooking("bk-refund-ok", domain.RefundReasonTripCancelled)
		d.expectPendingRefunds(b)
		d.expectDeparture(b, departure)
		d.paymentClient.On("RequestRefund", mock.Anything, mock.MatchedBy(func(in *client.RefundInput) bool {
			return in.BookingID == "bk-refund-ok" &&
				in.RefundReason == domain.RefundReasonTripCancelled &&
				in.DepartureDatetime == departure &&
				in.CancelledAt == b.CancelledAt.Format(time.RFC3339) &&
				in.OriginalAmount == b.Subtotal && in.ServiceFee == b.ServiceFee
		})).Return(nil)
		d.writeRepo.On("MarkRefundRequested", mock.Anything, "bk-refund-ok").Return(nil)

		service.AsImpl(d.svc).ProcessRefundRequests(context.Background())

		d.writeRepo.AssertCalled(t, "MarkRefundRequested", mock.Anything, "bk-refund-ok")
		d.writeRepo.AssertNotCalled(t, "IncrementRefundRequestAttempts", mock.Anything, mock.Anything)
	})

	t.Run("payment-service indisponible : rejeu au prochain tick sans compteur", func(t *testing.T) {
		d := newTestService()
		b := refundDueBooking("bk-refund-down", domain.RefundReasonCancelledByDriver)
		d.expectPendingRefunds(b)
		d.expectDeparture(b, departure)
		d.paymentClient.On("RequestRefund", mock.Anything, mock.Anything).
			Return(status.Error(codes.Unavailable, "connection refused"))

		service.AsImpl(d.svc).ProcessRefundRequests(context.Background())

		d.writeRepo.AssertNotCalled(t, "MarkRefundRequested", mock.Anything, mock.Anything)
		d.writeRepo.AssertNotCalled(t, "IncrementRefundRequestAttempts", mock.Anything, mock.Anything)
	})

	t.Run("refus définitif : tentative comptée, le passage par l'abandon ne bloque pas le lot", func(t *testing.T) {
		d := newTestService()
		rejected := refundDueBooking("bk-refund-rejected", domain.RefundReasonNoShowDriver)
		abandoned := refundDueBooking("bk-refund-abandoned", domain.RefundReasonNoShowDriver)
		d.expectPendingRefunds(rejected, abandoned)
		d.expectDeparture(rejected, departure)
		d.paymentClient.On("RequestRefund", mock.Anything, mock.Anything).
			Return(errors.Join(errors.New("payment-service: RequestRefund failed"),
				status.Error(codes.FailedPrecondition, "ErrorInvalidPaymentStatus")))
		d.writeRepo.On("IncrementRefundRequestAttempts", mock.Anything, "bk-refund-rejected").Return(1, nil)
		d.writeRepo.On("IncrementRefundRequestAttempts", mock.Anything, "bk-refund-abandoned").Return(5, nil)

		service.AsImpl(d.svc).ProcessRefundRequests(context.Background())

		d.writeRepo.AssertNumberOfCalls(t, "IncrementRefundRequestAttempts", 2)
		d.writeRepo.AssertNotCalled(t, "MarkRefundRequested", mock.Anything, mock.Anything)
	})

	t.Run("annulation passager sans date de départ : pas d'appel, rejeu", func(t *testing.T) {
		d := newTestService()
		b := refundDueBooking("bk-refund-pax", domain.RefundReasonCancelledByPassenger)
		d.expectPendingRefunds(b)
		d.tripClient.On("GetTripDetails", mock.Anything, b.TripID).Return(nil, errors.New("trips-service down"))

		service.AsImpl(d.svc).ProcessRefundRequests(context.Background())

		d.paymentClient.AssertNotCalled(t, "RequestRefund", mock.Anything, mock.Anything)
		d.writeRepo.AssertNotCalled(t, "IncrementRefundRequestAttempts", mock.Anything, mock.Anything)
	})

	t.Run("arrêt supprimé sans date de départ : demande envoyée sans départ", func(t *testing.T) {
		d := newTestService()
		b := refundDueBooking("bk-refund-wp", domain.RefundReasonWaypointCancelled)
		d.expectPendingRefunds(b)
		// Le waypoint de pickup supprimé n'est plus retourné par trips-service
		d.tripClient.On("GetTripDetails", mock.Anything, b.TripID).Return(&client.TripDetails{TripID: b.TripID}, nil)
		d.paymentClient.On("RequestRefund", mock.Anything, mock.MatchedBy(func(in *client.RefundInput) bool {
			return in.RefundReason == domain.RefundReasonWaypointCancelled && in.DepartureDatetime == ""
		})).Return(nil)
		d.writeRepo.On("MarkRefundRequested", mock.Anything, "bk-refund-wp").Return(nil)

		service.AsImpl(d.svc).ProcessRefundRequests(context.Background())

		d.writeRepo.AssertCalled(t, "MarkRefundRequested", mock.Anything, "bk-refund-wp")
	})

	t.Run("rejet : la date transmise est celle du rejet", func(t *testing.T) {
		d := newTestService()
		rejectedAt := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
		b := refundDueBooking("bk-refund-reject", domain.RefundReasonBookingRejected)
		b.Status, b.CancelledAt, b.RejectedAt = domain.BookingStatusRejected, nil, &rejectedAt
		d.expectPendingRefunds(b)
		d.expectDeparture(b, departure)
		d.paymentClient.On("RequestRefund", mock.Anything, mock.MatchedBy(func(in *client.RefundInput) bool {
			return in.CancelledAt == rejectedAt.Format(time.RFC3339)
		})).Return(nil)
		d.writeRepo.On("MarkRefundRequested", mock.Anything, "bk-refund-reject").Return(nil)

		service.AsImpl(d.svc).ProcessRefundRequests(context.Background())

		d.writeRepo.AssertCalled(t, "MarkRefundRequested", mock.Anything, "bk-refund-reject")
	})

	t.Run("aucune demande en attente", func(t *testing.T) {
		d := newTestService()
		d.expectPendingRefunds()

		service.AsImpl(d.svc).ProcessRefundRequests(context.Background())

		d.paymentClient.AssertNotCalled(t, "RequestRefund", mock.Anything, mock.Anything)
	})
}
