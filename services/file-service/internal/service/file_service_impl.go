package service

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Kpeewu/tissi-mah/services/file-service/internal/domain"
	repoInterfaces "github.com/Kpeewu/tissi-mah/services/file-service/internal/repository/interfaces"
	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/file-service/internal/service/interfaces"
	"github.com/Kpeewu/tissi-mah/services/file-service/internal/storage"
	fileErrors "github.com/Kpeewu/tissi-mah/services/file-service/pkg/errors"
)

// Types MIME autorisés
var allowedMimeTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
	"application/pdf": true,
}

// Taille maximale : 10 Mo
const maxFileSize int64 = 10 * 1024 * 1024

type fileServiceImpl struct {
	userDocRead     repoInterfaces.UserDocumentRepositoryRead
	userDocWrite    repoInterfaces.UserDocumentRepositoryWrite
	vehicleDocRead  repoInterfaces.VehicleDocumentRepositoryRead
	vehicleDocWrite repoInterfaces.VehicleDocumentRepositoryWrite
	reviewRead      repoInterfaces.DocumentReviewRepositoryRead
	reviewWrite     repoInterfaces.DocumentReviewRepositoryWrite
	storage         storage.StorageClient
}

func NewFileService(
	userDocRead repoInterfaces.UserDocumentRepositoryRead,
	userDocWrite repoInterfaces.UserDocumentRepositoryWrite,
	vehicleDocRead repoInterfaces.VehicleDocumentRepositoryRead,
	vehicleDocWrite repoInterfaces.VehicleDocumentRepositoryWrite,
	reviewRead repoInterfaces.DocumentReviewRepositoryRead,
	reviewWrite repoInterfaces.DocumentReviewRepositoryWrite,
	storageClient storage.StorageClient,
) serviceInterfaces.FileService {
	return &fileServiceImpl{
		userDocRead:     userDocRead,
		userDocWrite:    userDocWrite,
		vehicleDocRead:  vehicleDocRead,
		vehicleDocWrite: vehicleDocWrite,
		reviewRead:      reviewRead,
		reviewWrite:     reviewWrite,
		storage:         storageClient,
	}
}

// --- Documents utilisateur ---

func (s *fileServiceImpl) UploadUserDocument(ctx context.Context, input serviceInterfaces.UploadUserDocumentInput) (*domain.UserDocument, error) {
	// Validation du type de document
	if !domain.IsValidUserDocumentType(input.DocumentType) {
		return nil, fileErrors.ErrorInvalidDocumentType
	}

	// Validation du type MIME
	if !allowedMimeTypes[input.MimeType] {
		return nil, fileErrors.ErrorInvalidMimeType
	}

	// Validation de la taille
	if input.FileSizeBytes > maxFileSize {
		return nil, fileErrors.ErrorFileTooLarge
	}

	documentID := uuid.New().String()
	ext := extensionFromMimeType(input.MimeType)
	s3Key := fmt.Sprintf("%s/%s/%s%s", input.DocumentType, input.UserID, documentID, ext)

	// Upload vers S3/MinIO
	documentURL, err := s.storage.Upload(ctx, s3Key, input.Data, input.MimeType, input.FileSizeBytes)
	if err != nil {
		return nil, fileErrors.ErrorUploadFailed
	}

	// Marquer l'ancien document courant comme remplacé
	existing, err := s.userDocRead.GetCurrentByUserIDAndType(ctx, input.UserID, input.DocumentType)
	if err == nil && existing != nil {
		_ = s.userDocWrite.MarkAsReplaced(ctx, existing.DocumentID, documentID)
	}

	now := time.Now().UTC()
	doc := &domain.UserDocument{
		DocumentID:     documentID,
		UserID:         input.UserID,
		DocumentName:   input.DocumentName,
		DocumentType:   input.DocumentType,
		DocumentURL:    documentURL,
		FileSizeBytes:  input.FileSizeBytes,
		MimeType:       input.MimeType,
		DocumentNumber: input.DocumentNumber,
		IssuingCountry: input.IssuingCountry,
		Status:         "pending",
		IsCurrent:      true,
		UploadedAt:     now,
		UpdatedAt:      now,
	}

	_, err = s.userDocWrite.Create(ctx, doc)
	if err != nil {
		return nil, fileErrors.ErrorInternalServer
	}

	return doc, nil
}

func (s *fileServiceImpl) GetUserDocuments(ctx context.Context, userID string) ([]*domain.UserDocument, error) {
	return s.userDocRead.GetByUserID(ctx, userID)
}

func (s *fileServiceImpl) GetUserDocument(ctx context.Context, documentID string) (*domain.UserDocument, error) {
	return s.userDocRead.GetByID(ctx, documentID)
}

func (s *fileServiceImpl) GetCurrentUserDocument(ctx context.Context, userID string, documentType string) (*domain.UserDocument, error) {
	if !domain.IsValidUserDocumentType(documentType) {
		return nil, fileErrors.ErrorInvalidDocumentType
	}
	return s.userDocRead.GetCurrentByUserIDAndType(ctx, userID, documentType)
}

func (s *fileServiceImpl) DeleteUserDocument(ctx context.Context, documentID string) error {
	doc, err := s.userDocRead.GetByID(ctx, documentID)
	if err != nil {
		return err
	}

	// Extraire la clé S3 depuis l'URL
	s3Key := s3KeyFromURL(doc.DocumentURL, doc.DocumentType, doc.UserID, doc.DocumentID)
	_ = s.storage.Delete(ctx, s3Key)

	return s.userDocWrite.Delete(ctx, documentID)
}

// --- Documents véhicule ---

func (s *fileServiceImpl) UploadVehicleDocument(ctx context.Context, input serviceInterfaces.UploadVehicleDocumentInput) (*domain.VehicleDocument, error) {
	if !domain.IsValidVehicleDocumentType(input.DocumentType) {
		return nil, fileErrors.ErrorInvalidDocumentType
	}

	if !allowedMimeTypes[input.MimeType] {
		return nil, fileErrors.ErrorInvalidMimeType
	}

	if input.FileSizeBytes > maxFileSize {
		return nil, fileErrors.ErrorFileTooLarge
	}

	documentID := uuid.New().String()
	ext := extensionFromMimeType(input.MimeType)
	s3Key := fmt.Sprintf("%s/%s/%s%s", input.DocumentType, input.VehicleID, documentID, ext)

	documentURL, err := s.storage.Upload(ctx, s3Key, input.Data, input.MimeType, input.FileSizeBytes)
	if err != nil {
		return nil, fileErrors.ErrorUploadFailed
	}

	now := time.Now().UTC()
	doc := &domain.VehicleDocument{
		DocumentID:       documentID,
		VehicleID:        input.VehicleID,
		DocumentName:     input.DocumentName,
		DocumentType:     input.DocumentType,
		DocumentURL:      documentURL,
		FileSizeBytes:    input.FileSizeBytes,
		MimeType:         input.MimeType,
		DocumentNumber:   input.DocumentNumber,
		IssuingAuthority: input.IssuingAuthority,
		Status:           "pending",
		IsCurrent:        true,
		UploadedAt:       now,
		UpdatedAt:        now,
	}

	_, err = s.vehicleDocWrite.Create(ctx, doc)
	if err != nil {
		return nil, fileErrors.ErrorInternalServer
	}

	return doc, nil
}

func (s *fileServiceImpl) GetVehicleDocuments(ctx context.Context, vehicleID string) ([]*domain.VehicleDocument, error) {
	return s.vehicleDocRead.GetByVehicleID(ctx, vehicleID)
}

func (s *fileServiceImpl) GetVehicleDocument(ctx context.Context, documentID string) (*domain.VehicleDocument, error) {
	return s.vehicleDocRead.GetByID(ctx, documentID)
}

func (s *fileServiceImpl) DeleteVehicleDocument(ctx context.Context, documentID string) error {
	doc, err := s.vehicleDocRead.GetByID(ctx, documentID)
	if err != nil {
		return err
	}

	s3Key := s3KeyFromURL(doc.DocumentURL, doc.DocumentType, doc.VehicleID, doc.DocumentID)
	_ = s.storage.Delete(ctx, s3Key)

	return s.vehicleDocWrite.Delete(ctx, documentID)
}

// --- Revues ---

func (s *fileServiceImpl) CreateDocumentReview(ctx context.Context, input serviceInterfaces.CreateReviewInput) (*domain.DocumentReview, error) {
	if !domain.IsValidReviewDecision(input.Decision) {
		return nil, fileErrors.ErrorInvalidReviewDecision
	}
	if !domain.IsValidReviewerType(input.ReviewedByType) {
		return nil, fileErrors.ErrorInternalServer
	}

	reviewID := uuid.New().String()

	var userDocID *string
	var vehicleDocID *string

	if input.UserDocumentID != "" {
		userDocID = &input.UserDocumentID
		// Vérifier que le document existe
		_, err := s.userDocRead.GetByID(ctx, input.UserDocumentID)
		if err != nil {
			return nil, err
		}
	}
	if input.VehicleDocumentID != "" {
		vehicleDocID = &input.VehicleDocumentID
		_, err := s.vehicleDocRead.GetByID(ctx, input.VehicleDocumentID)
		if err != nil {
			return nil, err
		}
	}

	var extractedData json.RawMessage
	if len(input.ExtractedData) > 0 {
		extractedData = input.ExtractedData
	}

	review := &domain.DocumentReview{
		ReviewID:          reviewID,
		UserDocumentID:    userDocID,
		VehicleDocumentID: vehicleDocID,
		Decision:          input.Decision,
		ReasonRejection:   input.ReasonRejection,
		RejectionDetails:  input.RejectionDetails,
		ReviewedBy:        input.ReviewedBy,
		ReviewedByType:    input.ReviewedByType,
		ReviewedAt:        time.Now().UTC(),
		Notes:             input.Notes,
		ExtractedData:     extractedData,
	}

	_, err := s.reviewWrite.Create(ctx, review)
	if err != nil {
		return nil, fileErrors.ErrorInternalServer
	}

	// Mettre à jour le statut du document selon la décision
	newStatus := mapDecisionToStatus(input.Decision)
	if input.UserDocumentID != "" {
		doc, _ := s.userDocRead.GetByID(ctx, input.UserDocumentID)
		if doc != nil {
			doc.Status = newStatus
			_, _ = s.userDocWrite.Update(ctx, doc)
		}
	}
	if input.VehicleDocumentID != "" {
		doc, _ := s.vehicleDocRead.GetByID(ctx, input.VehicleDocumentID)
		if doc != nil {
			doc.Status = newStatus
			_, _ = s.vehicleDocWrite.Update(ctx, doc)
		}
	}

	return review, nil
}

func (s *fileServiceImpl) GetDocumentReviews(ctx context.Context, userDocumentID string, vehicleDocumentID string) ([]*domain.DocumentReview, error) {
	if userDocumentID != "" {
		return s.reviewRead.GetByUserDocumentID(ctx, userDocumentID)
	}
	if vehicleDocumentID != "" {
		return s.reviewRead.GetByVehicleDocumentID(ctx, vehicleDocumentID)
	}
	return nil, fileErrors.ErrorDocumentNotFound
}

// --- Helpers ---

func extensionFromMimeType(mimeType string) string {
	switch mimeType {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "application/pdf":
		return ".pdf"
	default:
		return ""
	}
}

func s3KeyFromURL(documentURL string, documentType string, ownerID string, documentID string) string {
	ext := filepath.Ext(documentURL)
	if ext == "" {
		ext = ".jpg"
	}
	// Enlever le point initial si nécessaire pour reconstruire
	ext = strings.TrimPrefix(ext, "?")
	return fmt.Sprintf("%s/%s/%s%s", documentType, ownerID, documentID, ext)
}

func mapDecisionToStatus(decision string) string {
	switch decision {
	case "approved":
		return "approved"
	case "rejected":
		return "rejected"
	case "resubmission":
		return "rejected"
	default:
		return "pending"
	}
}
