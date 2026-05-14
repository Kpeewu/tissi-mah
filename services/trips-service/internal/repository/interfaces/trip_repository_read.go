package interfaces

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/trips-service/internal/domain"
)

// SearchTripsParams contient les filtres pour la recherche de trajets passager.
type SearchTripsParams struct {
	PassengerLng          *float64
	PassengerLat          *float64
	DistanceRangeMeters   int     // converti en mètres (défaut 5000)
	DepartureLocationName string  // obligatoire
	ArrivalLocationName   string  // obligatoire
	TripStartDate         *string // "YYYY-MM-DD" (UTC)
	TripStartHour         *string // "HH:MM" (UTC)
	TripArrivalHour       *string // "HH:MM" (UTC)
	PageIndex             int
	PageSize              int // 10 par défaut
}

// SearchTripsResult contient les résultats paginés de la recherche.
type SearchTripsResult struct {
	Previews   []*domain.TripPreview
	TotalCount int
}

// TripRepositoryRead définit les opérations de lecture sur la table trips.
type TripRepositoryRead interface {
	// GetDriverTripsPreviews retourne la liste paginée des trajets d'un conducteur
	// dont le statut est différent de "completed" (10 items, ordre décroissant de départ).
	GetDriverTripsPreviews(ctx context.Context, driverID string, pageIndex int) ([]*domain.TripPreview, error)

	// GetDriverCompletedTripsPreviews retourne la liste paginée des trajets complétés
	// d'un conducteur (10 items, ordre décroissant de départ).
	GetDriverCompletedTripsPreviews(ctx context.Context, driverID string, pageIndex int) ([]*domain.TripPreview, error)

	// GetTripTotalSeats retourne le nombre total de places d'un trajet.
	// Retourne ErrorTripNotFound si le trajet n'existe pas.
	GetTripTotalSeats(ctx context.Context, tripID string) (int16, error)

	// GetTripByID retourne les détails complets d'un trajet avec ses waypoints.
	// Retourne ErrorTripNotFound si le trajet n'existe pas ou est supprimé.
	GetTripByID(ctx context.Context, tripID string) (*domain.Trip, []*domain.Waypoint, error)

	// GetWaypointIDByType retourne l'ID du waypoint d'un type donné (departure, arrival) pour un trajet.
	GetWaypointIDByType(ctx context.Context, tripID, waypointType string) (string, error)

	// GetTripIDByWaypointID retourne le tripID associé à un waypointID.
	GetTripIDByWaypointID(ctx context.Context, waypointID string) (string, error)

	// SearchScheduledTripSegments recherche les trajets/segments disponibles avec pagination.
	// Retourne les segments dont le départ ET l'arrivée matchent les filtres textuels (fuzzy).
	SearchScheduledTripSegments(ctx context.Context, params *SearchTripsParams) (*SearchTripsResult, error)

	// HasActiveTripAsDriver vérifie si un conducteur a un trajet en cours (status = inProgress).
	// Utilisé lors de la vérification d'éligibilité à la suppression de compte.
	HasActiveTripAsDriver(ctx context.Context, driverID string) (bool, error)
}
