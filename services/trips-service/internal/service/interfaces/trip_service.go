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

	// ChangeTripDateAndTime modifie la date/heure de départ d'un trajet planifié.
	// Seuls les trajets avec le statut "scheduled" peuvent être modifiés.
	ChangeTripDateAndTime(ctx context.Context, input *ChangeTripDateAndTimeInput) error

	// ChangeTripVehicle modifie le véhicule associé à un trajet planifié.
	// Seuls les trajets avec le statut "scheduled" peuvent être modifiés.
	// Vérifie que le véhicule appartient au conducteur via vehicle-service.
	ChangeTripVehicle(ctx context.Context, input *ChangeTripVehicleInput) error

	// ChangeTripAllowances modifie les autorisations d'un trajet planifié.
	// Seuls les trajets avec le statut "scheduled" peuvent être modifiés,
	// et uniquement plus de 24h avant le départ.
	ChangeTripAllowances(ctx context.Context, input *ChangeTripAllowancesInput) error

	// ChangeAutoApprove active ou désactive l'approbation automatique d'un trajet.
	// Le trajet doit avoir le statut "scheduled" ou "inProgress".
	ChangeAutoApprove(ctx context.Context, input *ChangeAutoApproveInput) error

	// StartTrip démarre un trajet planifié. Le conducteur ne peut avoir qu'un seul
	// trajet inProgress à la fois. Passe le statut à "inProgress" et renseigne
	// l'heure réelle de départ sur le trajet et le waypoint de départ.
	StartTrip(ctx context.Context, input *StartTripInput) error

	// EndTrip termine un trajet en cours. Passe le statut à "completed" et renseigne
	// l'heure réelle d'arrivée sur le trajet et le waypoint d'arrivée.
	EndTrip(ctx context.Context, input *EndTripInput) error

	// ConfirmWaypointArrival enregistre l'arrivée du conducteur à un waypoint de type "stop".
	ConfirmWaypointArrival(ctx context.Context, input *ConfirmWaypointArrivalInput) error

	// ConfirmWaypointDeparture enregistre le départ du conducteur d'un waypoint de type "stop".
	ConfirmWaypointDeparture(ctx context.Context, input *ConfirmWaypointDepartureInput) error
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

// ChangeTripDateAndTimeInput contient les données nécessaires à la modification
// de la date/heure de départ d'un trajet.
type ChangeTripDateAndTimeInput struct {
	DriverID          string
	TripID            string
	DepartureDatetime string // RFC3339
}

// ChangeTripVehicleInput contient les données nécessaires à la modification
// du véhicule d'un trajet.
type ChangeTripVehicleInput struct {
	DriverID  string
	TripID    string
	VehicleID string
}

// ChangeTripAllowancesInput contient les données nécessaires à la modification
// des autorisations d'un trajet.
type ChangeTripAllowancesInput struct {
	DriverID      string
	TripID        string
	AllowPets     bool
	AllowFood     bool
	AllowSmoking  bool
	AllowLuggages bool
}

// ChangeAutoApproveInput contient les données nécessaires à la modification
// de l'approbation automatique d'un trajet.
type ChangeAutoApproveInput struct {
	DriverID    string
	TripID      string
	AutoApprove bool
}

// StartTripInput contient les données nécessaires au démarrage d'un trajet.
type StartTripInput struct {
	DriverID string
	TripID   string
}

// EndTripInput contient les données nécessaires à la fin d'un trajet.
type EndTripInput struct {
	DriverID string
	TripID   string
}

// ConfirmWaypointArrivalInput contient les données nécessaires à la confirmation
// de l'arrivée du conducteur à un waypoint de type "stop".
type ConfirmWaypointArrivalInput struct {
	DriverID   string
	WaypointID string
}

// ConfirmWaypointDepartureInput contient les données nécessaires à la confirmation
// du départ du conducteur d'un waypoint de type "stop".
type ConfirmWaypointDepartureInput struct {
	DriverID   string
	WaypointID string
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
