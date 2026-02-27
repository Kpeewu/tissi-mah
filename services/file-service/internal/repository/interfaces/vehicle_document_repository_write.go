package interfaces

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/file-service/internal/domain"
)

type VehicleDocumentRepositoryWrite interface {
	// Crée un nouveau document véhicule
	Create(ctx context.Context, doc *domain.VehicleDocument) (string, error)

	// Met à jour un document véhicule
	Update(ctx context.Context, doc *domain.VehicleDocument) (*domain.VehicleDocument, error)

	// Supprime un document véhicule
	Delete(ctx context.Context, documentID string) error

	// Marque un document comme remplacé (is_current = false)
	MarkAsReplaced(ctx context.Context, documentID string, replacedBy string) error
}
