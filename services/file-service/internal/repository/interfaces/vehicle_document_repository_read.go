package interfaces

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/file-service/internal/domain"
)

type VehicleDocumentRepositoryRead interface {
	// Récupère un document par son ID
	GetByID(ctx context.Context, documentID string) (*domain.VehicleDocument, error)

	// Récupère tous les documents d'un véhicule
	GetByVehicleID(ctx context.Context, vehicleID string) ([]*domain.VehicleDocument, error)
}
