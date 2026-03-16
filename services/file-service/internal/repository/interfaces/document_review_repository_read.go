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

	// Récupère une revue par persona_inquiry_id
	GetByPersonaInquiryID(ctx context.Context, personaInquiryID string) (*domain.DocumentReview, error)

	// Récupère toutes les revues liées aux documents d'un utilisateur
	GetByUserID(ctx context.Context, userID string) ([]*domain.DocumentReview, error)

	// Liste les revues avec filtres et pagination
	List(ctx context.Context, userID string, status string, decision string, offset int32, limit int32) ([]*domain.DocumentReview, error)
}
