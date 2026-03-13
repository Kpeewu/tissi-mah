package client

import "context"

// UserClient définit le contrat pour appeler user-service depuis trips-service.
type UserClient interface {
	// IsVerifiedDriver vérifie que l'utilisateur existe et a un profil conducteur vérifié.
	IsVerifiedDriver(ctx context.Context, userID string) (bool, error)
	// GetDriverName retourne le prénom et nom de l'utilisateur ("FirstName Name").
	GetDriverName(ctx context.Context, userID string) (string, error)
	Close() error
}

// VehicleClient définit le contrat pour appeler vehicle-service depuis trips-service.
type VehicleClient interface {
	// GetVehicleInfo retourne la marque et la plaque d'immatriculation d'un véhicule.
	GetVehicleInfo(ctx context.Context, driverID, vehicleID string) (brand, plate string, err error)
	Close() error
}
