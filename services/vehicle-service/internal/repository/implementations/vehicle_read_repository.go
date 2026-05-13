package implementations

import (
	"context"
	"errors"

	"github.com/Kpeewu/tissi-mah/services/vehicle-service/internal/domain"
	i "github.com/Kpeewu/tissi-mah/services/vehicle-service/internal/repository/interfaces"
	vehicleErrors "github.com/Kpeewu/tissi-mah/services/vehicle-service/pkg/errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type vehicleReadRepositoryImpl struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

// NewVehicleReadRepository crée une nouvelle instance du repository de lecture des véhicules.
func NewVehicleReadRepository(pool *pgxpool.Pool, logger *zap.Logger) i.VehicleRepositoryRead {
	return &vehicleReadRepositoryImpl{
		pool:   pool,
		logger: logger,
	}
}

// GetByID récupère un véhicule par son identifiant.
func (r *vehicleReadRepositoryImpl) GetByID(ctx context.Context, vehicleID string) (*domain.Vehicle, error) {
	r.logger.Debug("get vehicle by id", zap.String("vehicleID", vehicleID))

	query := `SELECT vehicle_id, user_id, brand, number_of_seats, brand_model, color,
	                 licence_plate, is_verified, created_at, updated_at
	          FROM vehicles WHERE vehicle_id = $1`

	vehicle := &domain.Vehicle{}

	err := r.pool.QueryRow(ctx, query, vehicleID).Scan(
		&vehicle.VehicleID,
		&vehicle.UserID,
		&vehicle.Brand,
		&vehicle.NumberOfSeats,
		&vehicle.BrandModel,
		&vehicle.Color,
		&vehicle.LicencePlate,
		&vehicle.IsVerified,
		&vehicle.CreatedAt,
		&vehicle.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			r.logger.Debug("vehicle not found", zap.String("vehicleID", vehicleID))
			return nil, vehicleErrors.ErrorVehicleNotFound
		}
		r.logger.Error("get vehicle by id failed", zap.Error(err), zap.String("vehicleID", vehicleID))
		return nil, vehicleErrors.ErrorDataRetrievalFailed
	}

	return vehicle, nil
}

// GetByUserID récupère les aperçus de tous les véhicules d'un utilisateur.
func (r *vehicleReadRepositoryImpl) GetByUserID(ctx context.Context, userID string) ([]*domain.VehiclePreview, error) {
	r.logger.Debug("get vehicles by user id", zap.String("userID", userID))

	query := `SELECT vehicle_id, brand, brand_model, licence_plate, is_verified, number_of_seats
	          FROM vehicles WHERE user_id = $1
	          ORDER BY created_at DESC`

	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		r.logger.Error("get vehicles by user id failed", zap.Error(err), zap.String("userID", userID))
		return nil, vehicleErrors.ErrorDataRetrievalFailed
	}
	defer rows.Close()

	var previews []*domain.VehiclePreview
	for rows.Next() {
		preview := &domain.VehiclePreview{}
		err := rows.Scan(
			&preview.VehicleID,
			&preview.Brand,
			&preview.BrandModel,
			&preview.LicencePlate,
			&preview.IsVerified,
			&preview.NumberOfSeats,
		)
		if err != nil {
			r.logger.Error("scan vehicle preview row failed", zap.Error(err))
			return nil, vehicleErrors.ErrorDataRetrievalFailed
		}
		previews = append(previews, preview)
	}

	if err := rows.Err(); err != nil {
		r.logger.Error("rows iteration failed", zap.Error(err))
		return nil, vehicleErrors.ErrorDataRetrievalFailed
	}

	return previews, nil
}

// ExistsByLicencePlate vérifie si une plaque d'immatriculation est déjà utilisée.
func (r *vehicleReadRepositoryImpl) ExistsByLicencePlate(ctx context.Context, licencePlate string) (bool, error) {
	r.logger.Debug("checking licence plate exists", zap.String("licencePlate", licencePlate))

	query := `SELECT EXISTS(SELECT 1 FROM vehicles WHERE licence_plate = $1)`

	var exists bool
	err := r.pool.QueryRow(ctx, query, licencePlate).Scan(&exists)

	if err != nil {
		r.logger.Error("licence plate exists check failed", zap.Error(err))
		return false, vehicleErrors.ErrorDataRetrievalFailed
	}

	return exists, nil
}
