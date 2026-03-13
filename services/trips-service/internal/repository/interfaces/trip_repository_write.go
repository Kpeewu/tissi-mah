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

	// CreateRecurringPattern insère un pattern récurrent, ses waypoints de pattern,
	// puis génère les instances de trajet dans l'horizon [startDate, min(endDate, today+horizonDays)].
	// Tout est atomique. Retourne le patternID en cas de succès.
	CreateRecurringPattern(ctx context.Context, pattern *domain.RecurringPattern, patternWaypoints []*domain.PatternWaypoint) (string, error)
}
