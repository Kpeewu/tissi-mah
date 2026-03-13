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
}
