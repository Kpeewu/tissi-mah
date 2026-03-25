package client

import "context"

// TripClient définit le contrat pour appeler trips-service depuis booking-service.
type TripClient interface {
	// GetTripDetails retourne les détails d'un trajet nécessaires à la création d'une réservation.
	GetTripDetails(ctx context.Context, tripID string) (*TripDetails, error)
	// UpdateAvailableSeats met à jour le nombre de places disponibles d'un trajet (réconciliation).
	UpdateAvailableSeats(ctx context.Context, tripID string, newAvailableSeats int) error
	Close() error
}

// TripDetails contient les informations d'un trajet nécessaires au booking-service.
type TripDetails struct {
	TripID             string
	DriverID           string
	Status             string
	AvailableSeats     int
	TotalSeats         int
	PricePerSeat       int
	AutoApproveEnabled bool
	Waypoints          []TripWaypoint
}

// TripWaypoint contient les informations d'un waypoint.
type TripWaypoint struct {
	WaypointID              string
	WaypointType            string
	SequencerOrder          int
	LocationName            string
	City                    string
	ScheduledPickupDatetime string
	PriceFromPrevious       int
}

// UserClient définit le contrat pour appeler user-service depuis booking-service.
type UserClient interface {
	// UserExists vérifie qu'un utilisateur existe.
	UserExists(ctx context.Context, userID string) (bool, error)
	Close() error
}
