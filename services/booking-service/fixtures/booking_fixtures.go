package fixtures

import (
	"context"
	"time"

	"github.com/Kpeewu/tissi-mah/services/booking-service/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type BookingOption func(*domain.Booking)

func WithBookingID(id string) BookingOption {
	return func(b *domain.Booking) { b.BookingID = id }
}

func WithTripID(id string) BookingOption {
	return func(b *domain.Booking) { b.TripID = id }
}

func WithPassengerID(id string) BookingOption {
	return func(b *domain.Booking) { b.PassengerID = id }
}

func WithDriverID(id string) BookingOption {
	return func(b *domain.Booking) { b.DriverID = id }
}

func WithBookingStatus(status domain.BookingStatus) BookingOption {
	return func(b *domain.Booking) { b.Status = status }
}

func WithPaymentMethod(method domain.PaymentMethod) BookingOption {
	return func(b *domain.Booking) { b.PaymentMethod = method }
}

func WithSubtotal(amount int) BookingOption {
	return func(b *domain.Booking) { b.Subtotal = amount }
}

func WithServiceFee(amount int) BookingOption {
	return func(b *domain.Booking) { b.ServiceFee = amount }
}

func WithTotalAmount(amount int) BookingOption {
	return func(b *domain.Booking) { b.TotalAmount = amount }
}

func WithSeatsBooked(seats int16) BookingOption {
	return func(b *domain.Booking) { b.SeatsBooked = seats }
}

func WithPaymentCompletedAt(t time.Time) BookingOption {
	return func(b *domain.Booking) { b.PaymentCompletedAt = &t }
}

func WithNoPaymentCompleted() BookingOption {
	return func(b *domain.Booking) { b.PaymentCompletedAt = nil }
}

func WithApprovedAt(t time.Time) BookingOption {
	return func(b *domain.Booking) { b.ApprovedAt = &t }
}

func WithCancelledAt(t time.Time) BookingOption {
	return func(b *domain.Booking) { b.CancelledAt = &t }
}

func WithCompletedAt(t time.Time) BookingOption {
	return func(b *domain.Booking) { b.CompletedAt = &t }
}

func WithPickupWaypointID(id string) BookingOption {
	return func(b *domain.Booking) { b.PickupWaypointID = id }
}

func WithDropoffWaypointID(id string) BookingOption {
	return func(b *domain.Booking) { b.DropoffWaypointID = id }
}

func WithCancellerID(id string) BookingOption {
	return func(b *domain.Booking) { b.CancellerID = &id }
}

func WithCancellationReason(reason string) BookingOption {
	return func(b *domain.Booking) { b.CancellationReason = &reason }
}

// NewTestBooking cree un Booking avec des valeurs par defaut
func NewTestBooking(opts ...BookingOption) *domain.Booking {
	now := time.Now().UTC()
	id := uuid.New().String()

	booking := &domain.Booking{
		BookingID:         id,
		BookingReference:  "BK-" + id[:6],
		TripID:            uuid.New().String(),
		PassengerID:       uuid.New().String(),
		DriverID:          uuid.New().String(),
		PickupWaypointID:  uuid.New().String(),
		DropoffWaypointID: uuid.New().String(),
		SeatsBooked:       1,
		PricePerSeat:      5000,
		Subtotal:          5000,
		ServiceFee:        500,
		TotalAmount:       5500,
		PaymentMethod:     domain.PaymentMobileMoney,
		Status:            domain.BookingStatusPendingApproval,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	for _, opt := range opts {
		opt(booking)
	}

	return booking
}

// NewTestBookingWithPayment cree un booking avec paiement confirme
func NewTestBookingWithPayment(opts ...BookingOption) *domain.Booking {
	now := time.Now().UTC()
	allOpts := append([]BookingOption{
		WithPaymentCompletedAt(now),
		WithPaymentMethod(domain.PaymentMobileMoney),
	}, opts...)
	return NewTestBooking(allOpts...)
}

// NewTestCashBooking cree un booking en cash (sans paiement en ligne)
func NewTestCashBooking(opts ...BookingOption) *domain.Booking {
	allOpts := append([]BookingOption{
		WithPaymentMethod(domain.PaymentCash),
	}, opts...)
	return NewTestBooking(allOpts...)
}

// =============================================================================
// DB Insert helpers (pour tests d'integration)
// =============================================================================

// InsertBooking insere un Booking dans la base de donnees de test
func InsertBooking(ctx context.Context, pool *pgxpool.Pool, b *domain.Booking) error {
	query := `INSERT INTO bookings (
		booking_id, booking_reference, trip_id, passenger_id, driver_id,
		pickup_waypoint_id, dropoff_waypoint_id, seats_booked, price_per_seat,
		subtotal, service_fee, total_amount, payment_method, status,
		payment_completed_at, approved_at, rejected_at, cancelled_at, completed_at,
		canceller_id, cancellation_reason, no_show_type, no_show_reported_by,
		no_show_reported_at, no_show_description
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14,
		$15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25)`

	_, err := pool.Exec(ctx, query,
		b.BookingID, b.BookingReference, b.TripID, b.PassengerID, b.DriverID,
		b.PickupWaypointID, b.DropoffWaypointID, b.SeatsBooked, b.PricePerSeat,
		b.Subtotal, b.ServiceFee, b.TotalAmount, b.PaymentMethod, b.Status,
		b.PaymentCompletedAt, b.ApprovedAt, b.RejectedAt, b.CancelledAt, b.CompletedAt,
		b.CancellerID, b.CancellationReason, b.NoShowType, b.NoShowReportedBy,
		b.NoShowReportedAt, b.NoShowDescription,
	)
	return err
}

// InsertBookingWithHistory insere un Booking avec une entree d'historique initiale
func InsertBookingWithHistory(ctx context.Context, pool *pgxpool.Pool, b *domain.Booking) error {
	if err := InsertBooking(ctx, pool, b); err != nil {
		return err
	}

	historyQuery := `INSERT INTO bookings_status_history (
		history_id, booking_id, previous_status, new_status,
		changed_by, changed_by_type, change_reason
	) VALUES ($1, $2, '', $3, $4, 'system', 'creation')`

	_, err := pool.Exec(ctx, historyQuery,
		uuid.New().String(), b.BookingID, string(b.Status), b.PassengerID,
	)
	return err
}
