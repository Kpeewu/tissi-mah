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

// UploadIdDocumentInput contient les fichiers d'identité à uploader.
// Les champs requis dépendent du DocumentType :
//   - IDCard       : IDCardRecto + IDCardVerso
//   - Passport     : Passport
//   - DriverLicence: DriverLicenceRecto + DriverLicenceVerso
type UploadIdDocumentInput struct {
	ProfileID          string
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

	// --- Upload identité ---

	// Upload les documents d'identité vers S3/MinIO et sauvegarde les URLs en base.
	// Les fichiers fournis sont uploadés individuellement (un par type de pièce).
	UploadIdDocument(ctx context.Context, input UploadIdDocumentInput) error

	// --- Revues ---

	// Crée une revue de document
	CreateDocumentReview(ctx context.Context, input CreateReviewInput) (*domain.DocumentReview, error)

	// Récupère les revues d'un document (user ou vehicle)
	GetDocumentReviews(ctx context.Context, userDocumentID string, vehicleDocumentID string) ([]*domain.DocumentReview, error)
}
