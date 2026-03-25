package implementations

import (
	"context"
	"errors"

	"github.com/Kpeewu/tissi-mah/services/booking-service/internal/domain"
	i "github.com/Kpeewu/tissi-mah/services/booking-service/internal/repository/interfaces"
	bookingErrors "github.com/Kpeewu/tissi-mah/services/booking-service/pkg/errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

const pageSize = 10

type bookingReadRepositoryImpl struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

func NewBookingReadRepository(pool *pgxpool.Pool, logger *zap.Logger) i.BookingRepositoryRead {
	return &bookingReadRepositoryImpl{pool: pool, logger: logger}
}

// GetByID retourne une réservation par son ID (sans segments ni historique).
func (r *bookingReadRepositoryImpl) GetByID(ctx context.Context, bookingID string) (*domain.Booking, error) {
	query := `
		SELECT booking_id, booking_reference, trip_id, passenger_id, driver_id,
		       pickup_waypoint_id, dropoff_waypoint_id, seats_booked,
		       price_per_seat, subtotal, service_fee, total_amount,
		       payment_method, status,
		       payment_completed_at, approved_at, rejected_at, cancelled_at, completed_at,
		       canceller_id, cancellation_reason,
		       no_show_type, no_show_reported_by, no_show_reported_at, no_show_description,
		       created_at, updated_at
		FROM bookings
		WHERE booking_id = $1`

	b := &domain.Booking{}
	err := r.pool.QueryRow(ctx, query, bookingID).Scan(
		&b.BookingID, &b.BookingReference, &b.TripID, &b.PassengerID, &b.DriverID,
		&b.PickupWaypointID, &b.DropoffWaypointID, &b.SeatsBooked,
		&b.PricePerSeat, &b.Subtotal, &b.ServiceFee, &b.TotalAmount,
		&b.PaymentMethod, &b.Status,
		&b.PaymentCompletedAt, &b.ApprovedAt, &b.RejectedAt, &b.CancelledAt, &b.CompletedAt,
		&b.CancellerID, &b.CancellationReason,
		&b.NoShowType, &b.NoShowReportedBy, &b.NoShowReportedAt, &b.NoShowDescription,
		&b.CreatedAt, &b.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, bookingErrors.ErrorBookingNotFound
		}
		r.logger.Error("GetByID failed", zap.Error(err), zap.String("bookingID", bookingID))
		return nil, bookingErrors.ErrorDataRetrievalFailed
	}

	return b, nil
}

// GetByIDWithDetails retourne une réservation avec ses segments et son historique.
func (r *bookingReadRepositoryImpl) GetByIDWithDetails(ctx context.Context, bookingID string) (*domain.Booking, []*domain.Segment, []*domain.StatusHistoryEntry, error) {
	// Récupérer la réservation
	booking, err := r.GetByID(ctx, bookingID)
	if err != nil {
		return nil, nil, nil, err
	}

	// Récupérer les segments
	segments, err := r.getSegments(ctx, bookingID)
	if err != nil {
		return nil, nil, nil, err
	}

	// Récupérer l'historique
	history, err := r.getStatusHistory(ctx, bookingID)
	if err != nil {
		return nil, nil, nil, err
	}

	return booking, segments, history, nil
}

// getSegments récupère les segments d'une réservation.
func (r *bookingReadRepositoryImpl) getSegments(ctx context.Context, bookingID string) ([]*domain.Segment, error) {
	query := `
		SELECT segment_id, booking_id, pickup_waypoint_id, dropoff_waypoint_id,
		       pickup_location_name, pickup_city, pickup_lat, pickup_lng,
		       pickup_scheduled_at, pickup_actual_at,
		       dropoff_location_name, dropoff_city, dropoff_lat, dropoff_lng,
		       dropoff_scheduled_at, dropoff_actual_at,
		       segment_distance_meters, segment_duration_minutes, segment_price,
		       created_at
		FROM bookings_segments
		WHERE booking_id = $1
		ORDER BY created_at ASC`

	rows, err := r.pool.Query(ctx, query, bookingID)
	if err != nil {
		r.logger.Error("getSegments failed", zap.Error(err), zap.String("bookingID", bookingID))
		return nil, bookingErrors.ErrorDataRetrievalFailed
	}
	defer rows.Close()

	var segments []*domain.Segment
	for rows.Next() {
		s := &domain.Segment{}
		if err := rows.Scan(
			&s.SegmentID, &s.BookingID, &s.PickupWaypointID, &s.DropoffWaypointID,
			&s.PickupLocationName, &s.PickupCity, &s.PickupLat, &s.PickupLng,
			&s.PickupScheduledAt, &s.PickupActualAt,
			&s.DropoffLocationName, &s.DropoffCity, &s.DropoffLat, &s.DropoffLng,
			&s.DropoffScheduledAt, &s.DropoffActualAt,
			&s.SegmentDistanceMeters, &s.SegmentDurationMinutes, &s.SegmentPrice,
			&s.CreatedAt,
		); err != nil {
			r.logger.Error("getSegments scan failed", zap.Error(err))
			return nil, bookingErrors.ErrorDataRetrievalFailed
		}
		segments = append(segments, s)
	}

	return segments, nil
}

// getStatusHistory récupère l'historique des changements de statut.
func (r *bookingReadRepositoryImpl) getStatusHistory(ctx context.Context, bookingID string) ([]*domain.StatusHistoryEntry, error) {
	query := `
		SELECT history_id, booking_id, previous_status, new_status,
		       changed_by, changed_by_type, change_reason, metadata, created_at
		FROM bookings_status_history
		WHERE booking_id = $1
		ORDER BY created_at DESC`

	rows, err := r.pool.Query(ctx, query, bookingID)
	if err != nil {
		r.logger.Error("getStatusHistory failed", zap.Error(err), zap.String("bookingID", bookingID))
		return nil, bookingErrors.ErrorDataRetrievalFailed
	}
	defer rows.Close()

	var history []*domain.StatusHistoryEntry
	for rows.Next() {
		h := &domain.StatusHistoryEntry{}
		if err := rows.Scan(
			&h.HistoryID, &h.BookingID, &h.PreviousStatus, &h.NewStatus,
			&h.ChangedBy, &h.ChangedByType, &h.ChangeReason, &h.Metadata, &h.CreatedAt,
		); err != nil {
			r.logger.Error("getStatusHistory scan failed", zap.Error(err))
			return nil, bookingErrors.ErrorDataRetrievalFailed
		}
		history = append(history, h)
	}

	return history, nil
}

// GetPassengerBookings retourne la liste paginée des réservations d'un passager.
func (r *bookingReadRepositoryImpl) GetPassengerBookings(ctx context.Context, passengerID string, pageIndex int, statusFilter string) ([]*domain.BookingPreview, error) {
	offset := pageIndex * pageSize

	var query string
	var args []interface{}

	if statusFilter != "" {
		query = `
			SELECT b.booking_id, b.booking_reference, b.trip_id, b.status,
			       b.seats_booked, b.total_amount,
			       COALESCE(s_pickup.pickup_location_name, '') AS pickup_location_name,
			       COALESCE(s_dropoff.dropoff_location_name, '') AS dropoff_location_name,
			       b.created_at
			FROM bookings b
			LEFT JOIN LATERAL (
				SELECT pickup_location_name FROM bookings_segments
				WHERE booking_id = b.booking_id ORDER BY created_at ASC LIMIT 1
			) s_pickup ON true
			LEFT JOIN LATERAL (
				SELECT dropoff_location_name FROM bookings_segments
				WHERE booking_id = b.booking_id ORDER BY created_at DESC LIMIT 1
			) s_dropoff ON true
			WHERE b.passenger_id = $1 AND b.status = $2::booking_status
			ORDER BY b.created_at DESC
			LIMIT $3 OFFSET $4`
		args = []interface{}{passengerID, statusFilter, pageSize, offset}
	} else {
		query = `
			SELECT b.booking_id, b.booking_reference, b.trip_id, b.status,
			       b.seats_booked, b.total_amount,
			       COALESCE(s_pickup.pickup_location_name, '') AS pickup_location_name,
			       COALESCE(s_dropoff.dropoff_location_name, '') AS dropoff_location_name,
			       b.created_at
			FROM bookings b
			LEFT JOIN LATERAL (
				SELECT pickup_location_name FROM bookings_segments
				WHERE booking_id = b.booking_id ORDER BY created_at ASC LIMIT 1
			) s_pickup ON true
			LEFT JOIN LATERAL (
				SELECT dropoff_location_name FROM bookings_segments
				WHERE booking_id = b.booking_id ORDER BY created_at DESC LIMIT 1
			) s_dropoff ON true
			WHERE b.passenger_id = $1
			ORDER BY b.created_at DESC
			LIMIT $2 OFFSET $3`
		args = []interface{}{passengerID, pageSize, offset}
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		r.logger.Error("GetPassengerBookings failed", zap.Error(err), zap.String("passengerID", passengerID))
		return nil, bookingErrors.ErrorDataRetrievalFailed
	}
	defer rows.Close()

	return r.scanBookingPreviews(rows)
}

// GetDriverTripBookings retourne la liste paginée des réservations pour un trajet du conducteur.
func (r *bookingReadRepositoryImpl) GetDriverTripBookings(ctx context.Context, driverID, tripID string, pageIndex int) ([]*domain.BookingPreview, error) {
	offset := pageIndex * pageSize

	query := `
		SELECT b.booking_id, b.booking_reference, b.trip_id, b.status,
		       b.seats_booked, b.total_amount,
		       COALESCE(s_pickup.pickup_location_name, '') AS pickup_location_name,
		       COALESCE(s_dropoff.dropoff_location_name, '') AS dropoff_location_name,
		       b.created_at
		FROM bookings b
		LEFT JOIN LATERAL (
			SELECT pickup_location_name FROM bookings_segments
			WHERE booking_id = b.booking_id ORDER BY created_at ASC LIMIT 1
		) s_pickup ON true
		LEFT JOIN LATERAL (
			SELECT dropoff_location_name FROM bookings_segments
			WHERE booking_id = b.booking_id ORDER BY created_at DESC LIMIT 1
		) s_dropoff ON true
		WHERE b.driver_id = $1 AND b.trip_id = $2
		ORDER BY b.created_at DESC
		LIMIT $3 OFFSET $4`

	rows, err := r.pool.Query(ctx, query, driverID, tripID, pageSize, offset)
	if err != nil {
		r.logger.Error("GetDriverTripBookings failed", zap.Error(err))
		return nil, bookingErrors.ErrorDataRetrievalFailed
	}
	defer rows.Close()

	return r.scanBookingPreviews(rows)
}

// scanBookingPreviews scanne les résultats en BookingPreview.
func (r *bookingReadRepositoryImpl) scanBookingPreviews(rows pgx.Rows) ([]*domain.BookingPreview, error) {
	var previews []*domain.BookingPreview
	for rows.Next() {
		p := &domain.BookingPreview{}
		if err := rows.Scan(
			&p.BookingID, &p.BookingReference, &p.TripID, &p.Status,
			&p.SeatsBooked, &p.TotalAmount,
			&p.PickupLocationName, &p.DropoffLocationName,
			&p.DepartureDatetime,
		); err != nil {
			r.logger.Error("scanBookingPreviews failed", zap.Error(err))
			return nil, bookingErrors.ErrorDataRetrievalFailed
		}
		previews = append(previews, p)
	}
	return previews, nil
}

// HasActiveBooking vérifie si un passager a déjà une réservation active pour un trajet.
func (r *bookingReadRepositoryImpl) HasActiveBooking(ctx context.Context, passengerID, tripID string) (bool, error) {
	query := `
		SELECT EXISTS (
			SELECT 1 FROM bookings
			WHERE passenger_id = $1 AND trip_id = $2
			AND status IN ('created', 'paymentPending', 'pendingApproval', 'approved', 'inProgress')
		)`

	var exists bool
	if err := r.pool.QueryRow(ctx, query, passengerID, tripID).Scan(&exists); err != nil {
		r.logger.Error("HasActiveBooking failed", zap.Error(err))
		return false, bookingErrors.ErrorDataRetrievalFailed
	}

	return exists, nil
}

// GetActiveBookingsSeatsForTrip retourne le total des places réservées pour un trajet parmi les bookings actifs.
func (r *bookingReadRepositoryImpl) GetActiveBookingsSeatsForTrip(ctx context.Context, tripID string) (int, error) {
	query := `
		SELECT COALESCE(SUM(seats_booked), 0)
		FROM bookings
		WHERE trip_id = $1
		AND status IN ('created', 'paymentPending', 'pendingApproval', 'approved', 'inProgress')`

	var total int
	if err := r.pool.QueryRow(ctx, query, tripID).Scan(&total); err != nil {
		r.logger.Error("GetActiveBookingsSeatsForTrip failed", zap.Error(err), zap.String("tripID", tripID))
		return 0, bookingErrors.ErrorDataRetrievalFailed
	}

	return total, nil
}

// GetActiveTripsWithBookings retourne la liste des tripIDs ayant des bookings actifs.
func (r *bookingReadRepositoryImpl) GetActiveTripsWithBookings(ctx context.Context) ([]string, error) {
	query := `
		SELECT DISTINCT trip_id
		FROM bookings
		WHERE status IN ('created', 'paymentPending', 'pendingApproval', 'approved', 'inProgress')`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		r.logger.Error("GetActiveTripsWithBookings failed", zap.Error(err))
		return nil, bookingErrors.ErrorDataRetrievalFailed
	}
	defer rows.Close()

	var tripIDs []string
	for rows.Next() {
		var tripID string
		if err := rows.Scan(&tripID); err != nil {
			r.logger.Error("GetActiveTripsWithBookings scan failed", zap.Error(err))
			return nil, bookingErrors.ErrorDataRetrievalFailed
		}
		tripIDs = append(tripIDs, tripID)
	}

	return tripIDs, nil
}
