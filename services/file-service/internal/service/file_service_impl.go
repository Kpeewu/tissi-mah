package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/text/unicode/norm"

	fileClient "github.com/Kpeewu/tissi-mah/services/file-service/internal/client"
	"github.com/Kpeewu/tissi-mah/services/file-service/internal/domain"
	repoInterfaces "github.com/Kpeewu/tissi-mah/services/file-service/internal/repository/interfaces"
	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/file-service/internal/service/interfaces"
	"github.com/Kpeewu/tissi-mah/services/file-service/internal/storage"
	fileErrors "github.com/Kpeewu/tissi-mah/services/file-service/pkg/errors"
)

// Types MIME autorisés
var allowedMimeTypes = map[string]bool{
	"image/jpeg":      true,
	"image/jpg":       true, // alias non-standard de image/jpeg
	"image/png":       true,
	"image/webp":      true,
	"image/heic":      true, // iPhone (HEIF single image)
	"image/heif":      true, // HEIF générique
	"image/tiff":      true,
	"application/pdf": true,
}

// Taille maximale : 10 Mo
const maxFileSize int64 = 10 * 1024 * 1024

type fileServiceImpl struct {
	userDocRead      repoInterfaces.UserDocumentRepositoryRead
	userDocWrite     repoInterfaces.UserDocumentRepositoryWrite
	vehicleDocRead   repoInterfaces.VehicleDocumentRepositoryRead
	vehicleDocWrite  repoInterfaces.VehicleDocumentRepositoryWrite
	reviewRead       repoInterfaces.DocumentReviewRepositoryRead
	reviewWrite      repoInterfaces.DocumentReviewRepositoryWrite
	storage          storage.StorageClient
	moderationClient fileClient.ModerationClient // nil si désactivé
	logger           *zap.Logger
}

func NewFileService(
	userDocRead repoInterfaces.UserDocumentRepositoryRead,
	userDocWrite repoInterfaces.UserDocumentRepositoryWrite,
	vehicleDocRead repoInterfaces.VehicleDocumentRepositoryRead,
	vehicleDocWrite repoInterfaces.VehicleDocumentRepositoryWrite,
	reviewRead repoInterfaces.DocumentReviewRepositoryRead,
	reviewWrite repoInterfaces.DocumentReviewRepositoryWrite,
	storageClient storage.StorageClient,
	moderationClient fileClient.ModerationClient,
	logger *zap.Logger,
) serviceInterfaces.FileService {
	return &fileServiceImpl{
		userDocRead:      userDocRead,
		userDocWrite:     userDocWrite,
		vehicleDocRead:   vehicleDocRead,
		vehicleDocWrite:  vehicleDocWrite,
		reviewRead:       reviewRead,
		reviewWrite:      reviewWrite,
		storage:          storageClient,
		moderationClient: moderationClient,
		logger:           logger,
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

	// Bloquer si un document courant existe déjà pour ce type (hors profilePicture qui peut être mis à jour librement)
	if input.DocumentType != "profilePicture" {
		existing, _ := s.userDocRead.GetCurrentByUserIDAndType(ctx, input.UserID, input.DocumentType)
		if existing != nil {
			s.logger.Warn("document already submitted",
				zap.String("userID", input.UserID),
				zap.String("type", input.DocumentType),
				zap.String("existingID", existing.DocumentID),
			)
			return nil, fileErrors.ErrorDocumentAlreadySubmitted
		}
	}

	documentID := uuid.New().String()
	ext := extensionFromMimeType(input.MimeType)
	s3Key := fmt.Sprintf("documents/%s%s", documentID, ext)

	// Modération synchrone pour les photos de profil (avant upload S3).
	if input.DocumentType == "profilePicture" && s.moderationClient != nil {
		imageData, readErr := io.ReadAll(input.Data)
		if readErr != nil {
			s.logger.Error("failed to read image data for moderation", zap.Error(readErr))
			return nil, fileErrors.ErrorInternalServer
		}
		// Remettre les données dans le reader pour l'upload S3 qui suit.
		input.Data = bytes.NewReader(imageData)

		modResult, modErr := s.moderationClient.ModerateImage(ctx, documentID, input.UserID, imageData, input.MimeType)
		if modErr == nil && modResult.Decision == fileClient.ModerationBlocked {
			s.logger.Info("profile picture blocked by moderation",
				zap.String("userID", input.UserID),
				zap.String("reason", modResult.Reason),
			)
			return nil, fileErrors.ErrorContentBlocked
		}
	}

	if _, uploadErr := s.storage.Upload(ctx, s3Key, input.Data, input.MimeType, input.FileSizeBytes); uploadErr != nil {
		s.logger.Error("S3 upload failed", zap.Error(uploadErr), zap.String("key", s3Key))
		return nil, fileErrors.ErrorUploadFailed
	}

	now := time.Now().UTC()
	doc := &domain.UserDocument{
		DocumentID:     documentID,
		UserID:         input.UserID,
		DocumentName:   input.DocumentName,
		DocumentType:   input.DocumentType,
		DocumentKey:    s3Key,
		FileSizeBytes:  input.FileSizeBytes,
		MimeType:       input.MimeType,
		DocumentNumber: input.DocumentNumber,
		IssuedAt:       input.IssuedAt,
		ExpireAt:       input.ExpireAt,
		IssuingCountry: input.IssuingCountry,
		Status:         "pending",
		IsCurrent:      true,
		UploadedAt:     now,
		UpdatedAt:      now,
	}

	_, err := s.userDocWrite.Create(ctx, doc)
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

func (s *fileServiceImpl) GetDocument(ctx context.Context, input serviceInterfaces.GetDocumentInput) (*serviceInterfaces.GetDocumentResult, error) {
	s.logger.Debug("get document",
		zap.String("fileID", input.FileID),
		zap.String("userID", input.UserID),
		zap.String("supportID", input.SupportID),
	)

	if input.FileID == "" {
		return nil, fileErrors.ErrorDocumentNotFound
	}

	doc, err := s.userDocRead.GetByID(ctx, input.FileID)
	if err != nil {
		s.logger.Error("get document: not found", zap.Error(err), zap.String("fileID", input.FileID))
		return nil, fileErrors.ErrorDocumentNotFound
	}

	// Vérification de propriété uniquement pour les utilisateurs
	if input.SupportID == "" {
		if doc.UserID != input.UserID {
			s.logger.Warn("get document: unauthorized access",
				zap.String("fileID", input.FileID),
				zap.String("userID", input.UserID),
				zap.String("docOwner", doc.UserID),
			)
			return nil, fileErrors.ErrorUnauthorized
		}
	}

	presignedURL, err := s.storage.GeneratePresignedURL(ctx, doc.DocumentKey, time.Hour)
	if err != nil {
		s.logger.Error("get document: presign failed", zap.Error(err), zap.String("fileID", input.FileID))
		return nil, fileErrors.ErrorUploadFailed
	}

	s.logger.Info("get document: success", zap.String("fileID", input.FileID))
	return &serviceInterfaces.GetDocumentResult{
		FileID:                doc.DocumentID,
		FileURL:               presignedURL,
		FileType:              doc.DocumentType,
		PresignedURLExpiresAt: time.Now().UTC().Add(30 * time.Minute).Format(time.RFC3339),
	}, nil
}

func (s *fileServiceImpl) DeleteFile(ctx context.Context, input serviceInterfaces.DeleteFileInput) error {
	s.logger.Debug("delete file", zap.String("userID", input.UserID), zap.String("fileID", input.FileID))

	doc, err := s.userDocRead.GetByID(ctx, input.FileID)
	if err != nil {
		s.logger.Error("delete file: document not found", zap.Error(err), zap.String("fileID", input.FileID))
		return fileErrors.ErrorDocumentNotFound
	}

	if doc.UserID != input.UserID {
		s.logger.Warn("delete file: unauthorized",
			zap.String("fileID", input.FileID),
			zap.String("userID", input.UserID),
			zap.String("docOwner", doc.UserID),
		)
		return fileErrors.ErrorUnauthorized
	}

	return s.DeleteUserDocument(ctx, input.FileID)
}

func (s *fileServiceImpl) DeleteUserDocument(ctx context.Context, documentID string) error {
	s.logger.Debug("delete user document", zap.String("documentID", documentID))
	doc, err := s.userDocRead.GetByID(ctx, documentID)
	if err != nil {
		s.logger.Error("delete user document: not found", zap.Error(err), zap.String("documentID", documentID))
		return err
	}

	_ = s.storage.Delete(ctx, doc.DocumentKey)

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

	// Bloquer si un document courant existe déjà pour ce type sur ce véhicule
	if existingVeh, _ := s.vehicleDocRead.GetCurrentByVehicleIDAndType(ctx, input.VehicleID, input.DocumentType); existingVeh != nil {
		s.logger.Warn("vehicle document already submitted",
			zap.String("vehicleID", input.VehicleID),
			zap.String("type", input.DocumentType),
			zap.String("existingID", existingVeh.DocumentID),
		)
		return nil, fileErrors.ErrorDocumentAlreadySubmitted
	}

	documentID := uuid.New().String()
	ext := extensionFromMimeType(input.MimeType)
	s3Key := fmt.Sprintf("documents/%s%s", documentID, ext)

	if _, uploadErr := s.storage.Upload(ctx, s3Key, input.Data, input.MimeType, input.FileSizeBytes); uploadErr != nil {
		s.logger.Error("S3 upload failed for vehicle doc", zap.Error(uploadErr), zap.String("key", s3Key))
		return nil, fileErrors.ErrorUploadFailed
	}

	now := time.Now().UTC()
	doc := &domain.VehicleDocument{
		DocumentID:       documentID,
		UserID:           input.UserID,
		VehicleID:        input.VehicleID,
		DocumentName:     input.DocumentName,
		DocumentType:     input.DocumentType,
		DocumentKey:      s3Key,
		FileSizeBytes:    input.FileSizeBytes,
		MimeType:         input.MimeType,
		DocumentNumber:   input.DocumentNumber,
		IssuedAt:         input.IssuedAt,
		ExpireAt:         input.ExpireAt,
		IssuingAuthority: input.IssuingAuthority,
		Status:           "pending",
		IsCurrent:        true,
		UploadedAt:       now,
		UpdatedAt:        now,
	}

	if _, err := s.vehicleDocWrite.Create(ctx, doc); err != nil {
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

func (s *fileServiceImpl) GetVehicleDocumentsByUserID(ctx context.Context, userID string) ([]*domain.VehicleDocument, error) {
	s.logger.Debug("get vehicle documents by userID", zap.String("userID", userID))
	return s.vehicleDocRead.GetByUserID(ctx, userID)
}

// kycListLimit borne la file de validation manuelle (back-office).
const kycListLimit int32 = 1000

func (s *fileServiceImpl) ListKycDocuments(ctx context.Context, statuses []string) ([]*serviceInterfaces.KycDocument, error) {
	s.logger.Debug("list kyc documents", zap.Strings("statuses", statuses))

	userDocs, err := s.userDocRead.ListCurrentByStatuses(ctx, statuses, kycListLimit)
	if err != nil {
		return nil, err
	}
	vehicleDocs, err := s.vehicleDocRead.ListCurrentByStatuses(ctx, statuses, kycListLimit)
	if err != nil {
		return nil, err
	}

	out := make([]*serviceInterfaces.KycDocument, 0, len(userDocs)+len(vehicleDocs))
	for _, d := range userDocs {
		out = append(out, &serviceInterfaces.KycDocument{
			DocumentID:   d.DocumentID,
			UserID:       d.UserID,
			DocumentType: d.DocumentType,
			Status:       d.Status,
			OwnerKind:    "user",
			UpdatedAt:    d.UpdatedAt.UTC().Format(time.RFC3339),
		})
	}
	for _, d := range vehicleDocs {
		out = append(out, &serviceInterfaces.KycDocument{
			DocumentID:   d.DocumentID,
			UserID:       d.UserID,
			VehicleID:    d.VehicleID,
			DocumentType: d.DocumentType,
			Status:       d.Status,
			OwnerKind:    "vehicle",
			UpdatedAt:    d.UpdatedAt.UTC().Format(time.RFC3339),
		})
	}
	return out, nil
}

func (s *fileServiceImpl) DeleteVehicleDocument(ctx context.Context, documentID string) error {
	s.logger.Debug("delete vehicle document", zap.String("documentID", documentID))
	doc, err := s.vehicleDocRead.GetByID(ctx, documentID)
	if err != nil {
		s.logger.Error("delete vehicle document: not found", zap.Error(err), zap.String("documentID", documentID))
		return err
	}

	_ = s.storage.Delete(ctx, doc.DocumentKey)

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
		zap.String("userID", input.UserID),
		zap.String("documentType", input.DocumentType),
		zap.String("userDocID", input.UserDocumentID),
		zap.String("vehicleDocID", input.VehicleDocumentID),
		zap.String("decision", input.Decision),
	)

	// user_id est obligatoire (dénormalisé depuis la migration 000008 pour
	// permettre les reviews Persona 100% sans FK doc).
	if input.UserID == "" {
		s.logger.Error("review must include user_id")
		return nil, fileErrors.ErrorMissingUserID
	}

	// Les FK doc sont désormais optionnelles. Seule contrainte : pas les deux.
	if input.UserDocumentID != "" && input.VehicleDocumentID != "" {
		s.logger.Error("review must reference only one document, not both")
		return nil, fileErrors.ErrorMultipleDocumentReference
	}

	if !domain.IsValidReviewDecision(input.Decision) {
		s.logger.Error("invalid review decision", zap.String("decision", input.Decision))
		return nil, fileErrors.ErrorInvalidReviewDecision
	}
	if !domain.IsValidReviewType(input.ReviewType) {
		s.logger.Error("invalid review type", zap.String("reviewType", input.ReviewType))
		return nil, fileErrors.ErrorInvalidReviewType
	}

	reviewStatus := input.Status
	if reviewStatus == "" {
		reviewStatus = "pending"
	}
	if !domain.IsValidReviewStatus(reviewStatus) {
		s.logger.Error("invalid review status", zap.String("status", reviewStatus))
		return nil, fileErrors.ErrorInvalidReviewStatus
	}

	if !domain.IsValidReasonRejection(input.ReasonRejection) {
		s.logger.Error("invalid reason rejection", zap.String("reasonRejection", input.ReasonRejection))
		return nil, fileErrors.ErrorInvalidReasonRejection
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

	var personaRawPayload json.RawMessage
	if len(input.PersonaRawPayload) > 0 {
		personaRawPayload = input.PersonaRawPayload
	}

	var sessionExpiresAt *time.Time
	if input.SessionExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, input.SessionExpiresAt)
		if err != nil {
			s.logger.Error("invalid session_expires_at format", zap.String("value", input.SessionExpiresAt), zap.Error(err))
			return nil, fileErrors.ErrorInternalServer
		}
		sessionExpiresAt = &t
	}

	var webhookReceivedAt *time.Time
	if input.WebhookReceivedAt != "" {
		t, err := time.Parse(time.RFC3339, input.WebhookReceivedAt)
		if err != nil {
			s.logger.Error("invalid webhook_received_at format", zap.String("value", input.WebhookReceivedAt), zap.Error(err))
			return nil, fileErrors.ErrorInternalServer
		}
		webhookReceivedAt = &t
	}

	var submittedAt *time.Time
	if input.SubmittedAt != "" {
		t, err := time.Parse(time.RFC3339, input.SubmittedAt)
		if err != nil {
			s.logger.Error("invalid submitted_at format", zap.String("value", input.SubmittedAt), zap.Error(err))
			return nil, fileErrors.ErrorInternalServer
		}
		submittedAt = &t
	}

	var previousReviewID *string
	if input.PreviousReviewID != "" {
		previousReviewID = &input.PreviousReviewID
	}

	attemptNumber := int16(input.AttemptNumber)
	if attemptNumber == 0 {
		attemptNumber = 1
	}

	var secondUserDocID *string
	if input.SecondUserDocumentID != "" {
		secondUserDocID = &input.SecondUserDocumentID
	}

	now := time.Now().UTC()
	review := &domain.DocumentReview{
		ReviewID:             reviewID,
		UserID:               input.UserID,
		DocumentType:         input.DocumentType,
		LogicalDocumentType:  domain.ToLogicalDocumentType(input.DocumentType),
		UserDocumentID:       userDocID,
		SecondUserDocumentID: secondUserDocID,
		VehicleDocumentID:    vehicleDocID,

		PersonaInquiryID:    input.PersonaInquiryID,
		PersonaTemplateID:   input.PersonaTemplateID,
		PersonaSessionToken: input.PersonaSessionToken,
		SessionExpiresAt:    sessionExpiresAt,

		WebhookEventType:  input.WebhookEventType,
		WebhookReceivedAt: webhookReceivedAt,
		PersonaRawPayload: personaRawPayload,

		AttemptNumber:    attemptNumber,
		PreviousReviewID: previousReviewID,

		Status:           reviewStatus,
		Decision:         input.Decision,
		ReasonRejection:  input.ReasonRejection,
		RejectionDetails: input.RejectionDetails,

		ReviewedBy: input.ReviewedBy,
		ReviewType: input.ReviewType,
		ReviewedAt: now,

		Notes:         input.Notes,
		ExtractedData: extractedData,

		SubmittedAt: submittedAt,
		UpdatedAt:   now,
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
	if input.SecondUserDocumentID != "" {
		doc, _ := s.userDocRead.GetByID(ctx, input.SecondUserDocumentID)
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

func (s *fileServiceImpl) GetDocumentReview(ctx context.Context, reviewID string) (*domain.DocumentReview, error) {
	s.logger.Debug("get document review", zap.String("reviewID", reviewID))
	return s.reviewRead.GetByID(ctx, reviewID)
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

func (s *fileServiceImpl) GetDocumentReviewByPersonaInquiryID(ctx context.Context, personaInquiryID string) (*domain.DocumentReview, error) {
	s.logger.Debug("get document review by persona_inquiry_id", zap.String("personaInquiryID", personaInquiryID))
	return s.reviewRead.GetByPersonaInquiryID(ctx, personaInquiryID)
}

func (s *fileServiceImpl) GetDocumentReviewsByUserID(ctx context.Context, userID string) ([]*domain.DocumentReview, error) {
	s.logger.Debug("get document reviews by userID", zap.String("userID", userID))
	return s.reviewRead.GetByUserID(ctx, userID)
}

func (s *fileServiceImpl) UpdateDocumentReview(ctx context.Context, review *domain.DocumentReview) (*domain.DocumentReview, error) {
	s.logger.Debug("update document review", zap.String("reviewID", review.ReviewID))

	if err := s.reviewWrite.Update(ctx, review); err != nil {
		s.logger.Error("failed to update document review", zap.String("reviewID", review.ReviewID), zap.Error(err))
		return nil, err
	}

	// Relire la revue mise à jour
	updated, err := s.reviewRead.GetByID(ctx, review.ReviewID)
	if err != nil {
		s.logger.Error("failed to read updated review", zap.String("reviewID", review.ReviewID), zap.Error(err))
		return nil, err
	}

	s.logger.Info("document review updated", zap.String("reviewID", updated.ReviewID))
	return updated, nil
}

func (s *fileServiceImpl) GetDocumentReviewHistory(ctx context.Context, userID string, logicalDocumentType string) ([]*domain.DocumentReview, error) {
	s.logger.Debug("get document review history", zap.String("userID", userID), zap.String("logicalType", logicalDocumentType))
	return s.reviewRead.GetHistoryByUserIDAndLogicalType(ctx, userID, logicalDocumentType)
}

func (s *fileServiceImpl) ListDocumentReviews(ctx context.Context, userID string, status string, decision string, page int32, pageSize int32) ([]*domain.DocumentReview, error) {
	s.logger.Debug("list document reviews",
		zap.String("userID", userID),
		zap.String("status", status),
		zap.String("decision", decision),
		zap.Int32("page", page),
		zap.Int32("pageSize", pageSize),
	)
	offset := page * pageSize
	return s.reviewRead.List(ctx, userID, status, decision, offset, pageSize)
}

// --- Helpers ---

func extensionFromMimeType(mimeType string) string {
	switch mimeType {
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "image/heic":
		return ".heic"
	case "image/heif":
		return ".heif"
	case "image/tiff":
		return ".tiff"
	case "application/pdf":
		return ".pdf"
	default:
		return ""
	}
}

// detectMimeType complète http.DetectContentType avec un sniff HEIC/HEIF
// car la stdlib ne reconnaît pas ces formats (retourne application/octet-stream).
// Signature ISO/IEC 14496-12 : octets 4-7 = "ftyp", octets 8-11 = brand.
func detectMimeType(data []byte) string {
	mime := http.DetectContentType(data)
	if mime != "application/octet-stream" {
		return mime
	}
	if len(data) < 12 || !bytes.Equal(data[4:8], []byte("ftyp")) {
		return mime
	}
	switch string(data[8:12]) {
	case "heic", "heix", "hevc", "hevx":
		return "image/heic"
	case "mif1", "msf1", "heim", "heis", "hevm", "hevs":
		return "image/heif"
	}
	return mime
}

// --- Remplacement de document ---

// statuts qui bloquent le remplacement
var nonReplaceableStatuses = map[string]bool{
	"pending":     true,
	"underReview": true,
	"approved":    true,
}

// ChangeDocument remplace le fichier d'un document (utilisateur ou véhicule) existant.
// Le document courant doit avoir le statut "rejected" ou "expired".
// Gère lui-même l'upload S3 + création DB + MarkAsReplaced sans passer par UploadUserDocument
// (qui bloquerait désormais si un document courant existe déjà).
func (s *fileServiceImpl) ChangeDocument(ctx context.Context, input serviceInterfaces.ChangeDocumentInput) (*serviceInterfaces.UploadedDocument, error) {
	s.logger.Debug("change document", zap.String("userID", input.UserID), zap.String("fileID", input.FileID))

	if len(input.NewDocument) == 0 {
		return nil, fileErrors.ErrorInvalidDocumentType
	}

	// Chercher le document : user docs d'abord, vehicle docs ensuite
	userDoc, userErr := s.userDocRead.GetByID(ctx, input.FileID)
	vehicleDoc, vehicleErr := s.vehicleDocRead.GetByID(ctx, input.FileID)

	if userErr != nil && vehicleErr != nil {
		s.logger.Error("change document: document not found", zap.String("fileID", input.FileID))
		return nil, fileErrors.ErrorDocumentNotFound
	}

	mimeType := detectMimeType(input.NewDocument)
	if !allowedMimeTypes[mimeType] {
		s.logger.Error("change document: unsupported mime type",
			zap.String("fileID", input.FileID),
			zap.String("detectedMime", mimeType),
		)
		return nil, fileErrors.ErrorInvalidMimeType
	}

	documentID := uuid.New().String()
	ext := extensionFromMimeType(mimeType)
	s3Key := fmt.Sprintf("documents/%s%s", documentID, ext)
	now := time.Now().UTC()

	if userDoc != nil {
		// --- Document utilisateur ---
		if userDoc.UserID != input.UserID {
			s.logger.Error("change document: user mismatch",
				zap.String("expected", userDoc.UserID),
				zap.String("got", input.UserID),
			)
			return nil, fileErrors.ErrorDocumentNotFound
		}
		if !userDoc.IsCurrent {
			s.logger.Warn("change document: document is not current", zap.String("fileID", input.FileID))
			return nil, fileErrors.ErrorDocumentNotFound
		}
		if nonReplaceableStatuses[userDoc.Status] {
			s.logger.Warn("change document: document status does not allow replacement",
				zap.String("fileID", input.FileID),
				zap.String("status", userDoc.Status),
			)
			return nil, fileErrors.ErrorDocumentNotReplaceable
		}

		// Résoudre les métadonnées : conserver l'existant, écraser si fourni
		docNumber := userDoc.DocumentNumber
		if input.DocumentNumber != "" {
			docNumber = input.DocumentNumber
		}
		issuingPlace := userDoc.IssuingCountry
		if input.IssuingPlace != "" {
			issuingPlace = input.IssuingPlace
		}
		issuedAt := userDoc.IssuedAt
		if input.IssuedAt != "" {
			if t, parseErr := time.Parse(time.RFC3339, input.IssuedAt); parseErr == nil {
				issuedAt = &t
			}
		}
		expireAt := userDoc.ExpireAt
		if input.ExpireAt != "" {
			if t, parseErr := time.Parse(time.RFC3339, input.ExpireAt); parseErr == nil {
				expireAt = &t
			}
		}

		if _, uploadErr := s.storage.Upload(ctx, s3Key, bytes.NewReader(input.NewDocument), mimeType, int64(len(input.NewDocument))); uploadErr != nil {
			s.logger.Error("change document: S3 upload failed", zap.Error(uploadErr), zap.String("key", s3Key))
			return nil, fileErrors.ErrorUploadFailed
		}

		newDoc := &domain.UserDocument{
			DocumentID:     documentID,
			UserID:         userDoc.UserID,
			DocumentName:   userDoc.DocumentName,
			DocumentType:   userDoc.DocumentType,
			DocumentKey:    s3Key,
			FileSizeBytes:  int64(len(input.NewDocument)),
			MimeType:       mimeType,
			DocumentNumber: docNumber,
			IssuedAt:       issuedAt,
			ExpireAt:       expireAt,
			IssuingCountry: issuingPlace,
			Status:         "pending",
			IsCurrent:      true,
			UploadedAt:     now,
			UpdatedAt:      now,
		}
		if _, createErr := s.userDocWrite.Create(ctx, newDoc); createErr != nil {
			s.logger.Error("change document: create record failed", zap.Error(createErr))
			return nil, fileErrors.ErrorInternalServer
		}
		_ = s.userDocWrite.MarkAsReplaced(ctx, userDoc.DocumentID, documentID)

		presignedURL, _ := s.storage.GeneratePresignedURL(ctx, newDoc.DocumentKey, time.Hour)
		s.logger.Info("user document changed",
			zap.String("oldFileID", input.FileID),
			zap.String("newFileID", documentID),
			zap.String("userID", input.UserID),
		)
		return &serviceInterfaces.UploadedDocument{
			DocumentID:   newDoc.DocumentID,
			DocumentURL:  presignedURL,
			DocumentType: newDoc.DocumentType,
			DocumentName: newDoc.DocumentName,
		}, nil
	}

	// --- Document véhicule ---
	if vehicleDoc.UserID != input.UserID {
		s.logger.Error("change document: vehicle doc user mismatch",
			zap.String("expected", vehicleDoc.UserID),
			zap.String("got", input.UserID),
		)
		return nil, fileErrors.ErrorDocumentNotFound
	}
	if !vehicleDoc.IsCurrent {
		s.logger.Warn("change vehicle document: document is not current", zap.String("fileID", input.FileID))
		return nil, fileErrors.ErrorDocumentNotFound
	}
	if nonReplaceableStatuses[vehicleDoc.Status] {
		s.logger.Warn("change vehicle document: status does not allow replacement",
			zap.String("fileID", input.FileID),
			zap.String("status", vehicleDoc.Status),
		)
		return nil, fileErrors.ErrorDocumentNotReplaceable
	}

	// Résoudre les métadonnées véhicule
	docNumber := vehicleDoc.DocumentNumber
	if input.DocumentNumber != "" {
		docNumber = input.DocumentNumber
	}
	issuingPlace := vehicleDoc.IssuingAuthority
	if input.IssuingPlace != "" {
		issuingPlace = input.IssuingPlace
	}
	issuedAt := vehicleDoc.IssuedAt
	if input.IssuedAt != "" {
		if t, parseErr := time.Parse(time.RFC3339, input.IssuedAt); parseErr == nil {
			issuedAt = &t
		}
	}
	expireAt := vehicleDoc.ExpireAt
	if input.ExpireAt != "" {
		if t, parseErr := time.Parse(time.RFC3339, input.ExpireAt); parseErr == nil {
			expireAt = &t
		}
	}

	if _, uploadErr := s.storage.Upload(ctx, s3Key, bytes.NewReader(input.NewDocument), mimeType, int64(len(input.NewDocument))); uploadErr != nil {
		s.logger.Error("change vehicle document: S3 upload failed", zap.Error(uploadErr), zap.String("key", s3Key))
		return nil, fileErrors.ErrorUploadFailed
	}

	newVehicleDoc := &domain.VehicleDocument{
		DocumentID:       documentID,
		UserID:           vehicleDoc.UserID,
		VehicleID:        vehicleDoc.VehicleID,
		DocumentName:     vehicleDoc.DocumentName,
		DocumentType:     vehicleDoc.DocumentType,
		DocumentKey:      s3Key,
		FileSizeBytes:    int64(len(input.NewDocument)),
		MimeType:         mimeType,
		DocumentNumber:   docNumber,
		IssuedAt:         issuedAt,
		ExpireAt:         expireAt,
		IssuingAuthority: issuingPlace,
		Status:           "pending",
		IsCurrent:        true,
		UploadedAt:       now,
		UpdatedAt:        now,
	}
	if _, createErr := s.vehicleDocWrite.Create(ctx, newVehicleDoc); createErr != nil {
		s.logger.Error("change vehicle document: create record failed", zap.Error(createErr))
		return nil, fileErrors.ErrorInternalServer
	}
	_ = s.vehicleDocWrite.MarkAsReplaced(ctx, vehicleDoc.DocumentID, documentID)

	presignedURL, _ := s.storage.GeneratePresignedURL(ctx, newVehicleDoc.DocumentKey, time.Hour)
	s.logger.Info("vehicle document changed",
		zap.String("oldFileID", input.FileID),
		zap.String("newFileID", documentID),
		zap.String("userID", input.UserID),
	)
	return &serviceInterfaces.UploadedDocument{
		DocumentID:   newVehicleDoc.DocumentID,
		DocumentURL:  presignedURL,
		DocumentType: newVehicleDoc.DocumentType,
		DocumentName: newVehicleDoc.DocumentName,
	}, nil
}

// --- Upload identité ---

// UploadIdDocument upload les documents d'identité vers S3/MinIO et sauvegarde les URLs en base.
// Les fichiers sont uploadés individuellement via UploadUserDocument.
// Retourne la liste des documents créés (1 pour Passport, 2 pour IDCard / DriverLicence).
func (s *fileServiceImpl) UploadIdDocument(ctx context.Context, input serviceInterfaces.UploadIdDocumentInput) ([]*serviceInterfaces.UploadedDocument, error) {
	s.logger.Debug("upload id document", zap.String("profileID", input.UserID), zap.String("type", input.DocumentType))

	if input.UserID == "" {
		return nil, fileErrors.ErrorInvalidDocumentType
	}

	// Valider les métadonnées légales obligatoires
	if input.DocumentNumber == "" || input.IssuedAt == "" || input.ExpireAt == "" || input.IssuingCountry == "" {
		s.logger.Error("upload id document: métadonnées légales manquantes",
			zap.String("profileID", input.UserID),
			zap.String("type", input.DocumentType),
		)
		return nil, fileErrors.ErrorMissingDocumentMetadata
	}
	issuedAt, err := time.Parse(time.RFC3339, input.IssuedAt)
	if err != nil {
		s.logger.Error("upload id document: issued_at invalide", zap.String("value", input.IssuedAt), zap.Error(err))
		return nil, fileErrors.ErrorMissingDocumentMetadata
	}
	expireAt, err := time.Parse(time.RFC3339, input.ExpireAt)
	if err != nil {
		s.logger.Error("upload id document: expire_at invalide", zap.String("value", input.ExpireAt), zap.Error(err))
		return nil, fileErrors.ErrorMissingDocumentMetadata
	}

	// Détermine les fichiers requis selon le type de document
	type fileUpload struct {
		data    []byte
		docType string
		docName string
	}

	timestamp := time.Now().UTC().Format("20060102_150405")
	lastName := sanitizeForDocName(input.LastName)
	firstName := sanitizeForDocName(input.FirstName)
	prefix := fmt.Sprintf("%s_%s_%s", lastName, firstName, timestamp)

	var uploads []fileUpload

	switch input.DocumentType {
	case "IDCard":
		if len(input.IDCardRecto) == 0 || len(input.IDCardVerso) == 0 {
			s.logger.Error("IDCard: recto et verso obligatoires", zap.String("profileID", input.UserID))
			return nil, fileErrors.ErrorInvalidDocumentType
		}
		uploads = []fileUpload{
			{data: input.IDCardRecto, docType: "idCardFront", docName: prefix + "_id_card_recto"},
			{data: input.IDCardVerso, docType: "idCardBack", docName: prefix + "_id_card_verso"},
		}
	case "Passport":
		if len(input.Passport) == 0 {
			s.logger.Error("Passport: fichier obligatoire", zap.String("profileID", input.UserID))
			return nil, fileErrors.ErrorInvalidDocumentType
		}
		uploads = []fileUpload{
			{data: input.Passport, docType: "passport", docName: prefix + "_passport"},
		}
	case "DriverLicence":
		if len(input.DriverLicenceRecto) == 0 || len(input.DriverLicenceVerso) == 0 {
			s.logger.Error("DriverLicence: recto et verso obligatoires", zap.String("profileID", input.UserID))
			return nil, fileErrors.ErrorInvalidDocumentType
		}
		uploads = []fileUpload{
			{data: input.DriverLicenceRecto, docType: "driverLicenceFront", docName: prefix + "_driver_licence_recto"},
			{data: input.DriverLicenceVerso, docType: "driverLicenceBack", docName: prefix + "_driver_licence_verso"},
		}
	default:
		s.logger.Error("type de document inconnu", zap.String("type", input.DocumentType))
		return nil, fileErrors.ErrorInvalidDocumentType
	}

	// Upload chaque fichier vers S3 + DB
	created := make([]*serviceInterfaces.UploadedDocument, 0, len(uploads))
	for _, u := range uploads {
		mimeType := detectMimeType(u.data)
		if !allowedMimeTypes[mimeType] {
			s.logger.Error("upload id document: unsupported mime type",
				zap.String("profileID", input.UserID),
				zap.String("docType", u.docType),
				zap.String("detectedMime", mimeType),
			)
			return nil, fileErrors.ErrorInvalidMimeType
		}

		doc, err := s.UploadUserDocument(ctx, serviceInterfaces.UploadUserDocumentInput{
			UserID:         input.UserID,
			DocumentName:   u.docName,
			DocumentType:   u.docType,
			MimeType:       mimeType,
			FileSizeBytes:  int64(len(u.data)),
			Data:           bytes.NewReader(u.data),
			DocumentNumber: input.DocumentNumber,
			IssuedAt:       &issuedAt,
			ExpireAt:       &expireAt,
			IssuingCountry: input.IssuingCountry,
		})
		if err != nil {
			s.logger.Error("upload id document file failed",
				zap.String("profileID", input.UserID),
				zap.String("docType", u.docType),
				zap.Error(err),
			)
			return nil, err
		}
		s.logger.Info("id document file uploaded",
			zap.String("profileID", input.UserID),
			zap.String("docType", u.docType),
			zap.String("documentID", doc.DocumentID),
		)
		presignedURL, _ := s.storage.GeneratePresignedURL(ctx, doc.DocumentKey, time.Hour)
		created = append(created, &serviceInterfaces.UploadedDocument{
			DocumentID:   doc.DocumentID,
			DocumentURL:  presignedURL,
			DocumentType: doc.DocumentType,
			DocumentName: doc.DocumentName,
		})
	}

	return created, nil
}

// UploadVehicleDocuments upload le permis de conduire, l'assurance et la carte grise
// vers S3/MinIO et sauvegarde les URLs en base via UploadVehicleDocument.
// Retourne les 3 documents créés dans l'ordre permis / assurance / carte grise.
func (s *fileServiceImpl) UploadVehicleDocuments(ctx context.Context, input serviceInterfaces.UploadVehicleDocumentsInput) ([]*serviceInterfaces.UploadedDocument, error) {
	s.logger.Debug("upload vehicle documents",
		zap.String("profileID", input.UserID),
		zap.String("vehicleID", input.VehicleID),
	)

	if input.VehicleID == "" {
		return nil, fileErrors.ErrorInvalidDocumentType
	}

	type fileUpload struct {
		file    serviceInterfaces.VehicleDocFileInput
		docType string
		docName string
	}

	timestamp := time.Now().UTC().Format("20060102_150405")
	lastName := sanitizeForDocName(input.LastName)
	firstName := sanitizeForDocName(input.FirstName)
	prefix := fmt.Sprintf("%s_%s_%s", lastName, firstName, timestamp)

	uploads := []fileUpload{
		{file: input.DriverLicence, docType: "driverLicence", docName: prefix + "_driver_licence"},
		{file: input.Assurance, docType: "insurance", docName: prefix + "_assurance"},
		{file: input.RegistrationCard, docType: "registrationCard", docName: prefix + "_vehicle_registration"},
	}

	created := make([]*serviceInterfaces.UploadedDocument, 0, len(uploads))
	for _, u := range uploads {
		if len(u.file.Data) == 0 {
			s.logger.Error("upload vehicle documents: fichier manquant",
				zap.String("vehicleID", input.VehicleID),
				zap.String("docName", u.docName),
			)
			return nil, fileErrors.ErrorInvalidDocumentType
		}

		// Valider les métadonnées obligatoires par type
		if u.file.DocumentNumber == "" || u.file.IssuedAt == "" || u.file.IssuingAuthority == "" {
			s.logger.Error("upload vehicle documents: métadonnées légales manquantes",
				zap.String("vehicleID", input.VehicleID),
				zap.String("docType", u.docType),
			)
			return nil, fileErrors.ErrorMissingDocumentMetadata
		}
		// expire_at obligatoire pour driverLicence et insurance, optionnel pour registrationCard
		if u.docType != "registrationCard" && u.file.ExpireAt == "" {
			s.logger.Error("upload vehicle documents: expire_at obligatoire",
				zap.String("vehicleID", input.VehicleID),
				zap.String("docType", u.docType),
			)
			return nil, fileErrors.ErrorMissingDocumentMetadata
		}

		issuedAt, err := time.Parse(time.RFC3339, u.file.IssuedAt)
		if err != nil {
			s.logger.Error("upload vehicle documents: issued_at invalide", zap.String("value", u.file.IssuedAt), zap.Error(err))
			return nil, fileErrors.ErrorMissingDocumentMetadata
		}
		var expireAtPtr *time.Time
		if u.file.ExpireAt != "" {
			expireAt, parseErr := time.Parse(time.RFC3339, u.file.ExpireAt)
			if parseErr != nil {
				s.logger.Error("upload vehicle documents: expire_at invalide", zap.String("value", u.file.ExpireAt), zap.Error(parseErr))
				return nil, fileErrors.ErrorMissingDocumentMetadata
			}
			expireAtPtr = &expireAt
		}

		mimeType := detectMimeType(u.file.Data)
		if !allowedMimeTypes[mimeType] {
			s.logger.Error("upload vehicle documents: unsupported mime type",
				zap.String("vehicleID", input.VehicleID),
				zap.String("docName", u.docName),
				zap.String("detectedMime", mimeType),
			)
			return nil, fileErrors.ErrorInvalidMimeType
		}

		doc, err := s.UploadVehicleDocument(ctx, serviceInterfaces.UploadVehicleDocumentInput{
			UserID:           input.UserID,
			VehicleID:        input.VehicleID,
			DocumentName:     u.docName,
			DocumentType:     u.docType,
			MimeType:         mimeType,
			FileSizeBytes:    int64(len(u.file.Data)),
			Data:             bytes.NewReader(u.file.Data),
			DocumentNumber:   u.file.DocumentNumber,
			IssuedAt:         &issuedAt,
			ExpireAt:         expireAtPtr,
			IssuingAuthority: u.file.IssuingAuthority,
		})
		if err != nil {
			s.logger.Error("upload vehicle documents: file upload failed",
				zap.String("vehicleID", input.VehicleID),
				zap.String("docName", u.docName),
				zap.Error(err),
			)
			return nil, err
		}
		s.logger.Info("vehicle document file uploaded",
			zap.String("vehicleID", input.VehicleID),
			zap.String("docType", u.docType),
			zap.String("documentID", doc.DocumentID),
		)
		presignedURL, _ := s.storage.GeneratePresignedURL(ctx, doc.DocumentKey, time.Hour)
		created = append(created, &serviceInterfaces.UploadedDocument{
			DocumentID:   doc.DocumentID,
			DocumentURL:  presignedURL,
			DocumentType: doc.DocumentType,
			DocumentName: doc.DocumentName,
		})
	}

	return created, nil
}

// sanitizeForDocName normalise un nom ou prénom pour composer un docName safe :
// - décompose les accents (NFD) puis strippe les diacritiques (é → e, ç → c, …)
// - lowercase
// - remplace espaces et apostrophes par des tirets
// - garde uniquement [a-z0-9-]
// Retourne "x" si le résultat est vide, pour éviter les tokens vides dans le docName.
func sanitizeForDocName(s string) string {
	decomposed := norm.NFD.String(s)
	var b strings.Builder
	for _, r := range decomposed {
		if unicode.Is(unicode.Mn, r) {
			continue // strip combining marks (accents)
		}
		switch {
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r + 32)
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '\'' || r == '-' || r == '_':
			b.WriteRune('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "x"
	}
	return out
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

// DeleteAllUserFiles supprime tous les documents et fichiers S3/MinIO d'un utilisateur.
func (s *fileServiceImpl) DeleteAllUserFiles(ctx context.Context, userID string) error {
	if userID == "" {
		return fileErrors.ErrorInvalidInput
	}

	// Supprimer user_documents
	userKeys, err := s.userDocWrite.DeleteAllByUserID(ctx, userID)
	if err != nil {
		s.logger.Error("DeleteAllUserFiles: delete user docs failed", zap.Error(err), zap.String("userID", userID))
		return err
	}
	for _, key := range userKeys {
		if delErr := s.storage.Delete(ctx, key); delErr != nil {
			s.logger.Warn("DeleteAllUserFiles: S3 delete failed (user doc)", zap.String("key", key), zap.Error(delErr))
		}
	}

	// Supprimer vehicle_documents
	vehicleKeys, err := s.vehicleDocWrite.DeleteAllByUserID(ctx, userID)
	if err != nil {
		s.logger.Error("DeleteAllUserFiles: delete vehicle docs failed", zap.Error(err), zap.String("userID", userID))
		return err
	}
	for _, key := range vehicleKeys {
		if delErr := s.storage.Delete(ctx, key); delErr != nil {
			s.logger.Warn("DeleteAllUserFiles: S3 delete failed (vehicle doc)", zap.String("key", key), zap.Error(delErr))
		}
	}

	return nil
}
