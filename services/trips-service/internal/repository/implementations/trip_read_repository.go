package implementations

import (
	"context"
	"errors"

	"github.com/Kpeewu/tissi-mah/services/trips-service/internal/domain"
	i "github.com/Kpeewu/tissi-mah/services/trips-service/internal/repository/interfaces"
	tripErrors "github.com/Kpeewu/tissi-mah/services/trips-service/pkg/errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// GetTripTotalSeats retourne le nombre total de places d'un trajet.
func (r *tripReadRepositoryImpl) GetTripTotalSeats(ctx context.Context, tripID string) (int16, error) {
	var totalSeats int16
	err := r.pool.QueryRow(ctx, `SELECT total_seats FROM trips WHERE trip_id = $1 AND deleted_at IS NULL`, tripID).Scan(&totalSeats)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, tripErrors.ErrorTripNotFound
		}
		r.logger.Error("GetTripTotalSeats failed", zap.Error(err), zap.String("tripID", tripID))
		return 0, tripErrors.ErrorInternalServer
	}
	return totalSeats, nil
}

// GetTripByID retourne les détails complets d'un trajet avec ses waypoints.
func (r *tripReadRepositoryImpl) GetTripByID(ctx context.Context, tripID string) (*domain.Trip, []*domain.Waypoint, error) {
	r.logger.Debug("GetTripByID", zap.String("tripID", tripID))

	// Récupérer le trajet
	tripQuery := `
		SELECT trip_id, driver_id, vehicle_id, recurring_pattern_id,
		       departure_datetime, actual_departure_datetime,
		       estimated_arrival_datetime, actual_arrival_datetime,
		       estimated_duration_minutes, estimated_distance_meters,
		       total_seats, available_seats, price_per_seat,
		       payment_methods_accepted::TEXT[],
		       allow_luggages, allow_pets, allow_food, allow_smoking,
		       status, auto_approve_enabled,
		       canceller_id, cancellation_reason,
		       description, created_at, updated_at
		FROM trips
		WHERE trip_id = $1 AND deleted_at IS NULL`

	trip := &domain.Trip{}
	err := r.pool.QueryRow(ctx, tripQuery, tripID).Scan(
		&trip.TripID, &trip.DriverID, &trip.VehicleID, &trip.RecurringPatternID,
		&trip.DepartureDatetime, &trip.ActualDepartureDatetime,
		&trip.EstimatedArrivalDatetime, &trip.ActualArrivalDatetime,
		&trip.EstimatedDurationMinutes, &trip.EstimatedDistanceMeters,
		&trip.TotalSeats, &trip.AvailableSeats, &trip.PricePerSeat,
		&trip.PaymentMethodsAccepted,
		&trip.AllowLuggages, &trip.AllowPets, &trip.AllowFood, &trip.AllowSmoking,
		&trip.Status, &trip.AutoApproveEnabled,
		&trip.CancellerID, &trip.CancellationReason,
		&trip.Description, &trip.CreatedAt, &trip.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, tripErrors.ErrorTripNotFound
		}
		r.logger.Error("GetTripByID trip query failed", zap.Error(err), zap.String("tripID", tripID))
		return nil, nil, tripErrors.ErrorDataRetrievalFailed
	}

	// Récupérer les waypoints du trajet
	waypointQuery := `
		SELECT waypoint_id, trip_id, sequencer_order, waypoint_type,
		       location_name, location_lng, location_lat, city, country,
		       scheduled_pickup_datetime, actual_arrival_datetime,
		       actual_scheduled_pickup_datetime,
		       minutes_from_departure, price_from_previous,
		       created_at, updated_at
		FROM trips_waypoints
		WHERE trip_id = $1 AND deleted_at IS NULL
		ORDER BY sequencer_order ASC`

	rows, err := r.pool.Query(ctx, waypointQuery, tripID)
	if err != nil {
		r.logger.Error("GetTripByID waypoints query failed", zap.Error(err), zap.String("tripID", tripID))
		return nil, nil, tripErrors.ErrorDataRetrievalFailed
	}
	defer rows.Close()

	var waypoints []*domain.Waypoint
	for rows.Next() {
		wp := &domain.Waypoint{}
		if err := rows.Scan(
			&wp.WaypointID, &wp.TripID, &wp.SequencerOrder, &wp.WaypointType,
			&wp.LocationName, &wp.LocationLng, &wp.LocationLat, &wp.City, &wp.Country,
			&wp.ScheduledPickupDatetime, &wp.ActualArrivalDatetime,
			&wp.ActualScheduledPickupDatetime,
			&wp.MinutesFromDeparture, &wp.PriceFromPrevious,
			&wp.CreatedAt, &wp.UpdatedAt,
		); err != nil {
			r.logger.Error("GetTripByID waypoint scan failed", zap.Error(err))
			return nil, nil, tripErrors.ErrorDataRetrievalFailed
		}
		waypoints = append(waypoints, wp)
	}

	if err := rows.Err(); err != nil {
		r.logger.Error("GetTripByID waypoints rows error", zap.Error(err))
		return nil, nil, tripErrors.ErrorDataRetrievalFailed
	}

	return trip, waypoints, nil
}

type tripReadRepositoryImpl struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

func NewTripReadRepository(pool *pgxpool.Pool, logger *zap.Logger) i.TripRepositoryRead {
	return &tripReadRepositoryImpl{pool: pool, logger: logger}
}

// GetDriverTripsPreviews retourne la liste paginée des trajets d'un conducteur
// dont le statut est différent de "completed", avec les noms des points de départ et d'arrivée.
func (r *tripReadRepositoryImpl) GetDriverTripsPreviews(ctx context.Context, driverID string, pageIndex int) ([]*domain.TripPreview, error) {
	r.logger.Debug("GetDriverTripsPreviews", zap.String("driverID", driverID), zap.Int("pageIndex", pageIndex))

	query := `
		SELECT
			t.trip_id,
			t.driver_id,
			t.vehicle_id,
			t.departure_datetime,
			t.total_seats,
			t.available_seats,
			dep.location_name AS departure_location_name,
			arr.location_name AS arrival_location_name
		FROM trips t
		JOIN trips_waypoints dep ON dep.trip_id = t.trip_id AND dep.waypoint_type = 'departure'::waypoint_type
		JOIN trips_waypoints arr ON arr.trip_id = t.trip_id AND arr.waypoint_type = 'arrival'::waypoint_type
		WHERE t.driver_id = $1
		  AND t.status <> 'completed'::trip_status
		  AND t.deleted_at IS NULL
		ORDER BY t.departure_datetime DESC
		LIMIT 10 OFFSET $2`

	return r.scanTripPreviews(ctx, query, driverID, pageIndex)
}

// GetDriverCompletedTripsPreviews retourne la liste paginée des trajets complétés d'un conducteur.
func (r *tripReadRepositoryImpl) GetDriverCompletedTripsPreviews(ctx context.Context, driverID string, pageIndex int) ([]*domain.TripPreview, error) {
	r.logger.Debug("GetDriverCompletedTripsPreviews", zap.String("driverID", driverID), zap.Int("pageIndex", pageIndex))

	query := `
		SELECT
			t.trip_id,
			t.driver_id,
			t.vehicle_id,
			t.departure_datetime,
			t.total_seats,
			t.available_seats,
			dep.location_name AS departure_location_name,
			arr.location_name AS arrival_location_name
		FROM trips t
		JOIN trips_waypoints dep ON dep.trip_id = t.trip_id AND dep.waypoint_type = 'departure'::waypoint_type
		JOIN trips_waypoints arr ON arr.trip_id = t.trip_id AND arr.waypoint_type = 'arrival'::waypoint_type
		WHERE t.driver_id = $1
		  AND t.status = 'completed'::trip_status
		  AND t.deleted_at IS NULL
		ORDER BY t.departure_datetime DESC
		LIMIT 10 OFFSET $2`

	return r.scanTripPreviews(ctx, query, driverID, pageIndex)
}

// scanTripPreviews exécute une requête de previews et scanne les résultats.
func (r *tripReadRepositoryImpl) scanTripPreviews(ctx context.Context, query string, driverID string, pageIndex int) ([]*domain.TripPreview, error) {
	rows, err := r.pool.Query(ctx, query, driverID, pageIndex*10)
	if err != nil {
		r.logger.Error("trip previews query failed", zap.Error(err), zap.String("driverID", driverID))
		return nil, tripErrors.ErrorDataRetrievalFailed
	}
	defer rows.Close()

	var previews []*domain.TripPreview
	for rows.Next() {
		p := &domain.TripPreview{}
		if err := rows.Scan(
			&p.TripID,
			&p.DriverID,
			&p.VehicleID,
			&p.DepartureDatetime,
			&p.TotalSeats,
			&p.AvailableSeats,
			&p.DepartureLocationName,
			&p.ArrivalLocationName,
		); err != nil {
			r.logger.Error("trip previews scan failed", zap.Error(err))
			return nil, tripErrors.ErrorDataRetrievalFailed
		}
		previews = append(previews, p)
	}

	if err := rows.Err(); err != nil {
		r.logger.Error("trip previews rows error", zap.Error(err))
		return nil, tripErrors.ErrorDataRetrievalFailed
	}

	return previews, nil
}

// GetWaypointIDByType retourne l'ID du waypoint d'un type donné pour un trajet.
func (r *tripReadRepositoryImpl) GetWaypointIDByType(ctx context.Context, tripID, waypointType string) (string, error) {
	var waypointID string
	err := r.pool.QueryRow(ctx,
		`SELECT waypoint_id FROM trips_waypoints
		 WHERE trip_id = $1 AND waypoint_type::TEXT = $2 AND deleted_at IS NULL
		 LIMIT 1`,
		tripID, waypointType,
	).Scan(&waypointID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", tripErrors.ErrorWaypointNotFound
		}
		r.logger.Error("GetWaypointIDByType failed", zap.Error(err), zap.String("tripID", tripID))
		return "", tripErrors.ErrorDataRetrievalFailed
	}
	return waypointID, nil
}

// GetTripIDByWaypointID retourne le tripID associé à un waypointID.
func (r *tripReadRepositoryImpl) GetTripIDByWaypointID(ctx context.Context, waypointID string) (string, error) {
	var tripID string
	err := r.pool.QueryRow(ctx,
		`SELECT trip_id FROM trips_waypoints WHERE waypoint_id = $1 AND deleted_at IS NULL`,
		waypointID,
	).Scan(&tripID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", tripErrors.ErrorWaypointNotFound
		}
		r.logger.Error("GetTripIDByWaypointID failed", zap.Error(err), zap.String("waypointID", waypointID))
		return "", tripErrors.ErrorDataRetrievalFailed
	}
	return tripID, nil
}
