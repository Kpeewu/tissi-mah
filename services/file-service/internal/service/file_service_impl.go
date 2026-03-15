package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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

	// Chercher le document courant avant la création du nouveau (pour le remplacer ensuite)
	existing, _ := s.userDocRead.GetCurrentByUserIDAndType(ctx, input.UserID, input.DocumentType)

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

	// MarkAsReplaced APRÈS la création du nouveau doc pour satisfaire la contrainte FK
	if existing != nil {
		_ = s.userDocWrite.MarkAsReplaced(ctx, existing.DocumentID, documentID)
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

	s.logger.Info("get document: success", zap.String("fileID", input.FileID))
	return &serviceInterfaces.GetDocumentResult{
		FileID:   doc.DocumentID,
		FileURL:  doc.DocumentURL,
		FileType: doc.DocumentType,
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

	// Vérification : exactement un des deux document IDs doit être fourni
	if input.UserDocumentID == "" && input.VehicleDocumentID == "" {
		s.logger.Error("review must reference either a user document or a vehicle document")
		return nil, fileErrors.ErrorMissingDocumentReference
	}
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

	now := time.Now().UTC()
	review := &domain.DocumentReview{
		ReviewID:          reviewID,
		UserDocumentID:    userDocID,
		VehicleDocumentID: vehicleDocID,

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

// --- Remplacement de document ---

// ChangeDocument remplace le fichier d'un document utilisateur existant par un nouveau.
// Récupère le document existant, upload le nouveau fichier en S3, puis crée un nouvel
// enregistrement DB en marquant l'ancien comme remplacé.
func (s *fileServiceImpl) ChangeDocument(ctx context.Context, input serviceInterfaces.ChangeDocumentInput) error {
	s.logger.Debug("change document", zap.String("userID", input.UserID), zap.String("fileID", input.FileID))

	if len(input.NewDocument) == 0 {
		return fileErrors.ErrorInvalidDocumentType
	}

	existing, err := s.userDocRead.GetByID(ctx, input.FileID)
	if err != nil {
		s.logger.Error("change document: document not found", zap.Error(err), zap.String("fileID", input.FileID))
		return fileErrors.ErrorDocumentNotFound
	}

	if existing.UserID != input.UserID {
		s.logger.Error("change document: user mismatch",
			zap.String("expected", existing.UserID),
			zap.String("got", input.UserID),
		)
		return fileErrors.ErrorDocumentNotFound
	}

	mimeType := http.DetectContentType(input.NewDocument)
	if !allowedMimeTypes[mimeType] {
		mimeType = "image/jpeg"
	}

	_, err = s.UploadUserDocument(ctx, serviceInterfaces.UploadUserDocumentInput{
		UserID:         existing.UserID,
		DocumentName:   existing.DocumentName,
		DocumentType:   existing.DocumentType,
		MimeType:       mimeType,
		FileSizeBytes:  int64(len(input.NewDocument)),
		Data:           bytes.NewReader(input.NewDocument),
		DocumentNumber: existing.DocumentNumber,
		IssuingCountry: existing.IssuingCountry,
	})
	if err != nil {
		s.logger.Error("change document: upload failed", zap.Error(err), zap.String("fileID", input.FileID))
		return err
	}

	s.logger.Info("document changed", zap.String("fileID", input.FileID), zap.String("userID", input.UserID))
	return nil
}

// --- Upload identité ---

// UploadIdDocument upload les documents d'identité vers S3/MinIO et sauvegarde les URLs en base.
// Les fichiers sont uploadés individuellement via UploadUserDocument.
func (s *fileServiceImpl) UploadIdDocument(ctx context.Context, input serviceInterfaces.UploadIdDocumentInput) error {
	s.logger.Debug("upload id document", zap.String("profileID", input.UserID), zap.String("type", input.DocumentType))

	if input.UserID == "" {
		return fileErrors.ErrorInvalidDocumentType
	}

	// Détermine les fichiers requis selon le type de document
	type fileUpload struct {
		data     []byte
		docType  string
		docName  string
	}

	var uploads []fileUpload

	switch input.DocumentType {
	case "IDCard":
		if len(input.IDCardRecto) == 0 || len(input.IDCardVerso) == 0 {
			s.logger.Error("IDCard: recto et verso obligatoires", zap.String("profileID", input.UserID))
			return fileErrors.ErrorInvalidDocumentType
		}
		uploads = []fileUpload{
			{data: input.IDCardRecto, docType: "idCardFront", docName: "id_card_recto"},
			{data: input.IDCardVerso, docType: "idCardBack", docName: "id_card_verso"},
		}
	case "Passport":
		if len(input.Passport) == 0 {
			s.logger.Error("Passport: fichier obligatoire", zap.String("profileID", input.UserID))
			return fileErrors.ErrorInvalidDocumentType
		}
		uploads = []fileUpload{
			{data: input.Passport, docType: "passport", docName: "passport"},
		}
	case "DriverLicence":
		if len(input.DriverLicenceRecto) == 0 || len(input.DriverLicenceVerso) == 0 {
			s.logger.Error("DriverLicence: recto et verso obligatoires", zap.String("profileID", input.UserID))
			return fileErrors.ErrorInvalidDocumentType
		}
		uploads = []fileUpload{
			{data: input.DriverLicenceRecto, docType: "driverLicenceFront", docName: "driver_licence_recto"},
			{data: input.DriverLicenceVerso, docType: "driverLicenceBack", docName: "driver_licence_verso"},
		}
	default:
		s.logger.Error("type de document inconnu", zap.String("type", input.DocumentType))
		return fileErrors.ErrorInvalidDocumentType
	}

	// Upload chaque fichier vers S3 + DB
	for _, u := range uploads {
		mimeType := http.DetectContentType(u.data)
		if !allowedMimeTypes[mimeType] {
			mimeType = "image/jpeg"
		}

		_, err := s.UploadUserDocument(ctx, serviceInterfaces.UploadUserDocumentInput{
			UserID:        input.UserID,
			DocumentName:  u.docName,
			DocumentType:  u.docType,
			MimeType:      mimeType,
			FileSizeBytes: int64(len(u.data)),
			Data:          bytes.NewReader(u.data),
		})
		if err != nil {
			s.logger.Error("upload id document file failed",
				zap.String("profileID", input.UserID),
				zap.String("docType", u.docType),
				zap.Error(err),
			)
			return err
		}
		s.logger.Info("id document file uploaded", zap.String("profileID", input.UserID), zap.String("docType", u.docType))
	}

	return nil
}

// UploadVehicleDocuments upload le permis de conduire, l'assurance et la carte grise
// vers S3/MinIO et sauvegarde les URLs en base via UploadVehicleDocument.
func (s *fileServiceImpl) UploadVehicleDocuments(ctx context.Context, input serviceInterfaces.UploadVehicleDocumentsInput) error {
	s.logger.Debug("upload vehicle documents",
		zap.String("profileID", input.UserID),
		zap.String("vehicleID", input.VehicleID),
	)

	if input.VehicleID == "" {
		return fileErrors.ErrorInvalidDocumentType
	}

	type fileUpload struct {
		data     []byte
		docType  string
		docName  string
	}

	uploads := []fileUpload{
		{data: input.DriverLicenceImage, docType: "insurance", docName: "driver_licence"},
		{data: input.Assurance, docType: "insurance", docName: "assurance"},
		{data: input.VehicleRegistration, docType: "registrationCard", docName: "vehicle_registration"},
	}

	for _, u := range uploads {
		if len(u.data) == 0 {
			s.logger.Error("upload vehicle documents: fichier manquant",
				zap.String("vehicleID", input.VehicleID),
				zap.String("docName", u.docName),
			)
			return fileErrors.ErrorInvalidDocumentType
		}

		mimeType := http.DetectContentType(u.data)
		if !allowedMimeTypes[mimeType] {
			mimeType = "image/jpeg"
		}

		_, err := s.UploadVehicleDocument(ctx, serviceInterfaces.UploadVehicleDocumentInput{
			VehicleID:     input.VehicleID,
			DocumentName:  u.docName,
			DocumentType:  u.docType,
			MimeType:      mimeType,
			FileSizeBytes: int64(len(u.data)),
			Data:          bytes.NewReader(u.data),
		})
		if err != nil {
			s.logger.Error("upload vehicle documents: file upload failed",
				zap.String("vehicleID", input.VehicleID),
				zap.String("docName", u.docName),
				zap.Error(err),
			)
			return err
		}
		s.logger.Info("vehicle document file uploaded",
			zap.String("vehicleID", input.VehicleID),
			zap.String("docType", u.docType),
		)
	}

	return nil
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
