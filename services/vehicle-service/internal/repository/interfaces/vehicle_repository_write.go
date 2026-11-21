package interfaces

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/vehicle-service/internal/domain"
)

// VehicleRepositoryWrite définit les opérations d'écriture sur les véhicules.
type VehicleRepositoryWrite interface {
	Create(ctx context.Context, vehicle *domain.Vehicle) (string, error)
	Update(ctx context.Context, vehicle *domain.Vehicle) (*domain.Vehicle, error)
	Delete(ctx context.Context, vehicleID string) error
	SetVerified(ctx context.Context, vehicleID string, isVerified bool) error
}
