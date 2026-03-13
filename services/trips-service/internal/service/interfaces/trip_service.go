package interfaces

import (
	"context"
	"time"

	"github.com/Kpeewu/tissi-mah/services/trips-service/internal/domain"
)

// TripService définit le contrat métier pour la gestion des trajets.
type TripService interface {
	// CreateTrip crée un nouveau trajet avec ses waypoints.
	// Vérifie que le conducteur est certifié et qu'il n'y a pas de chevauchement.
	CreateTrip(ctx context.Context, input *CreateTripInput) (*domain.Trip, error)

	// CreateRecurringTrip crée un pattern récurrent et génère les instances de trajet
	// dans l'horizon de génération. Vérifie que le conducteur est certifié.
	// Retourne l'ID du pattern créé.
	CreateRecurringTrip(ctx context.Context, input *CreateRecurringTripInput) (string, error)

	// GetTripsPreviews retourne la liste paginée des trajets d'un conducteur
	// enrichie avec le nom du conducteur et les infos du véhicule.
	GetTripsPreviews(ctx context.Context, input *GetTripsPreviewsInput) ([]*TripPreviewResult, error)

	// GetCompletedTripsPreviews retourne la liste paginée des trajets complétés
	// d'un conducteur, enrichie avec le nom du conducteur et les infos du véhicule.
	GetCompletedTripsPreviews(ctx context.Context, input *GetTripsPreviewsInput) ([]*CompletedTripPreviewResult, error)
}

// GetTripsPreviewsInput contient les paramètres de la requête de liste.
type GetTripsPreviewsInput struct {
	DriverID  string
	PageIndex int
}

// TripPreviewResult contient les données enrichies d'un trajet pour l'affichage en liste.
type TripPreviewResult struct {
	TripID                string
	DriverID              string
	DriverName            string
	VehicleID             string
	VehicleBrand          string
	VehiclePlate          string
	DepartureDatetime     time.Time
	TotalSeats            int16
	AvailableSeats        int16
	DepartureLocationName string
	ArrivalLocationName   string
}

// CompletedTripPreviewResult contient les données enrichies d'un trajet complété.
type CompletedTripPreviewResult struct {
	TripID                string
	DriverID              string
	DriverName            string
	VehicleID             string
	VehicleBrand          string
	VehiclePlateNumber    string
	DepartureDatetime     time.Time
	TotalSeats            int16
	AvailableSeats        int16
	DepartureLocationName string
	ArrivalLocationName   string
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

// CreateRecurringTripInput regroupe toutes les données nécessaires à la création
// d'un pattern récurrent.
type CreateRecurringTripInput struct {
	DriverID              string
	VehicleID             string
	DepartureTime         string // RFC3339 — seule la partie heure est utilisée
	RecurrenceType        string // daily | weekly | custom
	DaysOfWeek            []int32
	StartDate             string // YYYY-MM-DD
	EndDate               string // YYYY-MM-DD
	TotalSeats            int
	PricePerSeat          int
	AllowLuggages         bool
	AllowPets             bool
	AllowFood             bool
	AllowSmoking          bool
	AutoApprove           bool
	Description           string
	GenerationHorizonDays int
	Waypoints             []WaypointInput
}
