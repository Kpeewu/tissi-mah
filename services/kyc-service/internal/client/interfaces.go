package client

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/kyc-service/internal/domain"
)

// FileServiceClient est l'interface pour communiquer avec le file-service via gRPC
type FileServiceClient interface {
	// Récupère le document utilisateur courant par type
	GetCurrentUserDocument(ctx context.Context, userID string, documentType string) (*domain.DocumentRef, error)

	// Récupère un document utilisateur par son ID (renseigne OwnerID = user_id)
	GetUserDocument(ctx context.Context, documentID string) (*domain.DocumentRef, error)

	// Récupère un document véhicule par son ID (renseigne OwnerID = vehicle_id)
	GetVehicleDocument(ctx context.Context, documentID string) (*domain.DocumentRef, error)

	// Récupère tous les documents d'un utilisateur
	GetUserDocuments(ctx context.Context, userID string) ([]*domain.DocumentRef, error)

	// Récupère les documents d'un véhicule
	GetVehicleDocuments(ctx context.Context, vehicleID string) ([]*domain.DocumentRef, error)

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

	// Soumet un document d'identité (recto + verso optionnel) à une inquiry existante
	// via les URLs S3/MinIO de notre file-service (POST /api/v1/government-ids).
	// kind doit être : "identification_card", "passport" ou "driver_license".
	// Permet au SDK Persona de sauter l'étape capture et d'aller directement au selfie.
	SubmitGovernmentID(ctx context.Context, inquiryID string, kind string, frontURL string, backURL string) error
}

// UserClient est l'interface pour communiquer avec user-service via gRPC.
// Utilisé pour résoudre le Firebase UID en UserID interne MongoDB, car
// le kyc-service reçoit un Firebase UID depuis l'api-gateway mais le
// file-service stocke les documents avec l'UserID interne.
type UserClient interface {
	// GetUserIDByFirebaseID résout un Firebase UID en UserID interne MongoDB.
	GetUserIDByFirebaseID(ctx context.Context, firebaseUID string) (string, error)

	// Ferme la connexion gRPC
	Close() error
}
