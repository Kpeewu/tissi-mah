package interfaces

import (
	"context"
	"time"

	"github.com/Kpeewu/tissi-mah/services/trips-service/internal/client"
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

	// GetTripByID retourne les détails complets d'un trajet avec ses waypoints.
	GetTripByID(ctx context.Context, input *GetTripByIDInput) (*TripDetailResult, error)

	// GetDriverTripDetails retourne les détails complets d'un trajet pour le conducteur.
	GetDriverTripDetails(ctx context.Context, input *GetDriverTripDetailsInput) (*DriverTripDetailResult, error)

	// GetPassengerTripDetails retourne les détails d'un trajet pour un passager.
	GetPassengerTripDetails(ctx context.Context, input *GetPassengerTripDetailsInput) (*PassengerTripDetailResult, error)

	// UpdateAvailableSeats met à jour le nombre de places disponibles d'un trajet.
	UpdateAvailableSeats(ctx context.Context, input *UpdateAvailableSeatsInput) error

	// CancelTrip annule un trajet planifié et toutes ses réservations associées.
	// Le trajet doit avoir le statut "scheduled".
	CancelTrip(ctx context.Context, input *CancelTripInput) error

	// CancelWaypoint annule un waypoint de type "stop" d'un trajet planifié.
	// Le trajet doit avoir le statut "scheduled".
	CancelWaypoint(ctx context.Context, input *CancelWaypointInput) error

	// GetScheduledTripsPreviews recherche les trajets/segments disponibles pour un passager.
	// Les filtres textuels (départ + arrivée) sont obligatoires.
	GetScheduledTripsPreviews(ctx context.Context, input *GetScheduledTripsPreviewsInput) (*ScheduledTripsPreviewsResult, error)

	// IncrementLegBookedSeats incrémente/décrémente booked_seats sur les legs d'un segment.
	// Appelé par booking-service à chaque réservation (+delta) ou annulation (-delta).
	IncrementLegBookedSeats(ctx context.Context, input *IncrementLegBookedSeatsInput) error

	// SyncLegBookedSeats force la valeur de booked_seats pour chaque leg (réconciliation).
	// Appelé par le job de réconciliation du booking-service.
	SyncLegBookedSeats(ctx context.Context, input *SyncLegBookedSeatsInput) error
}

// GetTripsPreviewsInput contient les paramètres de la requête de liste.
type GetTripsPreviewsInput struct {
	DriverID  string
	PageIndex int
}

// TripPreviewResult contient les données enrichies d'un trajet pour l'affichage en liste.
type TripPreviewResult struct {
	TripID                 string
	DriverID               string
	DriverName             string
	VehicleID              string
	VehicleBrand           string
	VehiclePlate           string
	DepartureDatetime      time.Time
	TotalSeats             int16
	AvailableSeats         int16
	DepartureLocationName  string
	ArrivalLocationName    string
	DepartureWaypointID    string
	ArrivalWaypointID      string
	SegmentPrice           int
	SegmentDurationMinutes int
	DriverProfileImageURL  string
	DriverRatingAverage    float64
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
	PaymentMethodsAccepted   []string
	AllowLuggages            bool
	AllowPets                bool
	AllowSmoking             bool
	AllowFood                bool
	AutoApprove              bool
	Description              string
	// RoutePolyline encodage Google polyline du tracé, calculé côté front via
	// geolocation-service. Optionnel — si vide, le tracé ne sera pas affiché
	// aux passagers tant qu'aucun calcul n'a été fait côté serveur.
	RoutePolyline string
	Waypoints     []WaypointInput
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
	AllowLuggages         bool
	AllowPets             bool
	AllowFood             bool
	AllowSmoking          bool
	AutoApprove           bool
	Description           string
	GenerationHorizonDays int
	Waypoints             []WaypointInput
}

// GetTripByIDInput contient les données nécessaires à la récupération d'un trajet.
type GetTripByIDInput struct {
	TripID string
}

// TripDetailResult contient les détails complets d'un trajet avec ses waypoints.
type TripDetailResult struct {
	TripID                   string
	DriverID                 string
	Status                   string
	TotalSeats               int16
	AvailableSeats           int16
	PricePerSeat             int
	AutoApproveEnabled       bool
	DepartureDatetime        time.Time
	EstimatedArrivalDatetime time.Time
	VehicleID                string
	VehicleBrand             string
	VehiclePlate             string
	Waypoints                []WaypointDetailResult
}

// WaypointDetailResult contient les informations d'un waypoint.
type WaypointDetailResult struct {
	WaypointID              string
	WaypointType            string
	SequencerOrder          int16
	LocationName            string
	City                    string
	ScheduledPickupDatetime *time.Time
	PriceFromPrevious       int
}

// UpdateAvailableSeatsInput contient les données pour la mise à jour des places.
type UpdateAvailableSeatsInput struct {
	TripID            string
	NewAvailableSeats int16
}

// CancelTripInput contient les données nécessaires à l'annulation d'un trajet.
type CancelTripInput struct {
	DriverID           string
	TripID             string
	CancellationReason string
}

// CancelWaypointInput contient les données nécessaires à l'annulation d'un waypoint.
type CancelWaypointInput struct {
	DriverID           string
	WaypointID         string
	CancellationReason string
}

// GetScheduledTripsPreviewsInput contient les paramètres de recherche passager.
type GetScheduledTripsPreviewsInput struct {
	PassengerPositionLng  *float64
	PassengerPositionLat  *float64
	DistanceRange         *int    // km, défaut 5
	DepartureLocationName string  // obligatoire
	ArrivalLocationName   string  // obligatoire
	TripStartDate         *string // "YYYY-MM-DD" (UTC)
	TripStartHour         *string // "HH:MM" (UTC)
	TripArrivalHour       *string // "HH:MM" (UTC)
	PageIndex             int
}

// ScheduledTripsPreviewsResult contient les résultats paginés de la recherche passager.
type ScheduledTripsPreviewsResult struct {
	Previews   []*TripPreviewResult
	NextIndex  int // -1 si plus de résultats
	TotalCount int
}

// IncrementLegBookedSeatsInput contient les données pour incrémenter/décrémenter booked_seats.
type IncrementLegBookedSeatsInput struct {
	TripID    string
	FromOrder int // sequencer_order du waypoint de départ (inclusif)
	ToOrder   int // sequencer_order du waypoint d'arrivée (exclusif)
	Delta     int // +N pour réservation, -N pour annulation
}

// LegBookedSeats contient le nombre de places réservées pour un leg donné.
type LegBookedSeats struct {
	SequencerOrder int
	BookedSeats    int
}

// SyncLegBookedSeatsInput contient les données pour la réconciliation des booked_seats.
type SyncLegBookedSeatsInput struct {
	TripID string
	Legs   []LegBookedSeats
}

// =============================================================================
// GetDriverTripDetails / GetPassengerTripDetails
// =============================================================================

// GetDriverTripDetailsInput contient les données nécessaires à la récupération d'un trajet pour le conducteur.
type GetDriverTripDetailsInput struct {
	TripID   string
	DriverID string // depuis x-firebase-uid, pour vérifier la propriété
}

// GetPassengerTripDetailsInput contient les données nécessaires à la récupération d'un trajet pour un passager.
type GetPassengerTripDetailsInput struct {
	TripID string
}

// DriverTripDetailResult contient les détails complets d'un trajet pour le conducteur.
type DriverTripDetailResult struct {
	TripID                   string
	DriverID                 string
	Status                   string
	TotalSeats               int16
	AvailableSeats           int16
	PricePerSeat             int
	AutoApproveEnabled       bool
	DepartureDatetime        time.Time
	EstimatedArrivalDatetime time.Time
	ActualDepartureDatetime  *time.Time
	ActualArrivalDatetime    *time.Time
	EstimatedDurationMinutes int
	EstimatedDistanceMeters  int
	VehicleID                string
	VehicleBrand             string
	VehiclePlate             string
	PaymentMethodsAccepted   []string
	AllowLuggages            bool
	AllowPets                bool
	AllowFood                bool
	AllowSmoking             bool
	Description              string
	RoutePolyline            string
	Waypoints                []DriverWaypointDetailResult
	Bookings                 []client.BookingPreview
}

// DriverWaypointDetailResult contient les informations complètes d'un waypoint pour le conducteur.
type DriverWaypointDetailResult struct {
	WaypointID                    string
	WaypointType                  string
	SequencerOrder                int16
	LocationName                  string
	LocationLng                   float64
	LocationLat                   float64
	City                          string
	Country                       string
	ScheduledPickupDatetime       *time.Time
	ActualArrivalDatetime         *time.Time
	ActualScheduledPickupDatetime *time.Time
	MinutesFromDeparture          int
	PriceFromPrevious             int
	IsCancelled                   bool
	CancellationReason            *string
}

// PassengerTripDetailResult contient les détails d'un trajet pour un passager.
type PassengerTripDetailResult struct {
	TripID                   string
	DriverID                 string
	DriverName               string
	DriverProfileImageURL    string
	DriverRatingAverage      float64
	Status                   string
	TotalSeats               int16
	AvailableSeats           int16
	PricePerSeat             int
	DepartureDatetime        time.Time
	EstimatedArrivalDatetime time.Time
	EstimatedDurationMinutes int
	VehicleID                string
	VehicleBrand             string
	VehiclePlate             string
	PaymentMethodsAccepted   []string
	AllowLuggages            bool
	AllowPets                bool
	AllowFood                bool
	AllowSmoking             bool
	Description              string
	RoutePolyline            string
	EstimatedDistanceMeters  int
	Waypoints                []PassengerWaypointDetailResult
}

// PassengerWaypointDetailResult contient les informations d'un waypoint pour un passager.
type PassengerWaypointDetailResult struct {
	WaypointID              string
	WaypointType            string
	SequencerOrder          int16
	LocationName            string
	City                    string
	ScheduledPickupDatetime *time.Time
	PriceFromPrevious       int
	MinutesFromDeparture    int
	IsCancelled             bool
}
