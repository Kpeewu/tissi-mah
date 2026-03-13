package domain

import "time"

// TripStatus représente le statut d'un trajet.
type TripStatus string

const (
	TripStatusScheduled  TripStatus = "scheduled"
	TripStatusInProgress TripStatus = "inProgress"
	TripStatusCompleted  TripStatus = "completed"
	TripStatusCancelled  TripStatus = "cancelled"
)

// Trip représente un trajet dans le domaine métier.
type Trip struct {
	TripID                   string
	DriverID                 string
	VehicleID                string
	RecurringPatternID       *string
	DepartureDatetime        time.Time
	ActualDepartureDatetime  *time.Time
	EstimatedArrivalDatetime time.Time
	ActualArrivalDatetime    *time.Time
	EstimatedDurationMinutes int
	EstimatedDistanceMeters  int
	TotalSeats               int16
	AvailableSeats           int16
	PricePerSeat             int
	PaymentMethodsAccepted   []string
	AllowLuggages            bool
	AllowPets                bool
	AllowFood                bool
	AllowSmoking             bool
	Status                   TripStatus
	AutoApproveEnabled       bool
	CancellerID              *string
	CancellationReason       *string
	Description              string
	CreatedAt                time.Time
	UpdatedAt                time.Time
}
