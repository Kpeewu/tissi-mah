package interfaces

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/vehicle-service/internal/domain"
)

// VehicleRepositoryRead définit les opérations de lecture sur les véhicules.
type VehicleRepositoryRead interface {
	GetByID(ctx context.Context, vehicleID string) (*domain.Vehicle, error)
	GetByUserID(ctx context.Context, userID string) ([]*domain.VehiclePreview, error)
	ExistsByLicencePlate(ctx context.Context, licencePlate string) (bool, error)
}
