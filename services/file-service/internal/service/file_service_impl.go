package service

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Kpeewu/tissi-mah/services/file-service/internal/domain"
	repoInterfaces "github.com/Kpeewu/tissi-mah/services/file-service/internal/repository/interfaces"
	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/file-service/internal/service/interfaces"
	"github.com/Kpeewu/tissi-mah/services/file-service/internal/storage"
	fileErrors "github.com/Kpeewu/tissi-mah/services/file-service/pkg/errors"
)

// Types MIME autorisés
var allowedMimeTypes = map[string]bool{
	"image/jpeg":      true,
	"image/png":       true,
	"image/webp":      true,
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
	logger          *zap.Logger
}

func NewFileService(
	userDocRead repoInterfaces.UserDocumentRepositoryRead,
	userDocWrite repoInterfaces.UserDocumentRepositoryWrite,
	vehicleDocRead repoInterfaces.VehicleDocumentRepositoryRead,
	vehicleDocWrite repoInterfaces.VehicleDocumentRepositoryWrite,
	reviewRead repoInterfaces.DocumentReviewRepositoryRead,
	reviewWrite repoInterfaces.DocumentReviewRepositoryWrite,
	storageClient storage.StorageClient,
	logger *zap.Logger,
) serviceInterfaces.FileService {
	return &fileServiceImpl{
		userDocRead:     userDocRead,
		userDocWrite:    userDocWrite,
		vehicleDocRead:  vehicleDocRead,
		vehicleDocWrite: vehicleDocWrite,
		reviewRead:      reviewRead,
		reviewWrite:     reviewWrite,
		storage:         storageClient,
		logger:          logger,
	}
}

// --- Documents utilisateur ---

func (s *fileServiceImpl) UploadUserDocument(ctx context.Context, input serviceInterfaces.UploadUserDocumentInput) (*domain.UserDocument, error) {
	s.logger.Debug("upload user document",
		zap.String("userID", input.UserID),
		zap.String("type", input.DocumentType),
		zap.String("mime", input.MimeType),
		zap.Int64("size", input.FileSizeBytes),
	)

	if !domain.IsValidUserDocumentType(input.DocumentType) {
		s.logger.Error("invalid document type", zap.String("type", input.DocumentType))
		return nil, fileErrors.ErrorInvalidDocumentType
	}

	if !allowedMimeTypes[input.MimeType] {
		s.logger.Error("invalid mime type", zap.String("mime", input.MimeType))
		return nil, fileErrors.ErrorInvalidMimeType
	}

	if input.FileSizeBytes > maxFileSize {
		s.logger.Error("file too large", zap.Int64("size", input.FileSizeBytes))
		return nil, fileErrors.ErrorFileTooLarge
	}

	documentID := uuid.New().String()
	ext := extensionFromMimeType(input.MimeType)
	s3Key := fmt.Sprintf("%s/%s/%s%s", input.DocumentType, input.UserID, documentID, ext)

	documentURL, err := s.storage.Upload(ctx, s3Key, input.Data, input.MimeType, input.FileSizeBytes)
	if err != nil {
		s.logger.Error("S3 upload failed", zap.Error(err), zap.String("key", s3Key))
		return nil, fileErrors.ErrorUploadFailed
	}

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
		s.logger.Error("create user document record failed", zap.Error(err), zap.String("documentID", documentID))
		return nil, fileErrors.ErrorInternalServer
	}

	s.logger.Info("user document uploaded", zap.String("documentID", documentID), zap.String("userID", input.UserID))
	return doc, nil
}

func (s *fileServiceImpl) GetUserDocuments(ctx context.Context, userID string) ([]*domain.UserDocument, error) {
	s.logger.Debug("get user documents", zap.String("userID", userID))
	return s.userDocRead.GetByUserID(ctx, userID)
}

func (s *fileServiceImpl) GetUserDocument(ctx context.Context, documentID string) (*domain.UserDocument, error) {
	s.logger.Debug("get user document", zap.String("documentID", documentID))
	return s.userDocRead.GetByID(ctx, documentID)
}

func (s *fileServiceImpl) GetCurrentUserDocument(ctx context.Context, userID string, documentType string) (*domain.UserDocument, error) {
	s.logger.Debug("get current user document", zap.String("userID", userID), zap.String("type", documentType))
	if !domain.IsValidUserDocumentType(documentType) {
		return nil, fileErrors.ErrorInvalidDocumentType
	}
	return s.userDocRead.GetCurrentByUserIDAndType(ctx, userID, documentType)
}

func (s *fileServiceImpl) DeleteUserDocument(ctx context.Context, documentID string) error {
	s.logger.Debug("delete user document", zap.String("documentID", documentID))
	doc, err := s.userDocRead.GetByID(ctx, documentID)
	if err != nil {
		s.logger.Error("delete user document: not found", zap.Error(err), zap.String("documentID", documentID))
		return err
	}

	s3Key := s3KeyFromURL(doc.DocumentURL, doc.DocumentType, doc.UserID, doc.DocumentID)
	_ = s.storage.Delete(ctx, s3Key)

	if err := s.userDocWrite.Delete(ctx, documentID); err != nil {
		s.logger.Error("delete user document: db delete failed", zap.Error(err), zap.String("documentID", documentID))
		return err
	}

	s.logger.Info("user document deleted", zap.String("documentID", documentID))
	return nil
}

// --- Documents véhicule ---

func (s *fileServiceImpl) UploadVehicleDocument(ctx context.Context, input serviceInterfaces.UploadVehicleDocumentInput) (*domain.VehicleDocument, error) {
	s.logger.Debug("upload vehicle document",
		zap.String("vehicleID", input.VehicleID),
		zap.String("type", input.DocumentType),
		zap.String("mime", input.MimeType),
		zap.Int64("size", input.FileSizeBytes),
	)

	if !domain.IsValidVehicleDocumentType(input.DocumentType) {
		s.logger.Error("invalid vehicle document type", zap.String("type", input.DocumentType))
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
		s.logger.Error("S3 upload failed for vehicle doc", zap.Error(err), zap.String("key", s3Key))
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
		s.logger.Error("create vehicle document record failed", zap.Error(err), zap.String("documentID", documentID))
		return nil, fileErrors.ErrorInternalServer
	}

	s.logger.Info("vehicle document uploaded", zap.String("documentID", documentID), zap.String("vehicleID", input.VehicleID))
	return doc, nil
}

func (s *fileServiceImpl) GetVehicleDocuments(ctx context.Context, vehicleID string) ([]*domain.VehicleDocument, error) {
	s.logger.Debug("get vehicle documents", zap.String("vehicleID", vehicleID))
	return s.vehicleDocRead.GetByVehicleID(ctx, vehicleID)
}

func (s *fileServiceImpl) GetVehicleDocument(ctx context.Context, documentID string) (*domain.VehicleDocument, error) {
	s.logger.Debug("get vehicle document", zap.String("documentID", documentID))
	return s.vehicleDocRead.GetByID(ctx, documentID)
}

func (s *fileServiceImpl) DeleteVehicleDocument(ctx context.Context, documentID string) error {
	s.logger.Debug("delete vehicle document", zap.String("documentID", documentID))
	doc, err := s.vehicleDocRead.GetByID(ctx, documentID)
	if err != nil {
		s.logger.Error("delete vehicle document: not found", zap.Error(err), zap.String("documentID", documentID))
		return err
	}

	s3Key := s3KeyFromURL(doc.DocumentURL, doc.DocumentType, doc.VehicleID, doc.DocumentID)
	_ = s.storage.Delete(ctx, s3Key)

	if err := s.vehicleDocWrite.Delete(ctx, documentID); err != nil {
		s.logger.Error("delete vehicle document: db delete failed", zap.Error(err), zap.String("documentID", documentID))
		return err
	}

	s.logger.Info("vehicle document deleted", zap.String("documentID", documentID))
	return nil
}

// --- Revues ---

func (s *fileServiceImpl) CreateDocumentReview(ctx context.Context, input serviceInterfaces.CreateReviewInput) (*domain.DocumentReview, error) {
	s.logger.Debug("create document review",
		zap.String("userDocID", input.UserDocumentID),
		zap.String("vehicleDocID", input.VehicleDocumentID),
		zap.String("decision", input.Decision),
	)

	if !domain.IsValidReviewDecision(input.Decision) {
		s.logger.Error("invalid review decision", zap.String("decision", input.Decision))
		return nil, fileErrors.ErrorInvalidReviewDecision
	}
	if !domain.IsValidReviewerType(input.ReviewedByType) {
		s.logger.Error("invalid reviewer type", zap.String("reviewedByType", input.ReviewedByType))
		return nil, fileErrors.ErrorInternalServer
	}

	reviewID := uuid.New().String()

	var userDocID *string
	var vehicleDocID *string

	if input.UserDocumentID != "" {
		userDocID = &input.UserDocumentID
		_, err := s.userDocRead.GetByID(ctx, input.UserDocumentID)
		if err != nil {
			s.logger.Error("review: user document not found", zap.Error(err), zap.String("userDocID", input.UserDocumentID))
			return nil, err
		}
	}
	if input.VehicleDocumentID != "" {
		vehicleDocID = &input.VehicleDocumentID
		_, err := s.vehicleDocRead.GetByID(ctx, input.VehicleDocumentID)
		if err != nil {
			s.logger.Error("review: vehicle document not found", zap.Error(err), zap.String("vehicleDocID", input.VehicleDocumentID))
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
		s.logger.Error("create review record failed", zap.Error(err), zap.String("reviewID", reviewID))
		return nil, fileErrors.ErrorInternalServer
	}

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

	s.logger.Info("document review created", zap.String("reviewID", reviewID), zap.String("decision", input.Decision))
	return review, nil
}

func (s *fileServiceImpl) GetDocumentReviews(ctx context.Context, userDocumentID string, vehicleDocumentID string) ([]*domain.DocumentReview, error) {
	s.logger.Debug("get document reviews", zap.String("userDocID", userDocumentID), zap.String("vehicleDocID", vehicleDocumentID))
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
