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
