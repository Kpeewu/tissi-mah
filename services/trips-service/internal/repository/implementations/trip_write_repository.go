package implementations

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Kpeewu/tissi-mah/services/trips-service/internal/domain"
	i "github.com/Kpeewu/tissi-mah/services/trips-service/internal/repository/interfaces"
	tripErrors "github.com/Kpeewu/tissi-mah/services/trips-service/pkg/errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type tripWriteRepositoryImpl struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

func NewTripWriteRepository(pool *pgxpool.Pool, logger *zap.Logger) i.TripRepositoryWrite {
	return &tripWriteRepositoryImpl{pool: pool, logger: logger}
}

// Create insère un trajet et ses waypoints dans une transaction unique.
func (r *tripWriteRepositoryImpl) Create(ctx context.Context, trip *domain.Trip, waypoints []*domain.Waypoint) (string, error) {
	r.logger.Debug("creating trip",
		zap.String("tripID", trip.TripID),
		zap.String("driverID", trip.DriverID),
		zap.Int("waypointCount", len(waypoints)),
	)

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		r.logger.Error("begin transaction failed", zap.Error(err))
		return "", tripErrors.ErrorInternalServer
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	tripID, err := r.insertTrip(ctx, tx, trip)
	if err != nil {
		return "", err
	}

	if err := r.insertWaypoints(ctx, tx, tripID, waypoints); err != nil {
		return "", err
	}

	if err := tx.Commit(ctx); err != nil {
		r.logger.Error("commit transaction failed", zap.Error(err), zap.String("tripID", tripID))
		return "", tripErrors.ErrorInternalServer
	}

	r.logger.Info("trip created", zap.String("tripID", tripID))
	return tripID, nil
}

// insertTrip insère un trajet dans la transaction fournie.
// recurring_pattern_id peut être nil pour les trajets ponctuels.
func (r *tripWriteRepositoryImpl) insertTrip(ctx context.Context, tx pgx.Tx, trip *domain.Trip) (string, error) {
	query := `
		INSERT INTO trips (
			trip_id, driver_id, vehicle_id,
			recurring_pattern_id,
			departure_datetime, estimated_arrival_datetime,
			estimated_duration_minutes, estimated_distance_meters,
			total_seats, available_seats, price_per_seat,
			payment_methods_accepted,
			allow_luggages, allow_pets, allow_food, allow_smoking,
			status, auto_approve_enabled, description
		) VALUES (
			$1, $2, $3,
			$4,
			$5, $6,
			$7, $8,
			$9, $10, $11,
			$12::payment_method[],
			$13, $14, $15, $16,
			$17::trip_status, $18, $19
		)
		RETURNING trip_id`

	var tripID string
	err := tx.QueryRow(ctx, query,
		trip.TripID, trip.DriverID, trip.VehicleID,
		trip.RecurringPatternID,
		trip.DepartureDatetime, trip.EstimatedArrivalDatetime,
		trip.EstimatedDurationMinutes, trip.EstimatedDistanceMeters,
		trip.TotalSeats, trip.AvailableSeats, trip.PricePerSeat,
		trip.PaymentMethodsAccepted,
		trip.AllowLuggages, trip.AllowPets, trip.AllowFood, trip.AllowSmoking,
		string(trip.Status), trip.AutoApproveEnabled, trip.Description,
	).Scan(&tripID)

	if err != nil {
		r.logger.Error("insert trip failed", zap.Error(err), zap.String("tripID", trip.TripID))
		if errors.Is(err, pgx.ErrNoRows) {
			return "", tripErrors.ErrorInternalServer
		}
		return "", fmt.Errorf("%w: %s", tripErrors.ErrorInternalServer, err.Error())
	}

	return tripID, nil
}

// insertWaypoints insère les waypoints d'un trajet dans la transaction fournie.
func (r *tripWriteRepositoryImpl) insertWaypoints(ctx context.Context, tx pgx.Tx, tripID string, waypoints []*domain.Waypoint) error {
	query := `
		INSERT INTO trips_waypoints (
			waypoint_id, trip_id, sequencer_order,
			waypoint_type, location_name,
			location_lng, location_lat,
			position,
			city, country,
			scheduled_pickup_datetime,
			minutes_from_departure, price_from_previous
		) VALUES (
			$1, $2, $3,
			$4::waypoint_type, $5,
			$6, $7,
			ST_SetSRID(ST_MakePoint($13::float8, $14::float8), 4326)::geography,
			$8, $9,
			$10,
			$11, $12
		)`

	for _, wp := range waypoints {
		_, err := tx.Exec(ctx, query,
			wp.WaypointID, tripID, wp.SequencerOrder,
			string(wp.WaypointType), wp.LocationName,
			wp.LocationLat, wp.LocationLng,
			wp.City, wp.Country,
			wp.ScheduledPickupDatetime,
			wp.MinutesFromDeparture, wp.PriceFromPrevious,
			wp.LocationLng, wp.LocationLat, // $13=lng, $14=lat pour ST_MakePoint
		)
		if err != nil {
			r.logger.Error("insert waypoint failed",
				zap.Error(err),
				zap.String("waypointID", wp.WaypointID),
				zap.String("tripID", tripID),
			)
			return tripErrors.ErrorInternalServer
		}
	}

	return nil
}

// UpdateDepartureDatetime met à jour la date/heure de départ d'un trajet planifié.
// Si aucune ligne n'est mise à jour, une SELECT détermine la raison exacte.
func (r *tripWriteRepositoryImpl) UpdateDepartureDatetime(ctx context.Context, tripID, driverID string, newDatetime time.Time) error {
	r.logger.Debug("updating trip departure datetime",
		zap.String("tripID", tripID),
		zap.String("driverID", driverID),
		zap.Time("newDatetime", newDatetime),
	)

	query := `
		UPDATE trips
		SET departure_datetime = $3, updated_at = NOW()
		WHERE trip_id = $1 AND driver_id = $2 AND status = 'scheduled'`

	tag, err := r.pool.Exec(ctx, query, tripID, driverID, newDatetime)
	if err != nil {
		r.logger.Error("update departure datetime failed", zap.Error(err), zap.String("tripID", tripID))
		return fmt.Errorf("%w: %s", tripErrors.ErrorInternalServer, err.Error())
	}

	if tag.RowsAffected() == 1 {
		r.logger.Info("trip departure datetime updated", zap.String("tripID", tripID))
		return nil
	}

	// Aucune ligne mise à jour — déterminer pourquoi
	var foundDriverID string
	var foundStatus string
	selectQuery := `SELECT driver_id, status FROM trips WHERE trip_id = $1`
	err = r.pool.QueryRow(ctx, selectQuery, tripID).Scan(&foundDriverID, &foundStatus)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return tripErrors.ErrorTripNotFound
		}
		r.logger.Error("select trip for diagnosis failed", zap.Error(err), zap.String("tripID", tripID))
		return tripErrors.ErrorInternalServer
	}

	if foundDriverID != driverID {
		return tripErrors.ErrorUnauthorized
	}
	return tripErrors.ErrorTripNotScheduled
}

// UpdateVehicle met à jour le véhicule associé à un trajet planifié.
// Si aucune ligne n'est mise à jour, une SELECT détermine la raison exacte.
func (r *tripWriteRepositoryImpl) UpdateVehicle(ctx context.Context, tripID, driverID, vehicleID string) error {
	r.logger.Debug("updating trip vehicle",
		zap.String("tripID", tripID),
		zap.String("driverID", driverID),
		zap.String("vehicleID", vehicleID),
	)

	query := `
		UPDATE trips
		SET vehicle_id = $3, updated_at = NOW()
		WHERE trip_id = $1 AND driver_id = $2 AND status = 'scheduled'`

	tag, err := r.pool.Exec(ctx, query, tripID, driverID, vehicleID)
	if err != nil {
		r.logger.Error("update vehicle failed", zap.Error(err), zap.String("tripID", tripID))
		return fmt.Errorf("%w: %s", tripErrors.ErrorInternalServer, err.Error())
	}

	if tag.RowsAffected() == 1 {
		r.logger.Info("trip vehicle updated", zap.String("tripID", tripID))
		return nil
	}

	return r.diagnoseTripUpdateFailure(ctx, tripID, driverID)
}

// UpdateAllowances met à jour les autorisations d'un trajet planifié.
// La contrainte 24h est vérifiée directement dans le WHERE pour atomicité.
func (r *tripWriteRepositoryImpl) UpdateAllowances(ctx context.Context, tripID, driverID string, allowPets, allowFood, allowSmoking, allowLuggages bool) error {
	r.logger.Debug("updating trip allowances",
		zap.String("tripID", tripID),
		zap.String("driverID", driverID),
	)

	query := `
		UPDATE trips
		SET allow_pets = $3, allow_food = $4, allow_smoking = $5, allow_luggages = $6, updated_at = NOW()
		WHERE trip_id = $1
		  AND driver_id = $2
		  AND status = 'scheduled'
		  AND departure_datetime > NOW() + INTERVAL '24 hours'`

	tag, err := r.pool.Exec(ctx, query, tripID, driverID, allowPets, allowFood, allowSmoking, allowLuggages)
	if err != nil {
		r.logger.Error("update allowances failed", zap.Error(err), zap.String("tripID", tripID))
		return fmt.Errorf("%w: %s", tripErrors.ErrorInternalServer, err.Error())
	}

	if tag.RowsAffected() == 1 {
		r.logger.Info("trip allowances updated", zap.String("tripID", tripID))
		return nil
	}

	// Pour UpdateAllowances, on a besoin de departure_datetime pour distinguer
	// TripNotScheduled de TripDepartureTooSoon
	var foundDriverID string
	var foundStatus string
	var departureDatetime time.Time
	selectQuery := `SELECT driver_id, status, departure_datetime FROM trips WHERE trip_id = $1`
	err = r.pool.QueryRow(ctx, selectQuery, tripID).Scan(&foundDriverID, &foundStatus, &departureDatetime)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return tripErrors.ErrorTripNotFound
		}
		r.logger.Error("select trip for diagnosis failed", zap.Error(err), zap.String("tripID", tripID))
		return tripErrors.ErrorInternalServer
	}

	if foundDriverID != driverID {
		return tripErrors.ErrorUnauthorized
	}
	if foundStatus != string(domain.TripStatusScheduled) {
		return tripErrors.ErrorTripNotScheduled
	}
	return tripErrors.ErrorTripDepartureTooSoon
}

// UpdateAutoApprove active ou désactive l'approbation automatique d'un trajet planifié ou en cours.
func (r *tripWriteRepositoryImpl) UpdateAutoApprove(ctx context.Context, tripID, driverID string, autoApprove bool) error {
	r.logger.Debug("updating trip auto_approve",
		zap.String("tripID", tripID),
		zap.String("driverID", driverID),
		zap.Bool("autoApprove", autoApprove),
	)

	query := `
		UPDATE trips
		SET auto_approve_enabled = $3, updated_at = NOW()
		WHERE trip_id = $1 AND driver_id = $2
		  AND status IN ('scheduled'::trip_status, 'inProgress'::trip_status)`

	tag, err := r.pool.Exec(ctx, query, tripID, driverID, autoApprove)
	if err != nil {
		r.logger.Error("update auto_approve failed", zap.Error(err), zap.String("tripID", tripID))
		return fmt.Errorf("%w: %s", tripErrors.ErrorInternalServer, err.Error())
	}

	if tag.RowsAffected() == 1 {
		r.logger.Info("trip auto_approve updated", zap.String("tripID", tripID))
		return nil
	}

	return r.diagnoseTripUpdateFailure(ctx, tripID, driverID)
}

// StartTrip passe un trajet planifié au statut inProgress de façon atomique.
// Vérifie en transaction que le conducteur n'a pas d'autre trajet inProgress,
// puis met à jour le statut du trajet et l'heure réelle de départ du waypoint de départ.
func (r *tripWriteRepositoryImpl) StartTrip(ctx context.Context, tripID, driverID string) error {
	r.logger.Debug("starting trip",
		zap.String("tripID", tripID),
		zap.String("driverID", driverID),
	)

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		r.logger.Error("begin transaction failed", zap.Error(err))
		return tripErrors.ErrorInternalServer
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Vérifie qu'aucun autre trajet du conducteur n'est déjà inProgress
	var activeCount int
	err = tx.QueryRow(ctx,
		`SELECT COUNT(*) FROM trips WHERE driver_id = $1 AND status = 'inProgress'::trip_status`,
		driverID,
	).Scan(&activeCount)
	if err != nil {
		r.logger.Error("check active trip failed", zap.Error(err))
		return tripErrors.ErrorInternalServer
	}
	if activeCount > 0 {
		return tripErrors.ErrorDriverAlreadyHasActiveTrip
	}

	// Met à jour le statut du trajet et l'heure réelle de départ
	tag, err := tx.Exec(ctx,
		`UPDATE trips
		 SET status = 'inProgress'::trip_status, actual_departure_datetime = NOW(), updated_at = NOW()
		 WHERE trip_id = $1 AND driver_id = $2 AND status = 'scheduled'::trip_status`,
		tripID, driverID,
	)
	if err != nil {
		r.logger.Error("update trip status failed", zap.Error(err), zap.String("tripID", tripID))
		return fmt.Errorf("%w: %s", tripErrors.ErrorInternalServer, err.Error())
	}
	if tag.RowsAffected() == 0 {
		// Rollback implicite via defer ; on diagnostique via un SELECT hors transaction
		if rbErr := tx.Rollback(ctx); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
			r.logger.Warn("rollback failed", zap.Error(rbErr))
		}
		return r.diagnoseTripUpdateFailure(ctx, tripID, driverID)
	}

	// Met à jour l'heure réelle et les minutes réelles sur le waypoint de départ
	_, err = tx.Exec(ctx,
		`UPDATE trips_waypoints
		 SET actual_scheduled_pickup_datetime = NOW(), minutes_from_departure = 0, updated_at = NOW()
		 WHERE trip_id = $1 AND waypoint_type = 'departure'::waypoint_type`,
		tripID,
	)
	if err != nil {
		r.logger.Error("update departure waypoint failed", zap.Error(err), zap.String("tripID", tripID))
		return fmt.Errorf("%w: %s", tripErrors.ErrorInternalServer, err.Error())
	}

	if err := tx.Commit(ctx); err != nil {
		r.logger.Error("commit transaction failed", zap.Error(err), zap.String("tripID", tripID))
		return tripErrors.ErrorInternalServer
	}

	r.logger.Info("trip started", zap.String("tripID", tripID))
	return nil
}

// EndTrip passe un trajet en cours au statut completed de façon atomique.
// Met à jour actual_arrival_datetime sur le trajet et actual_scheduled_pickup_datetime
// sur le waypoint d'arrivée.
func (r *tripWriteRepositoryImpl) EndTrip(ctx context.Context, tripID, driverID string) error {
	r.logger.Debug("ending trip",
		zap.String("tripID", tripID),
		zap.String("driverID", driverID),
	)

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		r.logger.Error("begin transaction failed", zap.Error(err))
		return tripErrors.ErrorInternalServer
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Met à jour le statut du trajet et l'heure réelle d'arrivée
	tag, err := tx.Exec(ctx,
		`UPDATE trips
		 SET status = 'completed'::trip_status, actual_arrival_datetime = NOW(), updated_at = NOW()
		 WHERE trip_id = $1 AND driver_id = $2 AND status = 'inProgress'::trip_status`,
		tripID, driverID,
	)
	if err != nil {
		r.logger.Error("update trip status to completed failed", zap.Error(err), zap.String("tripID", tripID))
		return fmt.Errorf("%w: %s", tripErrors.ErrorInternalServer, err.Error())
	}
	if tag.RowsAffected() == 0 {
		if rbErr := tx.Rollback(ctx); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
			r.logger.Warn("rollback failed", zap.Error(rbErr))
		}
		return r.diagnoseEndTripFailure(ctx, tripID, driverID)
	}

	// Met à jour l'heure réelle d'arrivée sur le waypoint d'arrivée
	_, err = tx.Exec(ctx,
		`UPDATE trips_waypoints
		 SET actual_arrival_datetime = NOW(), updated_at = NOW()
		 WHERE trip_id = $1 AND waypoint_type = 'arrival'::waypoint_type`,
		tripID,
	)
	if err != nil {
		r.logger.Error("update arrival waypoint failed", zap.Error(err), zap.String("tripID", tripID))
		return fmt.Errorf("%w: %s", tripErrors.ErrorInternalServer, err.Error())
	}

	if err := tx.Commit(ctx); err != nil {
		r.logger.Error("commit transaction failed", zap.Error(err), zap.String("tripID", tripID))
		return tripErrors.ErrorInternalServer
	}

	r.logger.Info("trip ended", zap.String("tripID", tripID))
	return nil
}

// ConfirmWaypointArrival enregistre l'arrivée du conducteur à un stop dans une transaction atomique.
func (r *tripWriteRepositoryImpl) ConfirmWaypointArrival(ctx context.Context, waypointID, driverID string) error {
	r.logger.Debug("confirming waypoint arrival",
		zap.String("waypointID", waypointID),
		zap.String("driverID", driverID),
	)

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		r.logger.Error("begin transaction failed", zap.Error(err))
		return tripErrors.ErrorInternalServer
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Récupère le waypoint, le trip associé (driver_id, status) et l'ordre du waypoint
	var tripID string
	var waypointType string
	var sequencerOrder int16
	var actualArrivalDatetime *time.Time
	var tripDriverID string
	var tripStatus string
	err = tx.QueryRow(ctx, `
		SELECT w.trip_id, w.waypoint_type, w.sequencer_order, w.actual_arrival_datetime,
		       t.driver_id, t.status
		FROM trips_waypoints w
		JOIN trips t ON t.trip_id = w.trip_id
		WHERE w.waypoint_id = $1`,
		waypointID,
	).Scan(&tripID, &waypointType, &sequencerOrder, &actualArrivalDatetime, &tripDriverID, &tripStatus)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return tripErrors.ErrorWaypointNotFound
		}
		r.logger.Error("select waypoint+trip failed", zap.Error(err), zap.String("waypointID", waypointID))
		return tripErrors.ErrorInternalServer
	}

	if tripDriverID != driverID {
		return tripErrors.ErrorUnauthorized
	}
	if tripStatus != "inProgress" {
		return tripErrors.ErrorTripNotInProgress
	}
	if waypointType != "stop" {
		return tripErrors.ErrorWaypointNotAStop
	}
	if actualArrivalDatetime != nil {
		return tripErrors.ErrorWaypointAlreadyArrived
	}

	// Vérifie qu'aucun autre stop de ce trajet n'est arrivé sans être encore parti
	var activeStops int
	err = tx.QueryRow(ctx, `
		SELECT COUNT(*) FROM trips_waypoints
		WHERE trip_id = $1
		  AND waypoint_type = 'stop'::waypoint_type
		  AND waypoint_id != $2
		  AND actual_arrival_datetime IS NOT NULL
		  AND actual_scheduled_pickup_datetime IS NULL`,
		tripID, waypointID,
	).Scan(&activeStops)
	if err != nil {
		r.logger.Error("check active stops failed", zap.Error(err), zap.String("tripID", tripID))
		return tripErrors.ErrorInternalServer
	}
	if activeStops > 0 {
		return tripErrors.ErrorAnotherStopAlreadyActive
	}

	// Vérifie que le waypoint précédent (sequencer_order - 1) a été confirmé
	// (actual_scheduled_pickup_datetime IS NOT NULL)
	var prevConfirmed bool
	err = tx.QueryRow(ctx, `
		SELECT COUNT(*) > 0 FROM trips_waypoints
		WHERE trip_id = $1
		  AND sequencer_order = $2
		  AND actual_scheduled_pickup_datetime IS NOT NULL`,
		tripID, sequencerOrder-1,
	).Scan(&prevConfirmed)
	if err != nil {
		r.logger.Error("check previous waypoint failed", zap.Error(err), zap.String("tripID", tripID))
		return tripErrors.ErrorInternalServer
	}
	if !prevConfirmed {
		return tripErrors.ErrorPreviousWaypointNotConfirmed
	}

	// Enregistre l'arrivée
	_, err = tx.Exec(ctx, `
		UPDATE trips_waypoints
		SET actual_arrival_datetime = NOW(), updated_at = NOW()
		WHERE waypoint_id = $1`,
		waypointID,
	)
	if err != nil {
		r.logger.Error("update waypoint arrival failed", zap.Error(err), zap.String("waypointID", waypointID))
		return fmt.Errorf("%w: %s", tripErrors.ErrorInternalServer, err.Error())
	}

	if err := tx.Commit(ctx); err != nil {
		r.logger.Error("commit transaction failed", zap.Error(err), zap.String("waypointID", waypointID))
		return tripErrors.ErrorInternalServer
	}

	r.logger.Info("waypoint arrival confirmed", zap.String("waypointID", waypointID), zap.String("tripID", tripID))
	return nil
}

// ConfirmWaypointDeparture enregistre le départ du conducteur d'un stop dans une transaction atomique.
func (r *tripWriteRepositoryImpl) ConfirmWaypointDeparture(ctx context.Context, waypointID, driverID string) error {
	r.logger.Debug("confirming waypoint departure",
		zap.String("waypointID", waypointID),
		zap.String("driverID", driverID),
	)

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		r.logger.Error("begin transaction failed", zap.Error(err))
		return tripErrors.ErrorInternalServer
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var tripID string
	var waypointType string
	var actualArrivalDatetime *time.Time
	var actualDepartureDatetime *time.Time
	var tripDriverID string
	var tripStatus string
	err = tx.QueryRow(ctx, `
		SELECT w.trip_id, w.waypoint_type, w.actual_arrival_datetime, w.actual_scheduled_pickup_datetime,
		       t.driver_id, t.status
		FROM trips_waypoints w
		JOIN trips t ON t.trip_id = w.trip_id
		WHERE w.waypoint_id = $1`,
		waypointID,
	).Scan(&tripID, &waypointType, &actualArrivalDatetime, &actualDepartureDatetime, &tripDriverID, &tripStatus)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return tripErrors.ErrorWaypointNotFound
		}
		r.logger.Error("select waypoint+trip failed", zap.Error(err), zap.String("waypointID", waypointID))
		return tripErrors.ErrorInternalServer
	}

	if tripDriverID != driverID {
		return tripErrors.ErrorUnauthorized
	}
	if tripStatus != "inProgress" {
		return tripErrors.ErrorTripNotInProgress
	}
	if waypointType != "stop" {
		return tripErrors.ErrorWaypointNotAStop
	}
	if actualArrivalDatetime == nil {
		return tripErrors.ErrorWaypointNotArrived
	}
	if actualDepartureDatetime != nil {
		return tripErrors.ErrorWaypointAlreadyDeparted
	}

	_, err = tx.Exec(ctx, `
		UPDATE trips_waypoints tw
		SET actual_scheduled_pickup_datetime = NOW(),
		    minutes_from_departure = GREATEST(0, (EXTRACT(EPOCH FROM (NOW() - t.actual_departure_datetime)) / 60)::INTEGER),
		    updated_at = NOW()
		FROM trips t
		WHERE tw.waypoint_id = $1
		  AND t.trip_id = $2`,
		waypointID, tripID,
	)
	if err != nil {
		r.logger.Error("update waypoint departure failed", zap.Error(err), zap.String("waypointID", waypointID))
		return fmt.Errorf("%w: %s", tripErrors.ErrorInternalServer, err.Error())
	}

	if err := tx.Commit(ctx); err != nil {
		r.logger.Error("commit transaction failed", zap.Error(err), zap.String("waypointID", waypointID))
		return tripErrors.ErrorInternalServer
	}

	r.logger.Info("waypoint departure confirmed", zap.String("waypointID", waypointID), zap.String("tripID", tripID))
	return nil
}

// diagnoseEndTripFailure effectue un SELECT pour déterminer pourquoi l'UPDATE endTrip a affecté 0 lignes.
func (r *tripWriteRepositoryImpl) diagnoseEndTripFailure(ctx context.Context, tripID, driverID string) error {
	var foundDriverID string
	var foundStatus string
	err := r.pool.QueryRow(ctx, `SELECT driver_id, status FROM trips WHERE trip_id = $1`, tripID).
		Scan(&foundDriverID, &foundStatus)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return tripErrors.ErrorTripNotFound
		}
		r.logger.Error("select trip for diagnosis failed", zap.Error(err), zap.String("tripID", tripID))
		return tripErrors.ErrorInternalServer
	}
	if foundDriverID != driverID {
		return tripErrors.ErrorUnauthorized
	}
	return tripErrors.ErrorTripNotInProgress
}

// diagnoseTripUpdateFailure effectue un SELECT pour déterminer pourquoi un UPDATE a affecté 0 lignes.
func (r *tripWriteRepositoryImpl) diagnoseTripUpdateFailure(ctx context.Context, tripID, driverID string) error {
	var foundDriverID string
	var foundStatus string
	selectQuery := `SELECT driver_id, status FROM trips WHERE trip_id = $1`
	err := r.pool.QueryRow(ctx, selectQuery, tripID).Scan(&foundDriverID, &foundStatus)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return tripErrors.ErrorTripNotFound
		}
		r.logger.Error("select trip for diagnosis failed", zap.Error(err), zap.String("tripID", tripID))
		return tripErrors.ErrorInternalServer
	}

	if foundDriverID != driverID {
		return tripErrors.ErrorUnauthorized
	}
	return tripErrors.ErrorTripNotScheduled
}
