package interfaces

import (
	"context"
	"time"
)

// TripRepositoryRead définit les opérations de lecture sur la table trips.
type TripRepositoryRead interface {
	// HasOverlappingTrip vérifie qu'un conducteur n'a pas déjà un trajet
	// dont la fenêtre temporelle (avec buffer de 2h) chevauche le nouveau trajet.
	HasOverlappingTrip(ctx context.Context, driverID string, departure time.Time, estimatedArrival time.Time) (bool, error)
}
