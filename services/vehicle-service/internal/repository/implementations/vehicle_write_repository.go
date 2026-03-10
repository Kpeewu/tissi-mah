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

type vehicleWriteRepositoryImpl struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

// NewVehicleWriteRepository crée une nouvelle instance du repository d'écriture des véhicules.
func NewVehicleWriteRepository(pool *pgxpool.Pool, logger *zap.Logger) i.VehicleRepositoryWrite {
	return &vehicleWriteRepositoryImpl{
		pool:   pool,
		logger: logger,
	}
}

// Create insère un nouveau véhicule et retourne son identifiant.
func (r *vehicleWriteRepositoryImpl) Create(ctx context.Context, vehicle *domain.Vehicle) (string, error) {
	r.logger.Debug("creating vehicle",
		zap.String("vehicleID", vehicle.VehicleID),
		zap.String("userID", vehicle.UserID),
		zap.String("licencePlate", vehicle.LicencePlate),
	)

	query := `INSERT INTO vehicles (vehicle_id, user_id, brand, number_of_seats, brand_model, color, licence_plate)
	          VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING vehicle_id`

	var vehicleID string
	err := r.pool.QueryRow(ctx, query,
		vehicle.VehicleID, vehicle.UserID, vehicle.Brand,
		vehicle.NumberOfSeats, vehicle.BrandModel, vehicle.Color, vehicle.LicencePlate,
	).Scan(&vehicleID)

	if err != nil {
		r.logger.Error("insert vehicle failed", zap.Error(err), zap.String("vehicleID", vehicle.VehicleID))
		if errors.Is(err, pgx.ErrNoRows) {
			return "", vehicleErrors.ErrorDataRetrievalFailed
		}
		return "", vehicleErrors.ErrorInternalServer
	}

	r.logger.Info("vehicle created", zap.String("vehicleID", vehicleID))
	return vehicleID, nil
}

// Update met à jour les champs modifiables d'un véhicule et retourne le véhicule mis à jour.
func (r *vehicleWriteRepositoryImpl) Update(ctx context.Context, vehicle *domain.Vehicle) (*domain.Vehicle, error) {
	r.logger.Debug("updating vehicle", zap.String("vehicleID", vehicle.VehicleID))

	query := `UPDATE vehicles SET
	                color = $1, licence_plate = $2, updated_at = NOW()
	          WHERE vehicle_id = $3
	          RETURNING vehicle_id, user_id, brand, number_of_seats, brand_model, color,
	                    licence_plate, is_verified, created_at, updated_at`

	updated := &domain.Vehicle{}
	err := r.pool.QueryRow(ctx, query,
		vehicle.Color, vehicle.LicencePlate, vehicle.VehicleID,
	).Scan(
		&updated.VehicleID,
		&updated.UserID,
		&updated.Brand,
		&updated.NumberOfSeats,
		&updated.BrandModel,
		&updated.Color,
		&updated.LicencePlate,
		&updated.IsVerified,
		&updated.CreatedAt,
		&updated.UpdatedAt,
	)

	if err != nil {
		r.logger.Error("update vehicle failed", zap.Error(err), zap.String("vehicleID", vehicle.VehicleID))
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, vehicleErrors.ErrorVehicleNotFound
		}
		return nil, vehicleErrors.ErrorInternalServer
	}

	r.logger.Info("vehicle updated", zap.String("vehicleID", vehicle.VehicleID))
	return updated, nil
}

// Delete supprime un véhicule par son identifiant.
func (r *vehicleWriteRepositoryImpl) Delete(ctx context.Context, vehicleID string) error {
	r.logger.Debug("deleting vehicle", zap.String("vehicleID", vehicleID))

	query := `DELETE FROM vehicles WHERE vehicle_id = $1`

	result, err := r.pool.Exec(ctx, query, vehicleID)

	if err != nil {
		r.logger.Error("delete vehicle failed", zap.Error(err), zap.String("vehicleID", vehicleID))
		return vehicleErrors.ErrorInternalServer
	}

	if result.RowsAffected() == 0 {
		r.logger.Debug("vehicle not found for deletion", zap.String("vehicleID", vehicleID))
		return vehicleErrors.ErrorVehicleNotFound
	}

	r.logger.Info("vehicle deleted", zap.String("vehicleID", vehicleID))
	return nil
}

// SetVerified met à jour le statut de vérification d'un véhicule.
func (r *vehicleWriteRepositoryImpl) SetVerified(ctx context.Context, vehicleID string, isVerified bool) error {
	r.logger.Debug("set vehicle verified", zap.String("vehicleID", vehicleID), zap.Bool("isVerified", isVerified))

	query := `UPDATE vehicles SET is_verified = $1, updated_at = NOW() WHERE vehicle_id = $2`

	result, err := r.pool.Exec(ctx, query, isVerified, vehicleID)

	if err != nil {
		r.logger.Error("set verified failed", zap.Error(err), zap.String("vehicleID", vehicleID))
		return vehicleErrors.ErrorInternalServer
	}

	if result.RowsAffected() == 0 {
		r.logger.Debug("vehicle not found for verification", zap.String("vehicleID", vehicleID))
		return vehicleErrors.ErrorVehicleNotFound
	}

	r.logger.Info("vehicle verification status updated", zap.String("vehicleID", vehicleID), zap.Bool("isVerified", isVerified))
	return nil
}
