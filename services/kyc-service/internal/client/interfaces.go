package client

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/kyc-service/internal/domain"
)

// FileServiceClient est l'interface pour communiquer avec le file-service via gRPC
type FileServiceClient interface {
	// Crée une revue de document dans le file-service
	CreateDocumentReview(ctx context.Context, review *domain.Review) (*domain.Review, error)

	// Récupère une revue par son ID
	GetDocumentReview(ctx context.Context, reviewID string) (*domain.Review, error)

	// Récupère une revue par persona_inquiry_id
	GetDocumentReviewByPersonaInquiryID(ctx context.Context, personaInquiryID string) (*domain.Review, error)

	// Récupère toutes les revues d'un utilisateur (via ses documents)
	GetDocumentReviewsByUserID(ctx context.Context, userID string) ([]*domain.Review, error)

	// Met à jour une revue existante
	UpdateDocumentReview(ctx context.Context, review *domain.Review) (*domain.Review, error)

	// Liste les revues avec filtres et pagination
	ListDocumentReviews(ctx context.Context, userID string, status string, decision string, page int32, pageSize int32) ([]*domain.Review, error)

	// Ferme la connexion gRPC
	Close() error
}

// PersonaClient est l'interface pour communiquer avec l'API Persona
type PersonaClient interface {
	// Crée une inquiry Persona pour un utilisateur
	CreateInquiry(ctx context.Context, templateID string, referenceID string) (*domain.PersonaInquiry, error)

	// Renouvelle le session token pour une inquiry existante
	ResumeInquiry(ctx context.Context, inquiryID string) (*domain.PersonaSession, error)
}
