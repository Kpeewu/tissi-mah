package implementations

import (
	"context"
	"errors"
	"fmt"

	"github.com/Kpeewu/tissi-mah/services/booking-service/internal/domain"
	i "github.com/Kpeewu/tissi-mah/services/booking-service/internal/repository/interfaces"
	bookingErrors "github.com/Kpeewu/tissi-mah/services/booking-service/pkg/errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type bookingWriteRepositoryImpl struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

func NewBookingWriteRepository(pool *pgxpool.Pool, logger *zap.Logger) i.BookingRepositoryWrite {
	return &bookingWriteRepositoryImpl{pool: pool, logger: logger}
}

// Create insère une réservation avec ses segments et une entrée d'historique dans une transaction unique.
func (r *bookingWriteRepositoryImpl) Create(ctx context.Context, booking *domain.Booking, segments []*domain.Segment, history *domain.StatusHistoryEntry) error {
	r.logger.Debug("creating booking",
		zap.String("bookingID", booking.BookingID),
		zap.String("tripID", booking.TripID),
		zap.String("passengerID", booking.PassengerID),
	)

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		r.logger.Error("begin transaction failed", zap.Error(err))
		return bookingErrors.ErrorInternalServer
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Insérer la réservation
	if err := r.insertBooking(ctx, tx, booking); err != nil {
		return err
	}

	// Insérer les segments
	for _, seg := range segments {
		if err := r.insertSegment(ctx, tx, seg); err != nil {
			return err
		}
	}

	// Insérer l'entrée d'historique
	if err := r.insertHistory(ctx, tx, history); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		r.logger.Error("commit transaction failed", zap.Error(err), zap.String("bookingID", booking.BookingID))
		return bookingErrors.ErrorInternalServer
	}

	r.logger.Info("booking created", zap.String("bookingID", booking.BookingID))
	return nil
}

func (r *bookingWriteRepositoryImpl) insertBooking(ctx context.Context, tx pgx.Tx, b *domain.Booking) error {
	query := `
		INSERT INTO bookings (
			booking_id, booking_reference, trip_id, passenger_id, driver_id,
			pickup_waypoint_id, dropoff_waypoint_id,
			pickup_sequencer_order, dropoff_sequencer_order,
			seats_booked,
			price_per_seat, subtotal, service_fee, total_amount,
			payment_method, status,
			payment_completed_at, approved_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7,
			$8, $9,
			$10,
			$11, $12, $13, $14,
			$15::booking_payment_method, $16::booking_status,
			$17, $18
		)`

	_, err := tx.Exec(ctx, query,
		b.BookingID, b.BookingReference, b.TripID, b.PassengerID, b.DriverID,
		b.PickupWaypointID, b.DropoffWaypointID,
		b.PickupSequencerOrder, b.DropoffSequencerOrder,
		b.SeatsBooked,
		b.PricePerSeat, b.Subtotal, b.ServiceFee, b.TotalAmount,
		string(b.PaymentMethod), string(b.Status),
		b.PaymentCompletedAt, b.ApprovedAt,
	)
	if err != nil {
		r.logger.Error("insertBooking failed", zap.Error(err), zap.String("bookingID", b.BookingID))
		return fmt.Errorf("%w: %s", bookingErrors.ErrorInternalServer, err.Error())
	}

	return nil
}

func (r *bookingWriteRepositoryImpl) insertSegment(ctx context.Context, tx pgx.Tx, s *domain.Segment) error {
	query := `
		INSERT INTO bookings_segments (
			segment_id, booking_id, pickup_waypoint_id, dropoff_waypoint_id,
			pickup_location_name, pickup_city, pickup_lat, pickup_lng,
			pickup_scheduled_at,
			dropoff_location_name, dropoff_city, dropoff_lat, dropoff_lng,
			dropoff_scheduled_at,
			segment_distance_meters, segment_duration_minutes, segment_price
		) VALUES (
			$1, $2, $3, $4,
			$5, $6, $7, $8,
			$9,
			$10, $11, $12, $13,
			$14,
			$15, $16, $17
		)`

	_, err := tx.Exec(ctx, query,
		s.SegmentID, s.BookingID, s.PickupWaypointID, s.DropoffWaypointID,
		s.PickupLocationName, s.PickupCity, s.PickupLat, s.PickupLng,
		s.PickupScheduledAt,
		s.DropoffLocationName, s.DropoffCity, s.DropoffLat, s.DropoffLng,
		s.DropoffScheduledAt,
		s.SegmentDistanceMeters, s.SegmentDurationMinutes, s.SegmentPrice,
	)
	if err != nil {
		r.logger.Error("insertSegment failed", zap.Error(err), zap.String("segmentID", s.SegmentID))
		return fmt.Errorf("%w: %s", bookingErrors.ErrorInternalServer, err.Error())
	}

	return nil
}

func (r *bookingWriteRepositoryImpl) insertHistory(ctx context.Context, tx pgx.Tx, h *domain.StatusHistoryEntry) error {
	query := `
		INSERT INTO bookings_status_history (
			history_id, booking_id, previous_status, new_status,
			changed_by, changed_by_type, change_reason, metadata
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	_, err := tx.Exec(ctx, query,
		h.HistoryID, h.BookingID, h.PreviousStatus, h.NewStatus,
		h.ChangedBy, h.ChangedByType, h.ChangeReason, h.Metadata,
	)
	if err != nil {
		r.logger.Error("insertHistory failed", zap.Error(err), zap.String("historyID", h.HistoryID))
		return fmt.Errorf("%w: %s", bookingErrors.ErrorInternalServer, err.Error())
	}

	return nil
}

// Approve approuve une réservation pendingApproval.
func (r *bookingWriteRepositoryImpl) Approve(ctx context.Context, bookingID, driverID string) error {
	r.logger.Debug("approving booking", zap.String("bookingID", bookingID))

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return bookingErrors.ErrorInternalServer
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Vérifier que le booking existe et est pendingApproval
	var currentStatus string
	var currentDriverID string
	err = tx.QueryRow(ctx, `
		SELECT status, driver_id FROM bookings WHERE booking_id = $1 FOR UPDATE`, bookingID,
	).Scan(&currentStatus, &currentDriverID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return bookingErrors.ErrorBookingNotFound
		}
		return bookingErrors.ErrorDataRetrievalFailed
	}

	if currentDriverID != driverID {
		return bookingErrors.ErrorUnauthorized
	}
	if currentStatus != string(domain.BookingStatusPendingApproval) {
		return bookingErrors.ErrorBookingNotPending
	}

	// Mettre à jour le statut
	_, err = tx.Exec(ctx, `
		UPDATE bookings SET status = 'approved', approved_at = NOW() WHERE booking_id = $1`, bookingID)
	if err != nil {
		r.logger.Error("approve update failed", zap.Error(err))
		return bookingErrors.ErrorInternalServer
	}

	// Historique
	if err := r.insertHistoryInTx(ctx, tx, bookingID, currentStatus, string(domain.BookingStatusApproved), driverID, "driver"); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// Reject rejette une réservation pendingApproval.
func (r *bookingWriteRepositoryImpl) Reject(ctx context.Context, bookingID, driverID, reason string) error {
	r.logger.Debug("rejecting booking", zap.String("bookingID", bookingID))

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return bookingErrors.ErrorInternalServer
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var currentStatus string
	var currentDriverID string
	err = tx.QueryRow(ctx, `
		SELECT status, driver_id FROM bookings WHERE booking_id = $1 FOR UPDATE`, bookingID,
	).Scan(&currentStatus, &currentDriverID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return bookingErrors.ErrorBookingNotFound
		}
		return bookingErrors.ErrorDataRetrievalFailed
	}

	if currentDriverID != driverID {
		return bookingErrors.ErrorUnauthorized
	}
	if currentStatus != string(domain.BookingStatusPendingApproval) {
		return bookingErrors.ErrorBookingNotPending
	}

	_, err = tx.Exec(ctx, `
		UPDATE bookings SET status = 'rejected', rejected_at = NOW(), cancellation_reason = $2 WHERE booking_id = $1`,
		bookingID, reason)
	if err != nil {
		r.logger.Error("reject update failed", zap.Error(err))
		return bookingErrors.ErrorInternalServer
	}

	if err := r.insertHistoryInTx(ctx, tx, bookingID, currentStatus, string(domain.BookingStatusRejected), driverID, "driver"); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// Cancel annule une réservation.
func (r *bookingWriteRepositoryImpl) Cancel(ctx context.Context, bookingID, cancellerID, reason string) error {
	r.logger.Debug("cancelling booking", zap.String("bookingID", bookingID))

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return bookingErrors.ErrorInternalServer
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var currentStatus string
	var passengerID, driverID string
	err = tx.QueryRow(ctx, `
		SELECT status, passenger_id, driver_id FROM bookings WHERE booking_id = $1 FOR UPDATE`, bookingID,
	).Scan(&currentStatus, &passengerID, &driverID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return bookingErrors.ErrorBookingNotFound
		}
		return bookingErrors.ErrorDataRetrievalFailed
	}

	// Vérifier que l'utilisateur est le passager ou le conducteur
	if cancellerID != passengerID && cancellerID != driverID {
		return bookingErrors.ErrorUnauthorized
	}

	// Vérifier le statut — on ne peut annuler que certains statuts
	cancellableStatuses := map[string]bool{
		string(domain.BookingStatusCreated):         true,
		string(domain.BookingStatusPaymentPending):  true,
		string(domain.BookingStatusPendingApproval): true,
		string(domain.BookingStatusApproved):        true,
	}
	if !cancellableStatuses[currentStatus] {
		return bookingErrors.ErrorBookingAlreadyCancelled
	}

	// Déterminer le type de l'annuleur
	changedByType := "passenger"
	if cancellerID == driverID {
		changedByType = "driver"
	}

	_, err = tx.Exec(ctx, `
		UPDATE bookings SET status = 'cancelled', cancelled_at = NOW(),
		       canceller_id = $2, cancellation_reason = $3
		WHERE booking_id = $1`,
		bookingID, cancellerID, reason)
	if err != nil {
		r.logger.Error("cancel update failed", zap.Error(err))
		return bookingErrors.ErrorInternalServer
	}

	if err := r.insertHistoryInTx(ctx, tx, bookingID, currentStatus, string(domain.BookingStatusCancelled), cancellerID, changedByType); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// StartBookingsForWaypoint démarre les réservations approved d'un waypoint (pickup).
func (r *bookingWriteRepositoryImpl) StartBookingsForWaypoint(ctx context.Context, tripID, waypointID string) (int, error) {
	r.logger.Debug("starting bookings for waypoint",
		zap.String("tripID", tripID), zap.String("waypointID", waypointID))

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, bookingErrors.ErrorInternalServer
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Récupérer les bookings approved pour ce waypoint de pickup
	rows, err := tx.Query(ctx, `
		SELECT booking_id FROM bookings
		WHERE trip_id = $1 AND pickup_waypoint_id = $2 AND status = 'approved'
		FOR UPDATE`, tripID, waypointID)
	if err != nil {
		r.logger.Error("start bookings query failed", zap.Error(err))
		return 0, bookingErrors.ErrorInternalServer
	}

	var bookingIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, bookingErrors.ErrorInternalServer
		}
		bookingIDs = append(bookingIDs, id)
	}
	rows.Close()

	if len(bookingIDs) == 0 {
		return 0, tx.Commit(ctx)
	}

	// Mettre à jour en batch
	for _, bid := range bookingIDs {
		_, err = tx.Exec(ctx, `
			UPDATE bookings SET status = 'inProgress' WHERE booking_id = $1`, bid)
		if err != nil {
			r.logger.Error("start booking update failed", zap.Error(err), zap.String("bookingID", bid))
			return 0, bookingErrors.ErrorInternalServer
		}

		if err := r.insertHistoryInTx(ctx, tx, bid, string(domain.BookingStatusApproved), string(domain.BookingStatusInProgress), "system", "system"); err != nil {
			return 0, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, bookingErrors.ErrorInternalServer
	}

	r.logger.Info("bookings started for waypoint",
		zap.String("tripID", tripID), zap.String("waypointID", waypointID), zap.Int("count", len(bookingIDs)))
	return len(bookingIDs), nil
}

// CompleteBookingsForWaypoint complète les réservations inProgress d'un waypoint (dropoff).
func (r *bookingWriteRepositoryImpl) CompleteBookingsForWaypoint(ctx context.Context, tripID, waypointID string) (int, error) {
	r.logger.Debug("completing bookings for waypoint",
		zap.String("tripID", tripID), zap.String("waypointID", waypointID))

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, bookingErrors.ErrorInternalServer
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Récupérer les bookings inProgress pour ce waypoint de dropoff
	rows, err := tx.Query(ctx, `
		SELECT booking_id FROM bookings
		WHERE trip_id = $1 AND dropoff_waypoint_id = $2 AND status = 'inProgress'
		FOR UPDATE`, tripID, waypointID)
	if err != nil {
		r.logger.Error("complete bookings query failed", zap.Error(err))
		return 0, bookingErrors.ErrorInternalServer
	}

	var bookingIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, bookingErrors.ErrorInternalServer
		}
		bookingIDs = append(bookingIDs, id)
	}
	rows.Close()

	if len(bookingIDs) == 0 {
		return 0, tx.Commit(ctx)
	}

	for _, bid := range bookingIDs {
		_, err = tx.Exec(ctx, `
			UPDATE bookings SET status = 'completed', completed_at = NOW() WHERE booking_id = $1`, bid)
		if err != nil {
			r.logger.Error("complete booking update failed", zap.Error(err), zap.String("bookingID", bid))
			return 0, bookingErrors.ErrorInternalServer
		}

		if err := r.insertHistoryInTx(ctx, tx, bid, string(domain.BookingStatusInProgress), string(domain.BookingStatusCompleted), "system", "system"); err != nil {
			return 0, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, bookingErrors.ErrorInternalServer
	}

	r.logger.Info("bookings completed for waypoint",
		zap.String("tripID", tripID), zap.String("waypointID", waypointID), zap.Int("count", len(bookingIDs)))
	return len(bookingIDs), nil
}

// ReportNoShow signale l'absence d'un passager ou d'un conducteur.
func (r *bookingWriteRepositoryImpl) ReportNoShow(ctx context.Context, bookingID, reporterID, noShowType, description string) error {
	r.logger.Debug("reporting no-show", zap.String("bookingID", bookingID), zap.String("type", noShowType))

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return bookingErrors.ErrorInternalServer
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var currentStatus string
	err = tx.QueryRow(ctx, `
		SELECT status FROM bookings WHERE booking_id = $1 FOR UPDATE`, bookingID,
	).Scan(&currentStatus)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return bookingErrors.ErrorBookingNotFound
		}
		return bookingErrors.ErrorDataRetrievalFailed
	}

	// On ne peut signaler un no-show que pour un booking approved ou inProgress
	if currentStatus != string(domain.BookingStatusApproved) && currentStatus != string(domain.BookingStatusInProgress) {
		return bookingErrors.ErrorInvalidStatusTransition
	}

	_, err = tx.Exec(ctx, `
		UPDATE bookings SET status = 'noShow',
		       no_show_type = $2, no_show_reported_by = $3,
		       no_show_reported_at = NOW(), no_show_description = $4
		WHERE booking_id = $1`,
		bookingID, noShowType, reporterID, description)
	if err != nil {
		r.logger.Error("report no-show update failed", zap.Error(err))
		return bookingErrors.ErrorInternalServer
	}

	if err := r.insertHistoryInTx(ctx, tx, bookingID, currentStatus, string(domain.BookingStatusNoShow), reporterID, noShowType); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// ConfirmPayment confirme le paiement d'une réservation.
func (r *bookingWriteRepositoryImpl) ConfirmPayment(ctx context.Context, bookingID, transactionID string) error {
	r.logger.Debug("confirming payment", zap.String("bookingID", bookingID))

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return bookingErrors.ErrorInternalServer
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var currentStatus string
	var tripID string
	err = tx.QueryRow(ctx, `
		SELECT status, trip_id FROM bookings WHERE booking_id = $1 FOR UPDATE`, bookingID,
	).Scan(&currentStatus, &tripID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return bookingErrors.ErrorBookingNotFound
		}
		return bookingErrors.ErrorDataRetrievalFailed
	}

	if currentStatus != string(domain.BookingStatusPaymentPending) {
		return bookingErrors.ErrorPaymentAlreadyConfirmed
	}

	// Paiement confirmé → passer à pendingApproval
	newStatus := string(domain.BookingStatusPendingApproval)

	_, err = tx.Exec(ctx, `
		UPDATE bookings SET status = $2::booking_status, payment_completed_at = NOW()
		WHERE booking_id = $1`, bookingID, newStatus)
	if err != nil {
		r.logger.Error("confirm payment update failed", zap.Error(err))
		return bookingErrors.ErrorInternalServer
	}

	confirmReason := "Paiement confirmé"
	if err := r.insertHistory(ctx, tx, &domain.StatusHistoryEntry{
		HistoryID:      uuid.New().String(),
		BookingID:      bookingID,
		PreviousStatus: currentStatus,
		NewStatus:      newStatus,
		ChangedBy:      "system",
		ChangedByType:  "system",
		ChangeReason:   &confirmReason,
	}); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// insertHistoryInTx est un helper pour insérer une entrée d'historique dans une transaction existante.
func (r *bookingWriteRepositoryImpl) insertHistoryInTx(ctx context.Context, tx pgx.Tx, bookingID, previousStatus, newStatus, changedBy, changedByType string) error {
	query := `
		INSERT INTO bookings_status_history (
			history_id, booking_id, previous_status, new_status,
			changed_by, changed_by_type
		) VALUES (gen_random_uuid(), $1, $2, $3, $4, $5)`

	_, err := tx.Exec(ctx, query, bookingID, previousStatus, newStatus, changedBy, changedByType)
	if err != nil {
		r.logger.Error("insertHistoryInTx failed", zap.Error(err), zap.String("bookingID", bookingID))
		return fmt.Errorf("%w: %s", bookingErrors.ErrorInternalServer, err.Error())
	}

	return nil
}

// FailPayment marque le paiement d'une réservation comme échoué.
func (r *bookingWriteRepositoryImpl) FailPayment(ctx context.Context, bookingID, reason string) error {
	r.logger.Debug("failing payment", zap.String("bookingID", bookingID))

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return bookingErrors.ErrorInternalServer
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var currentStatus string
	err = tx.QueryRow(ctx, `
		SELECT status FROM bookings WHERE booking_id = $1 FOR UPDATE`, bookingID,
	).Scan(&currentStatus)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return bookingErrors.ErrorBookingNotFound
		}
		return bookingErrors.ErrorDataRetrievalFailed
	}

	if currentStatus != string(domain.BookingStatusPaymentPending) {
		return bookingErrors.ErrorInvalidStatusTransition
	}

	newStatus := string(domain.BookingStatusPaymentFailed)

	_, err = tx.Exec(ctx, `
		UPDATE bookings SET status = $2::booking_status, cancellation_reason = $3
		WHERE booking_id = $1`, bookingID, newStatus, reason)
	if err != nil {
		r.logger.Error("fail payment update failed", zap.Error(err))
		return bookingErrors.ErrorInternalServer
	}

	if err := r.insertHistory(ctx, tx, &domain.StatusHistoryEntry{
		HistoryID:      uuid.New().String(),
		BookingID:      bookingID,
		PreviousStatus: currentStatus,
		NewStatus:      newStatus,
		ChangedBy:      "system",
		ChangedByType:  "system",
		ChangeReason:   &reason,
	}); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// CancelBookingsForWaypoint annule les réservations actives d'un waypoint supprimé.
func (r *bookingWriteRepositoryImpl) CancelBookingsForWaypoint(ctx context.Context, tripID, waypointID string) ([]*domain.Booking, error) {
	r.logger.Debug("cancelling bookings for waypoint",
		zap.String("tripID", tripID), zap.String("waypointID", waypointID))

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, bookingErrors.ErrorInternalServer
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Récupérer les bookings actifs ayant ce waypoint comme pickup ou dropoff
	rows, err := tx.Query(ctx, `
		SELECT booking_id, trip_id, passenger_id, driver_id,
		       pickup_waypoint_id, dropoff_waypoint_id,
		       pickup_sequencer_order, dropoff_sequencer_order,
		       seats_booked, total_amount, payment_method, status
		FROM bookings
		WHERE trip_id = $1
		  AND (pickup_waypoint_id = $2 OR dropoff_waypoint_id = $2)
		  AND status IN ('paymentPending', 'pendingApproval', 'approved')
		FOR UPDATE`, tripID, waypointID)
	if err != nil {
		r.logger.Error("cancel bookings for waypoint query failed", zap.Error(err))
		return nil, bookingErrors.ErrorInternalServer
	}

	var bookings []*domain.Booking
	for rows.Next() {
		b := &domain.Booking{}
		if err := rows.Scan(
			&b.BookingID, &b.TripID, &b.PassengerID, &b.DriverID,
			&b.PickupWaypointID, &b.DropoffWaypointID,
			&b.PickupSequencerOrder, &b.DropoffSequencerOrder,
			&b.SeatsBooked, &b.TotalAmount, &b.PaymentMethod, &b.Status,
		); err != nil {
			rows.Close()
			return nil, bookingErrors.ErrorInternalServer
		}
		bookings = append(bookings, b)
	}
	rows.Close()

	if len(bookings) == 0 {
		return nil, tx.Commit(ctx)
	}

	cancelReason := "Arrêt supprimé par le chauffeur"
	for _, b := range bookings {
		_, err = tx.Exec(ctx, `
			UPDATE bookings SET status = 'cancelled', cancelled_at = NOW(),
			       canceller_id = 'system', cancellation_reason = $2
			WHERE booking_id = $1`, b.BookingID, cancelReason)
		if err != nil {
			r.logger.Error("cancel booking for waypoint update failed", zap.Error(err), zap.String("bookingID", b.BookingID))
			return nil, bookingErrors.ErrorInternalServer
		}

		if err := r.insertHistory(ctx, tx, &domain.StatusHistoryEntry{
			HistoryID:      uuid.New().String(),
			BookingID:      b.BookingID,
			PreviousStatus: string(b.Status),
			NewStatus:      string(domain.BookingStatusCancelled),
			ChangedBy:      "system",
			ChangedByType:  "system",
			ChangeReason:   &cancelReason,
		}); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, bookingErrors.ErrorInternalServer
	}

	r.logger.Info("bookings cancelled for waypoint",
		zap.String("tripID", tripID), zap.String("waypointID", waypointID), zap.Int("count", len(bookings)))
	return bookings, nil
}

// MarkPaymentReleased marque le paiement d'un booking comme libéré.
func (r *bookingWriteRepositoryImpl) MarkPaymentReleased(ctx context.Context, bookingID string) error {
	query := `UPDATE bookings SET payment_released_at = NOW() WHERE booking_id = $1`

	tag, err := r.pool.Exec(ctx, query, bookingID)
	if err != nil {
		r.logger.Error("MarkPaymentReleased failed", zap.Error(err), zap.String("bookingID", bookingID))
		return bookingErrors.ErrorInternalServer
	}
	if tag.RowsAffected() == 0 {
		return bookingErrors.ErrorBookingNotFound
	}

	return nil
}
