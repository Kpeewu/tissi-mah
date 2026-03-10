package client

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/vehicle-service/internal/domain"
)

// FileServiceClient définit le contrat pour récupérer les documents d'un véhicule.
type FileServiceClient interface {
	// GetVehicleDocuments récupère les documents (assurance, carte grise) d'un véhicule.
	// Retourne des URLs vides si aucun document n'est trouvé.
	GetVehicleDocuments(ctx context.Context, vehicleID string) (domain.VehicleDocuments, error)

	// Close libère la connexion gRPC.
	Close() error
}
