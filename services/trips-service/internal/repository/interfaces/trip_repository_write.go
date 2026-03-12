package interfaces

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/trips-service/internal/domain"
)

// TripRepositoryWrite définit les opérations d'écriture sur les tables trips et trips_waypoints.
// La création d'un trajet et de ses waypoints est atomique (transaction unique).
type TripRepositoryWrite interface {
	// Create insère un trajet et ses waypoints dans une transaction unique.
	// Retourne le tripID en cas de succès.
	Create(ctx context.Context, trip *domain.Trip, waypoints []*domain.Waypoint) (string, error)
}
