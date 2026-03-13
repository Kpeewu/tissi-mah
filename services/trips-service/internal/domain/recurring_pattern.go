package domain

import "time"

// RecurrenceType représente le type de récurrence d'un pattern.
type RecurrenceType string

const (
	RecurrenceTypeDaily  RecurrenceType = "daily"
	RecurrenceTypeWeekly RecurrenceType = "weekly"
	RecurrenceTypeCustom RecurrenceType = "custom"
)

// RecurringPattern représente le schéma d'un trajet récurrent dans le domaine métier.
type RecurringPattern struct {
	TripPatternID         string
	DriverID              string
	VehicleID             string
	DepartureTime         time.Time // seule la partie heure est significative
	RecurrenceType        RecurrenceType
	DaysOfWeek            []int16 // 1=Lundi … 7=Dimanche
	StartDate             time.Time
	EndDate               time.Time
	TotalSeats            int
	PricePerSeat          int
	AllowLuggages         bool
	AllowPets             bool
	AllowFood             bool
	AllowSmoking          bool
	AutoApproveEnabled    bool
	Description           string
	GenerationHorizonDays int
	LastGeneratedDate     *time.Time
	IsActive              bool
}

// PatternWaypoint représente un point de passage d'un recurring_pattern.
type PatternWaypoint struct {
	PatternWaypointID    string
	TripPatternID        string
	SequencerOrder       int16
	WaypointType         WaypointType
	LocationName         string
	LocationLng          float64
	LocationLat          float64
	City                 string
	Country              string
	PriceFromPrevious    int
	MinutesFromDeparture int
}
