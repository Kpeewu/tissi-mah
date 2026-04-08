package client

import "context"

// UserClient définit le contrat pour appeler user-service depuis trips-service.
type UserClient interface {
	// IsVerifiedDriver vérifie que l'utilisateur existe et a un profil conducteur vérifié.
	IsVerifiedDriver(ctx context.Context, userID string) (bool, error)
	// GetDriverName retourne le prénom et nom de l'utilisateur ("FirstName Name").
	GetDriverName(ctx context.Context, userID string) (string, error)
	// GetDriverInfo retourne le nom et l'URL de la photo de profil du conducteur.
	GetDriverInfo(ctx context.Context, userID string) (name, profileImageURL string, err error)
	// GetUserIDByAuthID résout un Firebase UID (authID) en UserID interne.
	GetUserIDByAuthID(ctx context.Context, authID string) (string, error)
	Close() error
}

// VehicleClient définit le contrat pour appeler vehicle-service depuis trips-service.
type VehicleClient interface {
	// GetVehicleInfo retourne la marque, la plaque d'immatriculation et le nombre de places d'un véhicule.
	GetVehicleInfo(ctx context.Context, driverID, vehicleID string) (brand, plate string, numberOfSeats int, err error)
	Close() error
}

// RatingClient définit le contrat pour appeler rating-service depuis trips-service.
type RatingClient interface {
	// GetDriverRatingAverage retourne la note moyenne du conducteur.
	GetDriverRatingAverage(ctx context.Context, driverID string) (float64, error)
	Close() error
}

// BookingClient définit le contrat pour appeler booking-service depuis trips-service.
type BookingClient interface {
	// StartBookingsForWaypoint démarre les réservations approved d'un waypoint (pickup).
	StartBookingsForWaypoint(ctx context.Context, tripID, waypointID string) error
	// CompleteBookingsForWaypoint complète les réservations inProgress d'un waypoint (dropoff).
	CompleteBookingsForWaypoint(ctx context.Context, tripID, waypointID string) error
	// CancelBookingsForWaypoint annule les réservations actives d'un waypoint supprimé.
	CancelBookingsForWaypoint(ctx context.Context, tripID, waypointID string) error
	// CancelBookingsForTrip annule toutes les réservations actives d'un trajet annulé.
	CancelBookingsForTrip(ctx context.Context, tripID string) error
	// GetPassengerIDsForTrip retourne les IDs des passagers avec une réservation active (pour TRIP_MODIFIED).
	GetPassengerIDsForTrip(ctx context.Context, tripID string) ([]string, error)
	Close() error
}
