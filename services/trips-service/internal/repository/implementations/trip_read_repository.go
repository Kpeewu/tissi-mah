package implementations

import (
	"context"
	"time"

	i "github.com/Kpeewu/tissi-mah/services/trips-service/internal/repository/interfaces"
	tripErrors "github.com/Kpeewu/tissi-mah/services/trips-service/pkg/errors"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type tripReadRepositoryImpl struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

func NewTripReadRepository(pool *pgxpool.Pool, logger *zap.Logger) i.TripRepositoryRead {
	return &tripReadRepositoryImpl{pool: pool, logger: logger}
}

// HasOverlappingTrip vérifie si le conducteur a déjà un trajet qui chevauche
// la fenêtre [departure - 2h, estimatedArrival + 2h].
// Un trajet chevauche si :  existingDeparture < newArrival + 2h  ET  existingArrival > newDeparture - 2h
func (r *tripReadRepositoryImpl) HasOverlappingTrip(ctx context.Context, driverID string, departure time.Time, estimatedArrival time.Time) (bool, error) {
	r.logger.Debug("checking trip overlap",
		zap.String("driverID", driverID),
		zap.Time("departure", departure),
		zap.Time("estimatedArrival", estimatedArrival),
	)

	query := `
		SELECT EXISTS (
			SELECT 1
			FROM trips
			WHERE driver_id = $1
			  AND status NOT IN ('cancelled', 'completed')
			  AND departure_datetime          < $3 + INTERVAL '2 hours'
			  AND estimated_arrival_datetime  > $2 - INTERVAL '2 hours'
		)`

	var exists bool
	err := r.pool.QueryRow(ctx, query, driverID, departure, estimatedArrival).Scan(&exists)
	if err != nil {
		r.logger.Error("HasOverlappingTrip query failed",
			zap.Error(err),
			zap.String("driverID", driverID),
		)
		return false, tripErrors.ErrorDataRetrievalFailed
	}

	return exists, nil
}
