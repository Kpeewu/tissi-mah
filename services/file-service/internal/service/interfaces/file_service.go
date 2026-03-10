package interfaces

import (
	"context"
	"io"

	"github.com/Kpeewu/tissi-mah/services/file-service/internal/domain"
)

// UploadUserDocumentInput contient les données nécessaires à l'upload d'un document utilisateur
type UploadUserDocumentInput struct {
	UserID         string
	DocumentName   string
	DocumentType   string
	MimeType       string
	FileSizeBytes  int64
	Data           io.Reader
	DocumentNumber string
	IssuingCountry string
}

// UploadVehicleDocumentInput contient les données nécessaires à l'upload d'un document véhicule
type UploadVehicleDocumentInput struct {
	VehicleID        string
	DocumentName     string
	DocumentType     string
	MimeType         string
	FileSizeBytes    int64
	Data             io.Reader
	DocumentNumber   string
	IssuingAuthority string
}

// GetDocumentInput contient les paramètres pour récupérer un document.
// UserID ou SupportID doit être fourni (pas les deux à la fois).
type GetDocumentInput struct {
	FileID    string
	UserID    string // Optionnel — l'utilisateur doit être propriétaire du document
	SupportID string // Optionnel — accès support sans vérification de propriété
}

// GetDocumentResult contient le document récupéré.
type GetDocumentResult struct {
	FileID   string
	FileURL  string
	FileType string
}

// DeleteFileInput contient les données pour supprimer un fichier avec vérification de propriété.
type DeleteFileInput struct {
	UserID string
	FileID string
}

// ChangeDocumentInput contient les données pour remplacer le fichier d'un document existant.
type ChangeDocumentInput struct {
	UserID      string
	FileID      string
	NewDocument []byte
}

// UploadVehicleDocumentsInput contient les documents du véhicule à uploader.
// Tous les champs sont obligatoires.
type UploadVehicleDocumentsInput struct {
	UserID              string
	VehicleID           string
	DriverLicenceImage  []byte
	Assurance           []byte
	VehicleRegistration []byte
}

// UploadIdDocumentInput contient les fichiers d'identité à uploader.
// Les champs requis dépendent du DocumentType :
//   - IDCard       : IDCardRecto + IDCardVerso
//   - Passport     : Passport
//   - DriverLicence: DriverLicenceRecto + DriverLicenceVerso
type UploadIdDocumentInput struct {
	UserID             string
	DocumentType       string // IDCard | Passport | DriverLicence
	IDCardRecto        []byte
	IDCardVerso        []byte
	DriverLicenceRecto []byte
	DriverLicenceVerso []byte
	Passport           []byte
}

// CreateReviewInput contient les données nécessaires à la création d'une revue
type CreateReviewInput struct {
	UserDocumentID    string
	VehicleDocumentID string
	Decision          string
	ReasonRejection   string
	RejectionDetails  string
	ReviewedBy        string
	ReviewedByType    string
	Notes             string
	ExtractedData     []byte
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

	// Supprime un document véhicule (S3 + DB)
	DeleteVehicleDocument(ctx context.Context, documentID string) error

	// --- Remplacement de document ---

	// Remplace le fichier d'un document utilisateur existant par un nouveau.
	ChangeDocument(ctx context.Context, input ChangeDocumentInput) error

	// --- Upload identité ---

	// Upload les documents d'identité vers S3/MinIO et sauvegarde les URLs en base.
	// Les fichiers fournis sont uploadés individuellement (un par type de pièce).
	UploadIdDocument(ctx context.Context, input UploadIdDocumentInput) error

	// --- Upload documents véhicule ---

	// Upload les documents du véhicule (permis, assurance, carte grise) vers S3/MinIO
	// et sauvegarde les URLs en base.
	UploadVehicleDocuments(ctx context.Context, input UploadVehicleDocumentsInput) error

	// --- Revues ---

	// Crée une revue de document
	CreateDocumentReview(ctx context.Context, input CreateReviewInput) (*domain.DocumentReview, error)

	// Récupère les revues d'un document (user ou vehicle)
	GetDocumentReviews(ctx context.Context, userDocumentID string, vehicleDocumentID string) ([]*domain.DocumentReview, error)
}
