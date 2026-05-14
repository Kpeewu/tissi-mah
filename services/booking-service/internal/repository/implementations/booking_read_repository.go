package implementations

import (
	"context"
	"errors"
	"time"

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
		       pickup_waypoint_id, dropoff_waypoint_id,
		       pickup_sequencer_order, dropoff_sequencer_order,
		       seats_booked,
		       price_per_seat, subtotal, service_fee, total_amount,
		       payment_method, status,
		       payment_completed_at, approved_at, rejected_at, cancelled_at, completed_at,
		       canceller_id, cancellation_reason,
		       no_show_type, no_show_reported_by, no_show_reported_at, no_show_description,
		       payment_released_at, created_at, updated_at
		FROM bookings
		WHERE booking_id = $1`

	b := &domain.Booking{}
	err := r.pool.QueryRow(ctx, query, bookingID).Scan(
		&b.BookingID, &b.BookingReference, &b.TripID, &b.PassengerID, &b.DriverID,
		&b.PickupWaypointID, &b.DropoffWaypointID,
		&b.PickupSequencerOrder, &b.DropoffSequencerOrder,
		&b.SeatsBooked,
		&b.PricePerSeat, &b.Subtotal, &b.ServiceFee, &b.TotalAmount,
		&b.PaymentMethod, &b.Status,
		&b.PaymentCompletedAt, &b.ApprovedAt, &b.RejectedAt, &b.CancelledAt, &b.CompletedAt,
		&b.CancellerID, &b.CancellationReason,
		&b.NoShowType, &b.NoShowReportedBy, &b.NoShowReportedAt, &b.NoShowDescription,
		&b.PaymentReleasedAt, &b.CreatedAt, &b.UpdatedAt,
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

// GetSegmentOccupancy retourne le nombre de places occupées pour un segment donné.
func (r *bookingReadRepositoryImpl) GetSegmentOccupancy(ctx context.Context, tripID string, segmentOrder int) (int, error) {
	query := `
		SELECT COALESCE(SUM(seats_booked), 0)
		FROM bookings
		WHERE trip_id = $1
		AND pickup_sequencer_order <= $2
		AND dropoff_sequencer_order > $2
		AND status IN ('created', 'paymentPending', 'pendingApproval', 'approved', 'inProgress')`

	var total int
	if err := r.pool.QueryRow(ctx, query, tripID, segmentOrder).Scan(&total); err != nil {
		r.logger.Error("GetSegmentOccupancy failed", zap.Error(err), zap.String("tripID", tripID))
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

// GetCompletedBookingsPendingRelease retourne les bookings complétés non-cash
// dont le paiement n'a pas encore été libéré et dont la complétion est antérieure à completedBefore.
func (r *bookingReadRepositoryImpl) GetCompletedBookingsPendingRelease(ctx context.Context, completedBefore time.Time) ([]*domain.Booking, error) {
	query := `
		SELECT booking_id, booking_reference, trip_id, passenger_id, driver_id,
		       pickup_waypoint_id, dropoff_waypoint_id, seats_booked,
		       price_per_seat, subtotal, service_fee, total_amount,
		       payment_method, status,
		       payment_completed_at, approved_at, rejected_at, cancelled_at, completed_at,
		       canceller_id, cancellation_reason,
		       no_show_type, no_show_reported_by, no_show_reported_at, no_show_description,
		       payment_released_at, created_at, updated_at
		FROM bookings
		WHERE status = 'completed'
		  AND payment_method != 'cash'
		  AND payment_completed_at IS NOT NULL
		  AND completed_at <= $1
		  AND payment_released_at IS NULL`

	rows, err := r.pool.Query(ctx, query, completedBefore)
	if err != nil {
		r.logger.Error("GetCompletedBookingsPendingRelease failed", zap.Error(err))
		return nil, bookingErrors.ErrorDataRetrievalFailed
	}
	defer rows.Close()

	var bookings []*domain.Booking
	for rows.Next() {
		b := &domain.Booking{}
		if err := rows.Scan(
			&b.BookingID, &b.BookingReference, &b.TripID, &b.PassengerID, &b.DriverID,
			&b.PickupWaypointID, &b.DropoffWaypointID, &b.SeatsBooked,
			&b.PricePerSeat, &b.Subtotal, &b.ServiceFee, &b.TotalAmount,
			&b.PaymentMethod, &b.Status,
			&b.PaymentCompletedAt, &b.ApprovedAt, &b.RejectedAt, &b.CancelledAt, &b.CompletedAt,
			&b.CancellerID, &b.CancellationReason,
			&b.NoShowType, &b.NoShowReportedBy, &b.NoShowReportedAt, &b.NoShowDescription,
			&b.PaymentReleasedAt, &b.CreatedAt, &b.UpdatedAt,
		); err != nil {
			r.logger.Error("GetCompletedBookingsPendingRelease scan failed", zap.Error(err))
			return nil, bookingErrors.ErrorDataRetrievalFailed
		}
		bookings = append(bookings, b)
	}

	return bookings, nil
}

// GetActivePassengerIDsForTrip retourne les IDs distincts des passagers avec une réservation active.
func (r *bookingReadRepositoryImpl) GetActivePassengerIDsForTrip(ctx context.Context, tripID string) ([]string, error) {
	query := `
		SELECT DISTINCT passenger_id
		FROM bookings
		WHERE trip_id = $1
		  AND status IN ('pendingApproval', 'approved', 'inProgress')`

	rows, err := r.pool.Query(ctx, query, tripID)
	if err != nil {
		r.logger.Error("GetActivePassengerIDsForTrip failed", zap.Error(err), zap.String("tripID", tripID))
		return nil, bookingErrors.ErrorDataRetrievalFailed
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			r.logger.Error("GetActivePassengerIDsForTrip scan failed", zap.Error(err))
			return nil, bookingErrors.ErrorDataRetrievalFailed
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// rawDriverBookingsQuery est la requête partagée par GetDriverPendingBookings et GetDriverTripBookingsRaw.
// Elle retourne les colonnes pour RawDriverBookingPreview avec LATERAL join sur les segments.
const rawDriverBookingsBaseSelect = `
	SELECT b.booking_id, b.booking_reference, b.trip_id, b.passenger_id, b.status::text,
	       b.seats_booked, b.total_amount,
	       COALESCE(s_pick.pickup_location_name, '') AS pickup_location_name,
	       COALESCE(s_drop.dropoff_location_name, '') AS dropoff_location_name,
	       COALESCE(s_pick.pickup_scheduled_at, b.created_at) AS departure_datetime,
	       b.payment_method::text, b.passenger_message, b.extra_minutes_detour,
	       b.created_at, b.payment_completed_at
	FROM bookings b
	LEFT JOIN LATERAL (
		SELECT pickup_location_name, pickup_scheduled_at FROM bookings_segments
		WHERE booking_id = b.booking_id ORDER BY created_at ASC LIMIT 1
	) s_pick ON true
	LEFT JOIN LATERAL (
		SELECT dropoff_location_name FROM bookings_segments
		WHERE booking_id = b.booking_id ORDER BY created_at DESC LIMIT 1
	) s_drop ON true`

// scanRawDriverBookingPreviews scanne les résultats en RawDriverBookingPreview.
func (r *bookingReadRepositoryImpl) scanRawDriverBookingPreviews(rows pgx.Rows) ([]*domain.RawDriverBookingPreview, error) {
	var results []*domain.RawDriverBookingPreview
	for rows.Next() {
		p := &domain.RawDriverBookingPreview{}
		var statusStr string
		if err := rows.Scan(
			&p.BookingID, &p.BookingReference, &p.TripID, &p.PassengerID, &statusStr,
			&p.SeatsBooked, &p.TotalAmount,
			&p.PickupLocationName, &p.DropoffLocationName,
			&p.DepartureDatetime,
			&p.PaymentMethod, &p.PassengerMessage, &p.ExtraMinutesDetour,
			&p.CreatedAt, &p.PaymentCompletedAt,
		); err != nil {
			r.logger.Error("scanRawDriverBookingPreviews scan failed", zap.Error(err))
			return nil, bookingErrors.ErrorDataRetrievalFailed
		}
		p.Status = domain.BookingStatus(statusStr)
		results = append(results, p)
	}
	return results, nil
}

// GetDriverPendingBookings retourne la liste paginée des réservations en attente d'un conducteur tous trajets confondus.
func (r *bookingReadRepositoryImpl) GetDriverPendingBookings(ctx context.Context, driverID string, pageIndex int) ([]*domain.RawDriverBookingPreview, error) {
	offset := pageIndex * pageSize
	query := rawDriverBookingsBaseSelect + `
	WHERE b.driver_id = $1 AND b.status = 'pendingApproval' AND b.deleted_at IS NULL
	ORDER BY b.created_at DESC
	LIMIT $2 OFFSET $3`

	rows, err := r.pool.Query(ctx, query, driverID, pageSize, offset)
	if err != nil {
		r.logger.Error("GetDriverPendingBookings failed", zap.Error(err), zap.String("driverID", driverID))
		return nil, bookingErrors.ErrorDataRetrievalFailed
	}
	defer rows.Close()

	return r.scanRawDriverBookingPreviews(rows)
}

// GetDriverTripBookingsRaw retourne les réservations enrichissables d'un trajet du conducteur.
func (r *bookingReadRepositoryImpl) GetDriverTripBookingsRaw(ctx context.Context, driverID, tripID string, pageIndex int) ([]*domain.RawDriverBookingPreview, error) {
	offset := pageIndex * pageSize
	query := rawDriverBookingsBaseSelect + `
	WHERE b.driver_id = $1 AND b.trip_id = $2 AND b.deleted_at IS NULL
	ORDER BY b.created_at DESC
	LIMIT $3 OFFSET $4`

	rows, err := r.pool.Query(ctx, query, driverID, tripID, pageSize, offset)
	if err != nil {
		r.logger.Error("GetDriverTripBookingsRaw failed", zap.Error(err), zap.String("driverID", driverID), zap.String("tripID", tripID))
		return nil, bookingErrors.ErrorDataRetrievalFailed
	}
	defer rows.Close()

	return r.scanRawDriverBookingPreviews(rows)
}

// GetDriverTripBookingCounts retourne les compteurs de réservations par statut pour un trajet.
func (r *bookingReadRepositoryImpl) GetDriverTripBookingCounts(ctx context.Context, driverID, tripID string) (*domain.BookingCounts, error) {
	query := `
		SELECT
			COUNT(*) FILTER (WHERE status = 'pendingApproval') AS pending,
			COUNT(*) FILTER (WHERE status = 'approved')        AS approved,
			COUNT(*) FILTER (WHERE status = 'rejected')        AS rejected,
			COUNT(*) FILTER (WHERE status = 'cancelled')       AS cancelled
		FROM bookings
		WHERE driver_id = $1 AND trip_id = $2 AND deleted_at IS NULL`

	counts := &domain.BookingCounts{}
	if err := r.pool.QueryRow(ctx, query, driverID, tripID).Scan(
		&counts.Pending, &counts.Approved, &counts.Rejected, &counts.Cancelled,
	); err != nil {
		r.logger.Error("GetDriverTripBookingCounts failed", zap.Error(err), zap.String("tripID", tripID))
		return nil, bookingErrors.ErrorDataRetrievalFailed
	}

	return counts, nil
}

// GetActivePassengerSummariesForTrip retourne les données brutes des passagers actifs d'un trajet.
func (r *bookingReadRepositoryImpl) GetActivePassengerSummariesForTrip(ctx context.Context, tripID string) ([]*domain.RawPassengerSummary, error) {
	query := `
		SELECT passenger_id, booking_id, seats_booked, payment_method::text, payment_completed_at
		FROM bookings
		WHERE trip_id = $1
		  AND status IN ('pendingApproval', 'approved', 'inProgress')
		  AND deleted_at IS NULL`

	rows, err := r.pool.Query(ctx, query, tripID)
	if err != nil {
		r.logger.Error("GetActivePassengerSummariesForTrip failed", zap.Error(err), zap.String("tripID", tripID))
		return nil, bookingErrors.ErrorDataRetrievalFailed
	}
	defer rows.Close()

	var results []*domain.RawPassengerSummary
	for rows.Next() {
		s := &domain.RawPassengerSummary{}
		if err := rows.Scan(&s.PassengerID, &s.BookingID, &s.SeatsBooked, &s.PaymentMethod, &s.PaymentCompletedAt); err != nil {
			r.logger.Error("GetActivePassengerSummariesForTrip scan failed", zap.Error(err))
			return nil, bookingErrors.ErrorDataRetrievalFailed
		}
		results = append(results, s)
	}
	return results, nil
}

// GetPassengerCompletedBookingsCount retourne le nombre de réservations complétées d'un passager.
func (r *bookingReadRepositoryImpl) GetPassengerCompletedBookingsCount(ctx context.Context, passengerID string) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM bookings
		WHERE passenger_id = $1 AND status = 'completed' AND deleted_at IS NULL`

	var count int
	if err := r.pool.QueryRow(ctx, query, passengerID).Scan(&count); err != nil {
		r.logger.Error("GetPassengerCompletedBookingsCount failed", zap.Error(err), zap.String("passengerID", passengerID))
		return 0, bookingErrors.ErrorDataRetrievalFailed
	}
	return count, nil
}

// HasActiveBookingAsPassenger vérifie si un passager a une réservation active (tous trajets confondus).
func (r *bookingReadRepositoryImpl) HasActiveBookingAsPassenger(ctx context.Context, passengerID string) (bool, error) {
	query := `
		SELECT EXISTS (
			SELECT 1 FROM bookings
			WHERE passenger_id = $1
			AND status IN ('created', 'paymentPending', 'pendingApproval', 'approved', 'inProgress')
			AND deleted_at IS NULL
		)`

	var exists bool
	if err := r.pool.QueryRow(ctx, query, passengerID).Scan(&exists); err != nil {
		r.logger.Error("HasActiveBookingAsPassenger failed", zap.Error(err), zap.String("passengerID", passengerID))
		return false, bookingErrors.ErrorDataRetrievalFailed
	}
	return exists, nil
}

// HasActiveBookingAsDriver vérifie si un chauffeur a des réservations actives sur ses trajets.
func (r *bookingReadRepositoryImpl) HasActiveBookingAsDriver(ctx context.Context, driverID string) (bool, error) {
	query := `
		SELECT EXISTS (
			SELECT 1 FROM bookings
			WHERE driver_id = $1
			AND status IN ('pendingApproval', 'approved', 'inProgress')
			AND deleted_at IS NULL
		)`

	var exists bool
	if err := r.pool.QueryRow(ctx, query, driverID).Scan(&exists); err != nil {
		r.logger.Error("HasActiveBookingAsDriver failed", zap.Error(err), zap.String("driverID", driverID))
		return false, bookingErrors.ErrorDataRetrievalFailed
	}
	return exists, nil
}

// GetPassengerBookingIDs retourne tous les IDs de réservation d'un passager.
func (r *bookingReadRepositoryImpl) GetPassengerBookingIDs(ctx context.Context, passengerID string) ([]string, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT booking_id FROM bookings WHERE passenger_id = $1 AND deleted_at IS NULL`,
		passengerID,
	)
	if err != nil {
		r.logger.Error("GetPassengerBookingIDs failed", zap.Error(err), zap.String("passengerID", passengerID))
		return nil, bookingErrors.ErrorDataRetrievalFailed
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			r.logger.Error("GetPassengerBookingIDs scan failed", zap.Error(err))
			return nil, bookingErrors.ErrorDataRetrievalFailed
		}
		ids = append(ids, id)
	}
	return ids, nil
}
