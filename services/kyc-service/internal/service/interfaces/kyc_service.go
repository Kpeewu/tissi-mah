package interfaces

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/kyc-service/internal/domain"
)

// CreateInquiryInput contient les données pour démarrer une vérification KYC.
// DocumentID identifie précisément le document à vérifier (retourné par
// file-service lors de l'upload). VehicleID, si renseigné, indique qu'il s'agit
// d'un document véhicule (sinon document utilisateur).
type CreateInquiryInput struct {
	UserID       string
	DocumentID   string
	DocumentType string
	VehicleID    string // Optionnel : si renseigné, document véhicule
}

// CreateInquiryResult contient la réponse de création d'une inquiry
type CreateInquiryResult struct {
	ReviewID          string
	PersonaInquiryID  string
	PersonaTemplateID string
	SessionToken      string
	SessionExpiresAt  string // ISO 8601
	Status            string
	AttemptNumber     int32
	CreatedAt         string // ISO 8601
}

// InquiryDetail contient le détail d'une inquiry
type InquiryDetail struct {
	ReviewID          string
	PersonaInquiryID  string
	UserDocumentID    string
	PersonaTemplateID string
	VehicleDocumentID string
	Status            string
	Decision          string
	ReasonRejection   string
	RejectionDetails  string
	AttemptNumber     int32
	PreviousReviewID  string
	ReviewType        string
	ReviewedAt        string // ISO 8601
	CreatedAt         string // ISO 8601
	UpdatedAt         string // ISO 8601
}

// KYCStatus contient le statut KYC global d'un utilisateur
type KYCStatus struct {
	IdentityVerified bool
	DriverVerified   bool
	PendingReviews   []*domain.PendingReview
	LatestRejection  *domain.LatestRejection
}

// ResumeResult contient la réponse de reprise d'une inquiry
type ResumeResult struct {
	ReviewID         string
	PersonaInquiryID string
	SessionToken     string
	SessionExpiresAt string // ISO 8601
	Status           string
	AttemptNumber    int32
}

// WebhookInput contient les données du webhook Persona
type WebhookInput struct {
	Signature         string
	PersonaInquiryID  string
	WebhookEventType  string
	OccurredAt        string // ISO 8601
	PersonaRawPayload []byte // JSON
}

// GetAdminReviewsInput contient les filtres pour la liste admin des revues
type GetAdminReviewsInput struct {
	UserID   string
	Status   string // Optionnel
	Decision string // Optionnel
	Index    int32  // Numéro de page
}

// AdminReviewItem est une vue allégée pour la liste admin (sans persona_raw_payload ni extracted_data)
type AdminReviewItem struct {
	ReviewID          string
	PersonaInquiryID  string
	UserDocumentID    string
	VehicleDocumentID string
	Status            string
	Decision          string
	ReasonRejection   string
	RejectionDetails  string
	ReviewType        string
	ReviewedAt        string // ISO 8601
	AttemptNumber     int32
	PreviousReviewID  string
	WebhookEventType  string
	WebhookReceivedAt string // ISO 8601
	CreatedAt         string // ISO 8601
	UpdatedAt         string // ISO 8601
}

// AdminReviewDetail contient le détail complet d'une revue pour le support
type AdminReviewDetail struct {
	ReviewID          string
	PersonaInquiryID  string
	PersonaTemplateID string
	UserDocumentID    string
	VehicleDocumentID string
	Status            string
	Decision          string
	ReasonRejection   string
	RejectionDetails  string
	ReviewedBy        string
	ReviewType        string
	ReviewedAt        string // ISO 8601
	Notes             string
	ExtractedData     []byte // JSON
	WebhookEventType  string
	WebhookReceivedAt string // ISO 8601
	AttemptNumber     int32
	PreviousReviewID  string
	SessionExpiresAt  string // ISO 8601
	CreatedAt         string // ISO 8601
	UpdatedAt         string // ISO 8601
}

// OverrideReviewInput contient les données pour overrider une revue
type OverrideReviewInput struct {
	UserID           string
	ReviewID         string
	Decision         string
	ReasonRejection  string
	RejectionDetails string
	Notes            string
}

// OverrideResult contient la réponse d'un override
type OverrideResult struct {
	ReviewID         string
	PersonaInquiryID string
	Decision         string
	ReasonRejection  string
	RejectionDetails string
	ReviewedBy       string
	ReviewType       string
	ReviewedAt       string // ISO 8601
	Notes            string
	UpdatedAt        string // ISO 8601
}

// ValidateDocumentInput contient les données pour valider manuellement un document
type ValidateDocumentInput struct {
	SupportAgentID   string
	DocumentID       string
	VehicleID        string // Optionnel — si renseigné, document véhicule
	Decision         string // approved | rejected | resubmission
	ReasonRejection  string // Requis si Decision = "rejected"
	RejectionDetails string
	Notes            string
}

// ValidateDocumentResult contient la réponse d'une validation manuelle
type ValidateDocumentResult struct {
	ReviewID   string
	Decision   string
	ReviewedBy string
	ReviewType string
	ReviewedAt string // ISO 8601
	Notes      string
}

// GetManualReviewRequestsInput contient les filtres de la liste groupée par utilisateur.
type GetManualReviewRequestsInput struct {
	Status      string // Optionnel — ne garder que les users dont une catégorie a ce statut
	Page        int32  // 0-based
	PageSize    int32  // 0 = défaut
	Name        string // Optionnel — recherche partielle (contient, insensible casse) sur le nom
	FirstName   string // Optionnel — recherche partielle (contient, insensible casse) sur le prénom
	DepositFrom string // Optionnel — borne basse date dernier dépôt (ISO 8601 ou YYYY-MM-DD)
	DepositTo   string // Optionnel — borne haute date dernier dépôt (ISO 8601 ou YYYY-MM-DD)
}

// GetManualReviewRequestsResult : page de la liste groupée par utilisateur.
type GetManualReviewRequestsResult struct {
	Requests []*domain.ManualReviewRequest
	Total    int32
}

// KYCService définit les opérations du service KYC
type KYCService interface {
	// Démarre une nouvelle vérification d'identité
	CreateInquiry(ctx context.Context, input CreateInquiryInput) (*CreateInquiryResult, error)

	// Récupère le détail d'une inquiry
	GetInquiry(ctx context.Context, userID string, personaInquiryID string) (*InquiryDetail, error)

	// Récupère le statut KYC global d'un utilisateur
	GetKYCStatus(ctx context.Context, userID string) (*KYCStatus, error)

	// Reprend une session de vérification interrompue
	ResumeInquiry(ctx context.Context, userID string, personaInquiryID string) (*ResumeResult, error)

	// Traite un webhook Persona (validation HMAC + publication Redis Streams)
	ProcessWebhook(ctx context.Context, input WebhookInput) error

	// Liste les revues pour l'admin (avec filtres et pagination)
	GetAdminReviews(ctx context.Context, input GetAdminReviewsInput) ([]*AdminReviewItem, error)

	// Récupère le détail complet d'une revue pour l'admin
	GetAdminReview(ctx context.Context, userID string, reviewID string) (*AdminReviewDetail, error)

	// Override manuel d'une revue par un agent de support
	OverrideReview(ctx context.Context, input OverrideReviewInput) (*OverrideResult, error)

	// Validation manuelle directe d'un document par un agent support (sans Persona)
	ValidateDocument(ctx context.Context, input ValidateDocumentInput) (*ValidateDocumentResult, error)

	// GetManualReviewRequests liste, groupées par utilisateur, les demandes de
	// validation (statut par catégorie passenger/driver). Tous statuts retournés.
	GetManualReviewRequests(ctx context.Context, input GetManualReviewRequestsInput) (*GetManualReviewRequestsResult, error)

	// GetManualReviewRequestDetail retourne tous les documents soumis d'un utilisateur
	// avec leur statut et la dernière review associée.
	GetManualReviewRequestDetail(ctx context.Context, userID string) (*domain.ManualReviewRequestDetail, error)

	// GetDocumentHistory retourne l'historique chronologique complet des revues
	// pour un type logique de document (idCard, driverLicence, etc.).
	GetDocumentHistory(ctx context.Context, userID string, logicalDocumentType string) ([]*domain.DocumentHistoryEntry, error)
}
