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

	// --- Validation manuelle (support) ---

	// ListKycDocuments liste les documents KYC courants (user + vehicle) filtrés par statut.
	ListKycDocuments(ctx context.Context, statuses []string) ([]*domain.KycDocument, error)

	// GetUserDocumentSummaries récupère tous les documents utilisateur avec leur statut.
	GetUserDocumentSummaries(ctx context.Context, userID string) ([]*domain.DocumentSummary, error)

	// GetVehicleDocumentSummariesByUserID récupère tous les documents véhicule d'un
	// utilisateur (user_id dénormalisé) avec leur statut.
	GetVehicleDocumentSummariesByUserID(ctx context.Context, userID string) ([]*domain.DocumentSummary, error)

	// Crée une revue de document dans le file-service
	CreateDocumentReview(ctx context.Context, review *domain.Review) (*domain.Review, error)

	// Récupère une revue par son ID
	GetDocumentReview(ctx context.Context, reviewID string) (*domain.Review, error)

	// Récupère toutes les revues d'un utilisateur (via ses documents)
	GetDocumentReviewsByUserID(ctx context.Context, userID string) ([]*domain.Review, error)

	// Met à jour une revue existante
	UpdateDocumentReview(ctx context.Context, review *domain.Review) (*domain.Review, error)

	// Liste les revues avec filtres et pagination
	ListDocumentReviews(ctx context.Context, userID string, status string, decision string, page int32, pageSize int32) ([]*domain.Review, error)

	// GetDocumentReviewHistory retourne l'historique complet des revues pour un type logique
	GetDocumentReviewHistory(ctx context.Context, userID string, logicalDocumentType string) ([]*domain.Review, error)

	// Ferme la connexion gRPC
	Close() error
}

// UserClient est l'interface pour communiquer avec user-service via gRPC.
// Utilisé pour résoudre le Firebase UID en UserID interne MongoDB, car
// le kyc-service reçoit un Firebase UID depuis l'api-gateway mais le
// file-service stocke les documents avec l'UserID interne.
type UserClient interface {
	// GetUserIDByFirebaseID résout un Firebase UID en UserID interne MongoDB.
	GetUserIDByFirebaseID(ctx context.Context, firebaseUID string) (string, error)

	// GetUserByUserID récupère les infos profil par UserID interne (vue support).
	GetUserByUserID(ctx context.Context, userID string) (*domain.UserInfo, error)

	// GetUsersByUserIDs récupère en batch les infos profil (nom/prénom/photo) de
	// plusieurs utilisateurs. Léger : email/phone non renseignés. Clé = userID.
	GetUsersByUserIDs(ctx context.Context, userIDs []string) (map[string]*domain.UserInfo, error)

	// UpdateProfileVerification met à jour les flags de vérification KYC du profil
	// (passager / conducteur) après validation des documents par le support.
	// Retourne les valeurs précédentes des flags (détection des bascules false→true).
	UpdateProfileVerification(ctx context.Context, userID string, driver, passenger bool) (prevDriver, prevPassenger bool, err error)

	// Ferme la connexion gRPC
	Close() error
}

// VehicleClient est l'interface pour communiquer avec vehicle-service via gRPC.
// Utilisé pour pousser le flag is_verified d'un véhicule quand ses documents
// (assurance + carte grise) sont tous deux approuvés — ou cessent de l'être.
type VehicleClient interface {
	// SetVehicleVerification fixe le flag is_verified d'un véhicule.
	SetVehicleVerification(ctx context.Context, vehicleID string, isVerified bool) error

	// Ferme la connexion gRPC
	Close() error
}

// SupportClient est l'interface pour communiquer avec support-service via gRPC.
// Utilisé pour résoudre l'UID d'un agent support (stocké dans review.ReviewedBy)
// en prénom/nom affichables dans l'historique des reviews.
type SupportClient interface {
	// GetSupportUserByID récupère le prénom/nom/rôle d'un agent support par son UID.
	GetSupportUserByID(ctx context.Context, userID string) (*domain.SupportAgent, error)

	// Ferme la connexion gRPC
	Close() error
}
