package interfaces

import (
	"context"
	"io"
	"time"

	"github.com/Kpeewu/tissi-mah/services/file-service/internal/domain"
)

// KycDocument est une vue unifiée d'un document KYC (user ou véhicule) pour la file
// de validation manuelle. OwnerKind vaut "user" ou "vehicle" ; pour un document
// véhicule, VehicleID est renseigné (UserID reste le propriétaire dénormalisé).
type KycDocument struct {
	DocumentID   string
	UserID       string
	VehicleID    string // vide pour un document utilisateur
	DocumentType string
	Status       string
	OwnerKind    string // "user" | "vehicle"
	UpdatedAt    string // ISO 8601
	UploadedAt   string // ISO 8601 — date de dépôt initial
}

// UploadUserDocumentInput contient les données nécessaires à l'upload d'un document utilisateur
type UploadUserDocumentInput struct {
	UserID         string
	DocumentName   string
	DocumentType   string
	MimeType       string
	FileSizeBytes  int64
	Data           io.Reader
	DocumentNumber string
	IssuedAt       *time.Time
	ExpireAt       *time.Time
	IssuingCountry string
}

// UploadVehicleDocumentInput contient les données nécessaires à l'upload d'un document véhicule.
// UserID est dénormalisé sur vehicle_documents pour permettre la jointure inverse
// dans GetDocumentReviewsByUserID (file-service n'a pas accès à la table vehicles
// du user-service).
type UploadVehicleDocumentInput struct {
	UserID           string
	VehicleID        string
	DocumentName     string
	DocumentType     string
	MimeType         string
	FileSizeBytes    int64
	Data             io.Reader
	DocumentNumber   string
	IssuedAt         *time.Time
	ExpireAt         *time.Time
	IssuingAuthority string
}

// GetDocumentInput contient les paramètres pour récupérer un document.
// UserID ou SupportID doit être fourni (pas les deux à la fois).
type GetDocumentInput struct {
	FileID    string
	UserID    string // Optionnel — l'utilisateur doit être propriétaire du document
	SupportID string // Optionnel — accès support sans vérification de propriété
}

// GetDocumentResult contient le document récupéré avec une URL présignée.
type GetDocumentResult struct {
	FileID                string
	FileURL               string // URL présignée (30 min)
	FileType              string
	PresignedURLExpiresAt string // ISO 8601
}

// DeleteFileInput contient les données pour supprimer un fichier avec vérification de propriété.
type DeleteFileInput struct {
	UserID string
	FileID string
}

// ChangeDocumentInput contient les données pour remplacer le fichier d'un document existant.
type ChangeDocumentInput struct {
	UserID         string
	FileID         string
	NewDocument    []byte
	DocumentNumber string // vide = conserver l'existant
	IssuedAt       string // ISO 8601 — vide = conserver
	ExpireAt       string // ISO 8601 — vide = conserver
	IssuingPlace   string // vide = conserver (country ou authority selon le type)
}

// VehicleDocFileInput regroupe le fichier et les métadonnées légales d'un document véhicule.
type VehicleDocFileInput struct {
	Data             []byte
	DocumentNumber   string
	IssuedAt         string // ISO 8601
	ExpireAt         string // ISO 8601 — vide si registrationCard (optionnel)
	IssuingAuthority string
}

// DriverLicenceFileInput regroupe les deux faces du permis et ses métadonnées
// légales (partagées recto/verso).
type DriverLicenceFileInput struct {
	Recto            []byte
	Verso            []byte
	DocumentNumber   string
	IssuedAt         string // ISO 8601
	ExpireAt         string // ISO 8601
	IssuingAuthority string
}

// UploadVehicleDocumentsInput contient les documents du véhicule à uploader.
// UserID, VehicleID, Assurance et RegistrationCard sont obligatoires.
// DriverLicence est optionnel si l'utilisateur possède déjà un permis courant
// (documents utilisateur driverLicenceFront/driverLicenceBack) — les images sont
// alors ignorées ; sinon recto + verso sont obligatoires et stockés comme
// documents UTILISATEUR (pas véhicule).
// FirstName / LastName viennent de user-service et servent à construire le docName.
type UploadVehicleDocumentsInput struct {
	UserID           string
	VehicleID        string
	FirstName        string
	LastName         string
	DriverLicence    DriverLicenceFileInput
	Assurance        VehicleDocFileInput
	RegistrationCard VehicleDocFileInput
}

// UploadIdDocumentInput contient les fichiers d'identité à uploader.
// Les champs requis dépendent du DocumentType :
//   - IDCard       : IDCardRecto + IDCardVerso
//   - Passport     : Passport
//   - DriverLicence: DriverLicenceRecto + DriverLicenceVerso (document partagé identité/véhicule)
//
// FirstName / LastName viennent de user-service et servent à construire le docName
// au format {nom}_{prenom}_{YYYYMMDD}_{HHMMSS}_{type}.
// DocumentNumber, IssuedAt, ExpireAt, IssuingCountry sont obligatoires pour tous les types.
type UploadIdDocumentInput struct {
	UserID             string
	FirstName          string
	LastName           string
	DocumentType       string // IDCard | Passport | DriverLicence
	IDCardRecto        []byte
	IDCardVerso        []byte
	DriverLicenceRecto []byte
	DriverLicenceVerso []byte
	Passport           []byte
	DocumentNumber     string
	IssuedAt           string // ISO 8601
	ExpireAt           string // ISO 8601
	IssuingCountry     string
}

// UploadedDocument décrit un document fraîchement uploadé/remplacé,
// retourné aux handlers pour que le front affiche l'ID + l'URL S3.
type UploadedDocument struct {
	DocumentID   string
	DocumentURL  string
	DocumentType string
	DocumentName string
}

// CreateReviewInput contient les données nécessaires à la création d'une revue
type CreateReviewInput struct {
	UserID               string
	DocumentType         string
	UserDocumentID       string
	SecondUserDocumentID string // verso pour les documents recto-verso
	VehicleDocumentID    string

	// Persona
	PersonaInquiryID    string
	PersonaTemplateID   string
	PersonaSessionToken string
	SessionExpiresAt    string // ISO 8601

	// Webhook
	WebhookEventType  string
	WebhookReceivedAt string // ISO 8601
	PersonaRawPayload []byte // JSON

	// Retry / versioning
	AttemptNumber    int32
	PreviousReviewID string

	// Décision
	Status           string
	Decision         string
	ReasonRejection  string
	RejectionDetails string

	// Réviseur
	ReviewedBy string
	ReviewType string

	Notes         string
	ExtractedData []byte // JSON

	// Timestamps
	SubmittedAt string // ISO 8601
}

type FileService interface {
	// --- Documents utilisateur ---

	// Upload un document utilisateur vers S3 et sauvegarde les métadonnées
	UploadUserDocument(ctx context.Context, input UploadUserDocumentInput) (*domain.UserDocument, error)

	// Récupère tous les documents d'un utilisateur
	GetUserDocuments(ctx context.Context, userID string) ([]*domain.UserDocument, error)

	// Récupère un document par son ID
	GetUserDocument(ctx context.Context, documentID string) (*domain.UserDocument, error)

	// Récupère le document courant d'un utilisateur par type
	GetCurrentUserDocument(ctx context.Context, userID string, documentType string) (*domain.UserDocument, error)

	// Récupère un document par FileID avec contrôle d'accès (propriétaire ou support)
	GetDocument(ctx context.Context, input GetDocumentInput) (*GetDocumentResult, error)

	// Supprime un fichier après vérification que UserID est bien propriétaire (S3 + DB)
	DeleteFile(ctx context.Context, input DeleteFileInput) error

	// Supprime un document utilisateur (S3 + DB)
	DeleteUserDocument(ctx context.Context, documentID string) error

	// --- Documents véhicule ---

	// Upload un document véhicule vers S3 et sauvegarde les métadonnées
	UploadVehicleDocument(ctx context.Context, input UploadVehicleDocumentInput) (*domain.VehicleDocument, error)

	// Récupère tous les documents d'un véhicule
	GetVehicleDocuments(ctx context.Context, vehicleID string) ([]*domain.VehicleDocument, error)

	// Récupère un document véhicule par son ID
	GetVehicleDocument(ctx context.Context, documentID string) (*domain.VehicleDocument, error)

	// Récupère tous les documents véhicule d'un utilisateur (user_id dénormalisé)
	GetVehicleDocumentsByUserID(ctx context.Context, userID string) ([]*domain.VehicleDocument, error)

	// Supprime un document véhicule (S3 + DB)
	DeleteVehicleDocument(ctx context.Context, documentID string) error

	// --- Validation manuelle (support) ---

	// ListKycDocuments liste les documents KYC courants (user + vehicle) filtrés par
	// statut, pour la file de validation manuelle. statuses vide = tous les statuts.
	ListKycDocuments(ctx context.Context, statuses []string) ([]*KycDocument, error)

	// --- Remplacement de document ---

	// Remplace le fichier d'un document utilisateur existant par un nouveau.
	// Retourne le document fraîchement créé (ID + URL S3).
	ChangeDocument(ctx context.Context, input ChangeDocumentInput) (*UploadedDocument, error)

	// --- Upload identité ---

	// Upload les documents d'identité vers S3/MinIO et sauvegarde les URLs en base.
	// Les fichiers fournis sont uploadés individuellement (un par type de pièce).
	// Retourne la liste des documents créés (1 pour Passport, 2 pour IDCard / DriverLicence).
	UploadIdDocument(ctx context.Context, input UploadIdDocumentInput) ([]*UploadedDocument, error)

	// --- Upload documents véhicule ---

	// Upload les documents du véhicule (permis, assurance, carte grise) vers S3/MinIO
	// et sauvegarde les URLs en base. Retourne la liste des documents créés.
	UploadVehicleDocuments(ctx context.Context, input UploadVehicleDocumentsInput) ([]*UploadedDocument, error)

	// --- Revues ---

	// Crée une revue de document
	CreateDocumentReview(ctx context.Context, input CreateReviewInput) (*domain.DocumentReview, error)

	// Récupère une revue par son ID
	GetDocumentReview(ctx context.Context, reviewID string) (*domain.DocumentReview, error)

	// Récupère les revues d'un document (user ou vehicle)
	GetDocumentReviews(ctx context.Context, userDocumentID string, vehicleDocumentID string) ([]*domain.DocumentReview, error)

	// Récupère une revue par persona_inquiry_id
	GetDocumentReviewByPersonaInquiryID(ctx context.Context, personaInquiryID string) (*domain.DocumentReview, error)

	// Récupère toutes les revues liées aux documents d'un utilisateur
	GetDocumentReviewsByUserID(ctx context.Context, userID string) ([]*domain.DocumentReview, error)

	// Met à jour une revue existante
	UpdateDocumentReview(ctx context.Context, review *domain.DocumentReview) (*domain.DocumentReview, error)

	// Liste les revues avec filtres et pagination
	ListDocumentReviews(ctx context.Context, userID string, status string, decision string, page int32, pageSize int32) ([]*domain.DocumentReview, error)

	// Retourne l'historique complet d'un document logique pour un utilisateur (trié par date décroissante)
	GetDocumentReviewHistory(ctx context.Context, userID string, logicalDocumentType string) ([]*domain.DocumentReview, error)

	// --- Suppression de compte ---

	// DeleteAllUserFiles supprime tous les documents (user + vehicle) d'un utilisateur
	// ainsi que les fichiers correspondants dans S3/MinIO.
	DeleteAllUserFiles(ctx context.Context, userID string) error
}
