package interfaces

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/vehicle-service/internal/domain"
)

// VehicleService définit le contrat métier pour la gestion des véhicules.
type VehicleService interface {
	AddVehicle(ctx context.Context, input AddVehicleInput) (string, error)
	GetVehicleDetails(ctx context.Context, userID string, vehicleID string) (*domain.VehicleDetails, error)
	GetUserVehicles(ctx context.Context, userID string) ([]*domain.VehiclePreview, error)
	UpdateVehicle(ctx context.Context, input UpdateVehicleInput) error
	DeleteVehicle(ctx context.Context, userID string, vehicleID string) error
	VerifyVehicle(ctx context.Context, vehicleID string, isVerified bool) error
}

// AddVehicleInput contient les données nécessaires pour créer un véhicule.
type AddVehicleInput struct {
	UserID        string
	Brand         string
	NumberOfSeats int16
	BrandModel    string
	Color         string
	LicencePlate  string
}

// UpdateVehicleInput contient les données modifiables d'un véhicule.
type UpdateVehicleInput struct {
	VehicleID    string
	UserID       string
	Color        string
	LicencePlate string
}
