package interfaces

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/file-service/internal/domain"
)

type DocumentReviewRepositoryRead interface {
	// Récupère une revue par son ID
	GetByID(ctx context.Context, reviewID string) (*domain.DocumentReview, error)

	// Récupère les revues d'un document utilisateur
	GetByUserDocumentID(ctx context.Context, userDocumentID string) ([]*domain.DocumentReview, error)

	// Récupère les revues d'un document véhicule
	GetByVehicleDocumentID(ctx context.Context, vehicleDocumentID string) ([]*domain.DocumentReview, error)
}
