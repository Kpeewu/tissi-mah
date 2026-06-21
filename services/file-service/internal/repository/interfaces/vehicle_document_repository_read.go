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

	// Récupère tous les documents véhicule d'un utilisateur (user_id dénormalisé)
	GetByUserID(ctx context.Context, userID string) ([]*domain.VehicleDocument, error)

	// ListCurrentByStatuses liste les documents véhicule courants (is_current) dont le
	// statut est dans la liste fournie. statuses vide = tous les statuts.
	ListCurrentByStatuses(ctx context.Context, statuses []string, limit int32) ([]*domain.VehicleDocument, error)
}
