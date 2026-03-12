package domain

import "time"

// WaypointType représente le type d'un point de passage.
type WaypointType string

const (
	WaypointTypeDeparture WaypointType = "departure"
	WaypointTypeStop      WaypointType = "stop"
	WaypointTypeArrival   WaypointType = "arrival"
)

// Waypoint représente un point de passage d'un trajet dans le domaine métier.
type Waypoint struct {
	WaypointID                    string
	TripID                        string
	SequencerOrder                int16
	WaypointType                  WaypointType
	LocationName                  string
	LocationLng                   float64
	LocationLat                   float64
	City                          string
	Country                       string
	ScheduledPickupDatetime       *time.Time
	ActualScheduledPickupDatetime *time.Time
	MinutesFromDeparture          int
	PriceFromPrevious             int
	CreatedAt                     time.Time
	UpdatedAt                     time.Time
}
