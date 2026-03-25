package interfaces

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/trips-service/internal/domain"
)

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
}
