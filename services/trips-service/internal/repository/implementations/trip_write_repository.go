package implementations

import (
	"context"
	"errors"
	"fmt"

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

func (r *tripWriteRepositoryImpl) insertTrip(ctx context.Context, tx pgx.Tx, trip *domain.Trip) (string, error) {
	query := `
		INSERT INTO trips (
			trip_id, driver_id, vehicle_id,
			departure_datetime, estimated_arrival_datetime,
			estimated_duration_minutes, estimated_distance_meters,
			total_seats, available_seats, price_per_seat,
			payment_methods_accepted,
			allow_luggages, allow_pets, allow_food, allow_smoking,
			status, auto_approve_enabled, description
		) VALUES (
			$1, $2, $3,
			$4, $5,
			$6, $7,
			$8, $9, $10,
			$11::payment_method[],
			$12, $13, $14, $15,
			$16::trip_status, $17, $18
		)
		RETURNING trip_id`

	var tripID string
	err := tx.QueryRow(ctx, query,
		trip.TripID, trip.DriverID, trip.VehicleID,
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
