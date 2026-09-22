// Package interfaces définit les contrats du geolocation-service.
package interfaces

import "context"

// Coordinate représente un point géographique (WGS84).
type Coordinate struct {
	Lat float64
	Lng float64
}

// =============================================================================
// ComputeRoute
// =============================================================================

type ComputeRouteInput struct {
	Waypoints     []Coordinate
	Profile       string // V1 : "driving"
	DepartureTime string // RFC3339, optionnel
}

// RouteLeg = segment entre Waypoints[i] et Waypoints[i+1].
type RouteLeg struct {
	DistanceMeters       float64
	DurationSeconds      int64
	MinutesFromDeparture int32
	EstimatedArrivalTime string // RFC3339, vide si DepartureTime non fourni
}

type ComputeRouteResult struct {
	DistanceMeters  float64
	DurationSeconds int64
	PolylineEncoded string
	Legs            []RouteLeg
}

// =============================================================================
// Geocoding
// =============================================================================

type GeocodeInput struct {
	Query         string
	CountryFilter string
	Limit         int32
}

type GeocodeResult struct {
	DisplayName string
	Lat         float64
	Lng         float64
	Country     string
	City        string
	Type        string
}

type ReverseGeocodeInput struct {
	Lat float64
	Lng float64
}

// =============================================================================
// GeolocationService — contrat principal
// =============================================================================

type GeolocationService interface {
	// ComputeRoute calcule distance + durée + polyline + ETAs cumulés par leg.
	ComputeRoute(ctx context.Context, input ComputeRouteInput) (*ComputeRouteResult, error)

	// Geocode résout texte → coordonnées.
	Geocode(ctx context.Context, input GeocodeInput) (*GeocodeOutput, error)

	// ReverseGeocode résout lat/lng → adresse.
	ReverseGeocode(ctx context.Context, input ReverseGeocodeInput) (*GeocodeResult, error)
}

// GeocodeOutput porte les lieux trouvés et, le cas échéant, le nom réellement utilisé
// pour les obtenir : la saisie ne donnant rien a été rapprochée d'une localité connue.
type GeocodeOutput struct {
	Results []*GeocodeResult
	// CorrectedQuery est vide si la saisie a répondu telle quelle.
	CorrectedQuery string
}
