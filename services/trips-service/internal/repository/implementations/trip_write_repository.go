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
			ST_SetSRID(ST_MakePoint($7, $6), 4326)::geography,
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
