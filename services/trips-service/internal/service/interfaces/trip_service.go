package interfaces

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/trips-service/internal/domain"
)

// TripService définit le contrat métier pour la gestion des trajets.
type TripService interface {
	// CreateTrip crée un nouveau trajet avec ses waypoints.
	// Vérifie que le conducteur est certifié et qu'il n'y a pas de chevauchement.
	CreateTrip(ctx context.Context, input *CreateTripInput) (*domain.Trip, error)
}

// WaypointInput représente les données d'un waypoint passées au service.
type WaypointInput struct {
	SequencerOrder    int16
	WaypointType      string
	LocationName      string
	LocationLng       float64
	LocationLat       float64
	City              string
	Country           string
	ScheduledDatetime string // RFC3339, optionnel pour departure
	PriceFromPrevious int
}

// CreateTripInput regroupe toutes les données nécessaires à la création d'un trajet.
type CreateTripInput struct {
	DriverID                 string
	VehicleID                string
	DepartureDatetime        string // RFC3339
	EstimatedArrivalDatetime string // RFC3339
	EstimatedDurationMinutes int
	EstimatedDistanceMeters  int
	TotalSeats               int
	PricePerSeat             int
	PaymentMethodsAccepted   []string
	AllowLuggages            bool
	AllowPets                bool
	AllowSmoking             bool
	AllowFood                bool
	AutoApprove              bool
	Description              string
	Waypoints                []WaypointInput
}
