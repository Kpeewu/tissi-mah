package client

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/vehicle-service/internal/domain"
)

// FileServiceClient définit le contrat pour récupérer les documents d'un véhicule.
type FileServiceClient interface {
	// GetVehicleDocuments récupère les documents (assurance, carte grise) d'un véhicule
	// avec leurs URLs et statuts de revue.
	GetVehicleDocuments(ctx context.Context, vehicleID string) (domain.VehicleDocuments, error)

	// GetCurrentUserDocument récupère l'URL et le statut du document courant d'un utilisateur.
	// Retourne ("", "MISSING", nil) si aucun document n'est trouvé.
	GetCurrentUserDocument(ctx context.Context, userID string, docType string) (url string, status string, err error)

	// Close libère la connexion gRPC.
	Close() error
}
