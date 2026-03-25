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
	// GetVehicleInfo retourne la marque, la plaque d'immatriculation et le nombre de places d'un véhicule.
	GetVehicleInfo(ctx context.Context, driverID, vehicleID string) (brand, plate string, numberOfSeats int, err error)
	Close() error
}

// BookingClient définit le contrat pour appeler booking-service depuis trips-service.
type BookingClient interface {
	// StartBookingsForWaypoint démarre les réservations approved d'un waypoint (pickup).
	StartBookingsForWaypoint(ctx context.Context, tripID, waypointID string) error
	// CompleteBookingsForWaypoint complète les réservations inProgress d'un waypoint (dropoff).
	CompleteBookingsForWaypoint(ctx context.Context, tripID, waypointID string) error
	Close() error
}
