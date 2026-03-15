package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"go.uber.org/zap"

	"github.com/Kpeewu/tissi-mah/services/kyc-service/internal/client"
	"github.com/Kpeewu/tissi-mah/services/kyc-service/internal/domain"
	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/kyc-service/internal/service/interfaces"
	kycErrors "github.com/Kpeewu/tissi-mah/services/kyc-service/pkg/errors"
)

// PersonaTemplateID est le template Persona utilisé pour les inquiries.
// Injecté via le constructeur pour faciliter les tests.
type kycServiceImpl struct {
	fileClient        client.FileServiceClient
	personaClient     client.PersonaClient
	personaTemplateID string
	webhookSecret     string
	logger            *zap.Logger
}

// NewKYCService crée une nouvelle instance du service KYC
func NewKYCService(
	fileClient client.FileServiceClient,
	personaClient client.PersonaClient,
	personaTemplateID string,
	webhookSecret string,
	logger *zap.Logger,
) serviceInterfaces.KYCService {
	return &kycServiceImpl{
		fileClient:        fileClient,
		personaClient:     personaClient,
		personaTemplateID: personaTemplateID,
		webhookSecret:     webhookSecret,
		logger:            logger,
	}
}

// =============================================================================
// CreateInquiry
// =============================================================================

func (s *kycServiceImpl) CreateInquiry(ctx context.Context, input serviceInterfaces.CreateInquiryInput) (*serviceInterfaces.CreateInquiryResult, error) {
	s.logger.Debug("create inquiry",
		zap.String("userID", input.UserID),
		zap.String("documentType", input.DocumentType),
		zap.String("vehicleID", input.VehicleID),
	)

	// Validation des champs requis
	if input.UserID == "" {
		s.logger.Error("user_id is required")
		return nil, kycErrors.ErrorMissingUserID
	}
	if input.DocumentType == "" {
		s.logger.Error("document_type is required")
		return nil, kycErrors.ErrorMissingDocumentType
	}

	// Récupérer les revues existantes de l'utilisateur
	existingReviews, err := s.fileClient.GetDocumentReviewsByUserID(ctx, input.UserID)
	if err != nil {
		s.logger.Error("failed to get existing reviews", zap.Error(err))
		return nil, kycErrors.ErrorFileServiceUnavailable
	}

	// Vérifier qu'il n'existe pas de review active (pending ou inProgress)
	for _, review := range existingReviews {
		if domain.IsActiveStatus(review.Status) {
			s.logger.Warn("active review already exists",
				zap.String("userID", input.UserID),
				zap.String("existingReviewID", review.ReviewID),
				zap.String("existingStatus", review.Status),
			)
			return nil, kycErrors.ErrorInquiryAlreadyActive
		}
	}

	// Déterminer le document de référence et le previous_review_id
	var userDocumentID string
	var vehicleDocumentID string
	var previousReviewID string
	var attemptNumber int32 = 1

	if input.VehicleID != "" {
		// Document véhicule
		docs, err := s.fileClient.GetVehicleDocuments(ctx, input.VehicleID)
		if err != nil {
			s.logger.Error("failed to get vehicle documents", zap.Error(err))
			return nil, kycErrors.ErrorFileServiceUnavailable
		}
		// Trouver le document correspondant au type demandé
		for _, doc := range docs {
			if doc.DocumentType == input.DocumentType {
				vehicleDocumentID = doc.DocumentID
				break
			}
		}
		if vehicleDocumentID == "" {
			s.logger.Error("vehicle document not found for type", zap.String("type", input.DocumentType))
			return nil, kycErrors.ErrorReviewNotFound
		}
	} else {
		// Document utilisateur
		doc, err := s.fileClient.GetCurrentUserDocument(ctx, input.UserID, input.DocumentType)
		if err != nil {
			s.logger.Error("failed to get current user document", zap.Error(err))
			return nil, kycErrors.ErrorFileServiceUnavailable
		}
		userDocumentID = doc.DocumentID
	}

	// Calculer l'attempt_number et le previous_review_id à partir des revues existantes
	for _, review := range existingReviews {
		matchesDoc := (userDocumentID != "" && review.UserDocumentID == userDocumentID) ||
			(vehicleDocumentID != "" && review.VehicleDocumentID == vehicleDocumentID)
		if matchesDoc && review.AttemptNumber >= attemptNumber {
			attemptNumber = review.AttemptNumber + 1
			previousReviewID = review.ReviewID
		}
	}

	// Appeler l'API Persona pour créer l'inquiry
	referenceID := input.UserID
	personaInquiry, err := s.personaClient.CreateInquiry(ctx, s.personaTemplateID, referenceID)
	if err != nil {
		s.logger.Error("failed to create persona inquiry", zap.Error(err))
		return nil, kycErrors.ErrorPersonaUnavailable
	}

	// Créer la review dans le file-service
	now := time.Now().UTC()
	review := &domain.Review{
		UserDocumentID:    userDocumentID,
		VehicleDocumentID: vehicleDocumentID,

		PersonaInquiryID:    personaInquiry.InquiryID,
		PersonaTemplateID:   personaInquiry.TemplateID,
		PersonaSessionToken: personaInquiry.SessionToken,
		SessionExpiresAt:    &personaInquiry.ExpiresAt,

		AttemptNumber:    attemptNumber,
		PreviousReviewID: previousReviewID,

		Status:     "pending",
		ReviewType: "automatic",

		CreatedAt: now,
		UpdatedAt: now,
	}

	createdReview, err := s.fileClient.CreateDocumentReview(ctx, review)
	if err != nil {
		s.logger.Error("failed to create document review", zap.Error(err))
		return nil, kycErrors.ErrorFileServiceUnavailable
	}

	s.logger.Info("inquiry created",
		zap.String("reviewID", createdReview.ReviewID),
		zap.String("personaInquiryID", personaInquiry.InquiryID),
		zap.Int32("attemptNumber", attemptNumber),
	)

	return &serviceInterfaces.CreateInquiryResult{
		ReviewID:          createdReview.ReviewID,
		PersonaInquiryID:  personaInquiry.InquiryID,
		PersonaTemplateID: personaInquiry.TemplateID,
		SessionToken:      personaInquiry.SessionToken,
		SessionExpiresAt:  personaInquiry.ExpiresAt.Format(time.RFC3339),
		Status:            "pending",
		AttemptNumber:     attemptNumber,
		CreatedAt:         createdReview.CreatedAt.Format(time.RFC3339),
	}, nil
}

// =============================================================================
// Stubs — seront implémentés dans les prochaines étapes
// =============================================================================

// =============================================================================
// GetInquiry
// =============================================================================

func (s *kycServiceImpl) GetInquiry(ctx context.Context, userID string, personaInquiryID string) (*serviceInterfaces.InquiryDetail, error) {
	s.logger.Debug("get inquiry",
		zap.String("userID", userID),
		zap.String("personaInquiryID", personaInquiryID),
	)

	if userID == "" {
		s.logger.Error("user_id is required")
		return nil, kycErrors.ErrorMissingUserID
	}
	if personaInquiryID == "" {
		s.logger.Error("persona_inquiry_id is required")
		return nil, kycErrors.ErrorMissingInquiryID
	}

	// Récupérer la review par persona_inquiry_id
	review, err := s.fileClient.GetDocumentReviewByPersonaInquiryID(ctx, personaInquiryID)
	if err != nil {
		s.logger.Error("failed to get review by persona_inquiry_id", zap.Error(err))
		return nil, kycErrors.ErrorInquiryNotFound
	}

	// Vérifier que la review appartient bien à l'utilisateur
	ownershipValid := false
	if review.UserDocumentID != "" {
		// Récupérer les reviews de l'utilisateur pour vérifier la propriété
		reviews, err := s.fileClient.GetDocumentReviewsByUserID(ctx, userID)
		if err != nil {
			s.logger.Error("failed to verify ownership", zap.Error(err))
			return nil, kycErrors.ErrorFileServiceUnavailable
		}
		for _, r := range reviews {
			if r.ReviewID == review.ReviewID {
				ownershipValid = true
				break
			}
		}
	}
	if review.VehicleDocumentID != "" && !ownershipValid {
		reviews, err := s.fileClient.GetDocumentReviewsByUserID(ctx, userID)
		if err != nil {
			s.logger.Error("failed to verify ownership", zap.Error(err))
			return nil, kycErrors.ErrorFileServiceUnavailable
		}
		for _, r := range reviews {
			if r.ReviewID == review.ReviewID {
				ownershipValid = true
				break
			}
		}
	}

	if !ownershipValid {
		s.logger.Warn("unauthorized access to inquiry",
			zap.String("userID", userID),
			zap.String("personaInquiryID", personaInquiryID),
		)
		return nil, kycErrors.ErrorUnauthorized
	}

	detail := &serviceInterfaces.InquiryDetail{
		ReviewID:          review.ReviewID,
		PersonaInquiryID:  review.PersonaInquiryID,
		UserDocumentID:    review.UserDocumentID,
		PersonaTemplateID: review.PersonaTemplateID,
		VehicleDocumentID: review.VehicleDocumentID,
		Status:            review.Status,
		Decision:          review.Decision,
		ReasonRejection:   review.ReasonRejection,
		RejectionDetails:  review.RejectionDetails,
		AttemptNumber:     review.AttemptNumber,
		PreviousReviewID:  review.PreviousReviewID,
		ReviewType:        review.ReviewType,
		CreatedAt:         review.CreatedAt.Format(time.RFC3339),
		UpdatedAt:         review.UpdatedAt.Format(time.RFC3339),
	}

	if review.ReviewedAt != nil {
		detail.ReviewedAt = review.ReviewedAt.Format(time.RFC3339)
	}

	s.logger.Info("inquiry retrieved",
		zap.String("reviewID", review.ReviewID),
		zap.String("status", review.Status),
	)
	return detail, nil
}

// =============================================================================
// GetKYCStatus
// =============================================================================

// Types de documents d'identité
var identityDocumentTypes = map[string]bool{
	"idCardFront": true,
	"idCardBack":  true,
	"passport":    true,
}

// Types de documents de permis de conduire
var driverDocumentTypes = map[string]bool{
	"driverLicenceFront": true,
	"driverLicenceBack":  true,
}

func (s *kycServiceImpl) GetKYCStatus(ctx context.Context, userID string) (*serviceInterfaces.KYCStatus, error) {
	s.logger.Debug("get kyc status", zap.String("userID", userID))

	if userID == "" {
		s.logger.Error("user_id is required")
		return nil, kycErrors.ErrorMissingUserID
	}

	reviews, err := s.fileClient.GetDocumentReviewsByUserID(ctx, userID)
	if err != nil {
		s.logger.Error("failed to get reviews for kyc status", zap.Error(err))
		return nil, kycErrors.ErrorFileServiceUnavailable
	}

	identityVerified := false
	driverVerified := false
	var pendingReviews []*domain.PendingReview
	var latestRejection *domain.LatestRejection

	// Mapper document_id → document_type via le file-service
	docTypeByID := make(map[string]string)
	userDocs, err := s.fileClient.GetUserDocuments(ctx, userID)
	if err == nil {
		for _, doc := range userDocs {
			docTypeByID[doc.DocumentID] = doc.DocumentType
		}
	}

	// Analyser les reviews
	for _, review := range reviews {
		// Pending reviews (pending, inProgress, submitted)
		if review.Status == "pending" || review.Status == "inProgress" || review.Status == "submitted" {
			pendingReviews = append(pendingReviews, &domain.PendingReview{
				ReviewID:         review.ReviewID,
				PersonaInquiryID: review.PersonaInquiryID,
				Status:           review.Status,
				AttemptNumber:    review.AttemptNumber,
				SessionExpiresAt: review.SessionExpiresAt,
			})
		}

		// Identity verified : approved sur un document d'identité
		if review.Decision == "approved" && review.UserDocumentID != "" {
			docType := docTypeByID[review.UserDocumentID]
			if identityDocumentTypes[docType] {
				identityVerified = true
			}
		}

		// Driver verified : approved sur un document permis (si identity déjà vérifiée)
		if review.Decision == "approved" && review.UserDocumentID != "" {
			docType := docTypeByID[review.UserDocumentID]
			if driverDocumentTypes[docType] {
				driverVerified = true
			}
		}

		// Latest rejection : la plus récente par reviewed_at
		if review.Decision == "rejected" && review.ReviewedAt != nil {
			if latestRejection == nil || (review.ReviewedAt.After(*latestRejection.ReviewedAt)) {
				latestRejection = &domain.LatestRejection{
					ReviewID:         review.ReviewID,
					ReasonRejection:  review.ReasonRejection,
					RejectionDetails: review.RejectionDetails,
					ReviewType:       review.ReviewType,
					ReviewedAt:       review.ReviewedAt,
				}
			}
		}
	}

	// driver_verified nécessite identity_verified
	if !identityVerified {
		driverVerified = false
	}

	s.logger.Info("kyc status retrieved",
		zap.String("userID", userID),
		zap.Bool("identityVerified", identityVerified),
		zap.Bool("driverVerified", driverVerified),
		zap.Int("pendingCount", len(pendingReviews)),
	)

	return &serviceInterfaces.KYCStatus{
		IdentityVerified: identityVerified,
		DriverVerified:   driverVerified,
		PendingReviews:   pendingReviews,
		LatestRejection:  latestRejection,
	}, nil
}

// =============================================================================
// ResumeInquiry
// =============================================================================

// Statuts terminaux : la session ne peut pas être reprise
var nonResumableStatuses = map[string]bool{
	"completed": true,
	"expired":   true,
	"failed":    true,
}

func (s *kycServiceImpl) ResumeInquiry(ctx context.Context, userID string, personaInquiryID string) (*serviceInterfaces.ResumeResult, error) {
	s.logger.Debug("resume inquiry",
		zap.String("userID", userID),
		zap.String("personaInquiryID", personaInquiryID),
	)

	if userID == "" {
		s.logger.Error("user_id is required")
		return nil, kycErrors.ErrorMissingUserID
	}
	if personaInquiryID == "" {
		s.logger.Error("persona_inquiry_id is required")
		return nil, kycErrors.ErrorMissingInquiryID
	}

	// Récupérer la review par persona_inquiry_id
	review, err := s.fileClient.GetDocumentReviewByPersonaInquiryID(ctx, personaInquiryID)
	if err != nil {
		s.logger.Error("failed to get review by persona_inquiry_id", zap.Error(err))
		return nil, kycErrors.ErrorInquiryNotFound
	}

	// Vérifier l'ownership
	ownershipValid := false
	reviews, err := s.fileClient.GetDocumentReviewsByUserID(ctx, userID)
	if err != nil {
		s.logger.Error("failed to verify ownership", zap.Error(err))
		return nil, kycErrors.ErrorFileServiceUnavailable
	}
	for _, r := range reviews {
		if r.ReviewID == review.ReviewID {
			ownershipValid = true
			break
		}
	}
	if !ownershipValid {
		s.logger.Warn("unauthorized access to inquiry",
			zap.String("userID", userID),
			zap.String("personaInquiryID", personaInquiryID),
		)
		return nil, kycErrors.ErrorUnauthorized
	}

	// Vérifier que le statut permet une reprise
	if nonResumableStatuses[review.Status] {
		s.logger.Warn("inquiry not resumable",
			zap.String("status", review.Status),
			zap.String("personaInquiryID", personaInquiryID),
		)
		return nil, kycErrors.ErrorInquiryNotResumable
	}

	// Renouveler le session token via Persona
	session, err := s.personaClient.ResumeInquiry(ctx, personaInquiryID)
	if err != nil {
		s.logger.Error("failed to resume persona inquiry", zap.Error(err))
		return nil, kycErrors.ErrorPersonaUnavailable
	}

	// Mettre à jour la review dans le file-service
	review.PersonaSessionToken = session.SessionToken
	review.SessionExpiresAt = &session.ExpiresAt
	review.Status = "inProgress"
	review.UpdatedAt = time.Now().UTC()

	updatedReview, err := s.fileClient.UpdateDocumentReview(ctx, review)
	if err != nil {
		s.logger.Error("failed to update review", zap.Error(err))
		return nil, kycErrors.ErrorFileServiceUnavailable
	}

	s.logger.Info("inquiry resumed",
		zap.String("reviewID", updatedReview.ReviewID),
		zap.String("personaInquiryID", personaInquiryID),
	)

	return &serviceInterfaces.ResumeResult{
		ReviewID:         updatedReview.ReviewID,
		PersonaInquiryID: updatedReview.PersonaInquiryID,
		SessionToken:     session.SessionToken,
		SessionExpiresAt: session.ExpiresAt.Format(time.RFC3339),
		Status:           updatedReview.Status,
		AttemptNumber:    updatedReview.AttemptNumber,
	}, nil
}

// =============================================================================
// ProcessWebhook
// =============================================================================

// webhookStatusMapping associe les événements Persona aux statuts de review
var webhookStatusMapping = map[string]string{
	"inquiry.started":   "inProgress",
	"inquiry.submitted": "submitted",
	"inquiry.completed": "completed",
	"inquiry.approved":  "completed",
	"inquiry.declined":  "completed",
	"inquiry.expired":   "expired",
	"inquiry.failed":    "failed",
}

// webhookDecisionMapping associe les événements Persona aux décisions
var webhookDecisionMapping = map[string]string{
	"inquiry.approved": "approved",
	"inquiry.declined": "rejected",
}

func (s *kycServiceImpl) ProcessWebhook(ctx context.Context, input serviceInterfaces.WebhookInput) error {
	s.logger.Debug("process webhook",
		zap.String("eventType", input.WebhookEventType),
		zap.String("personaInquiryID", input.PersonaInquiryID),
	)

	// Valider la signature HMAC-SHA256
	if !s.verifyWebhookSignature(input.Signature, input.PersonaRawPayload) {
		s.logger.Warn("invalid webhook signature")
		return kycErrors.ErrorInvalidWebhookSignature
	}

	if input.PersonaInquiryID == "" {
		s.logger.Error("persona_inquiry_id is required in webhook")
		return kycErrors.ErrorMissingInquiryID
	}

	// Vérifier que l'événement est connu
	newStatus, known := webhookStatusMapping[input.WebhookEventType]
	if !known {
		s.logger.Warn("unknown webhook event type, ignoring",
			zap.String("eventType", input.WebhookEventType),
		)
		return nil // Ignorer les événements inconnus sans erreur
	}

	// Récupérer la review
	review, err := s.fileClient.GetDocumentReviewByPersonaInquiryID(ctx, input.PersonaInquiryID)
	if err != nil {
		s.logger.Error("failed to get review for webhook", zap.Error(err))
		return kycErrors.ErrorInquiryNotFound
	}

	// Mettre à jour la review
	review.Status = newStatus
	review.WebhookEventType = input.WebhookEventType
	now := time.Now().UTC()
	review.WebhookReceivedAt = &now
	review.PersonaRawPayload = json.RawMessage(input.PersonaRawPayload)
	review.UpdatedAt = now

	if decision, hasDecision := webhookDecisionMapping[input.WebhookEventType]; hasDecision {
		review.Decision = decision
		review.ReviewedAt = &now
	}

	if input.WebhookEventType == "inquiry.submitted" {
		review.SubmittedAt = &now
	}

	_, err = s.fileClient.UpdateDocumentReview(ctx, review)
	if err != nil {
		s.logger.Error("failed to update review from webhook", zap.Error(err))
		return kycErrors.ErrorFileServiceUnavailable
	}

	s.logger.Info("webhook processed",
		zap.String("reviewID", review.ReviewID),
		zap.String("eventType", input.WebhookEventType),
		zap.String("newStatus", newStatus),
	)

	return nil
}

// verifyWebhookSignature valide la signature HMAC-SHA256 du webhook
func (s *kycServiceImpl) verifyWebhookSignature(signature string, payload []byte) bool {
	mac := hmac.New(sha256.New, []byte(s.webhookSecret))
	mac.Write(payload)
	expectedMAC := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(signature), []byte(expectedMAC))
}

// =============================================================================
// GetAdminReviews
// =============================================================================

const adminPageSize int32 = 20

func (s *kycServiceImpl) GetAdminReviews(ctx context.Context, input serviceInterfaces.GetAdminReviewsInput) ([]*serviceInterfaces.AdminReviewItem, error) {
	s.logger.Debug("get admin reviews",
		zap.String("userID", input.UserID),
		zap.String("status", input.Status),
		zap.String("decision", input.Decision),
		zap.Int32("index", input.Index),
	)

	reviews, err := s.fileClient.ListDocumentReviews(ctx, input.UserID, input.Status, input.Decision, input.Index, adminPageSize)
	if err != nil {
		s.logger.Error("failed to list reviews", zap.Error(err))
		return nil, kycErrors.ErrorFileServiceUnavailable
	}

	items := make([]*serviceInterfaces.AdminReviewItem, 0, len(reviews))
	for _, review := range reviews {
		item := &serviceInterfaces.AdminReviewItem{
			ReviewID:          review.ReviewID,
			PersonaInquiryID:  review.PersonaInquiryID,
			UserDocumentID:    review.UserDocumentID,
			VehicleDocumentID: review.VehicleDocumentID,
			Status:            review.Status,
			Decision:          review.Decision,
			ReasonRejection:   review.ReasonRejection,
			RejectionDetails:  review.RejectionDetails,
			ReviewType:        review.ReviewType,
			AttemptNumber:     review.AttemptNumber,
			PreviousReviewID:  review.PreviousReviewID,
			WebhookEventType:  review.WebhookEventType,
			CreatedAt:         review.CreatedAt.Format(time.RFC3339),
			UpdatedAt:         review.UpdatedAt.Format(time.RFC3339),
		}
		if review.ReviewedAt != nil {
			item.ReviewedAt = review.ReviewedAt.Format(time.RFC3339)
		}
		if review.WebhookReceivedAt != nil {
			item.WebhookReceivedAt = review.WebhookReceivedAt.Format(time.RFC3339)
		}
		items = append(items, item)
	}

	s.logger.Info("admin reviews retrieved", zap.Int("count", len(items)))
	return items, nil
}

// =============================================================================
// GetAdminReview
// =============================================================================

func (s *kycServiceImpl) GetAdminReview(ctx context.Context, userID string, reviewID string) (*serviceInterfaces.AdminReviewDetail, error) {
	s.logger.Debug("get admin review",
		zap.String("userID", userID),
		zap.String("reviewID", reviewID),
	)

	if reviewID == "" {
		s.logger.Error("review_id is required")
		return nil, kycErrors.ErrorMissingReviewID
	}

	review, err := s.fileClient.GetDocumentReview(ctx, reviewID)
	if err != nil {
		s.logger.Error("failed to get review", zap.Error(err))
		return nil, kycErrors.ErrorReviewNotFound
	}

	detail := &serviceInterfaces.AdminReviewDetail{
		ReviewID:          review.ReviewID,
		PersonaInquiryID:  review.PersonaInquiryID,
		PersonaTemplateID: review.PersonaTemplateID,
		UserDocumentID:    review.UserDocumentID,
		VehicleDocumentID: review.VehicleDocumentID,
		Status:            review.Status,
		Decision:          review.Decision,
		ReasonRejection:   review.ReasonRejection,
		RejectionDetails:  review.RejectionDetails,
		ReviewedBy:        review.ReviewedBy,
		ReviewType:        review.ReviewType,
		Notes:             review.Notes,
		ExtractedData:     review.ExtractedData,
		WebhookEventType:  review.WebhookEventType,
		AttemptNumber:     review.AttemptNumber,
		PreviousReviewID:  review.PreviousReviewID,
		CreatedAt:         review.CreatedAt.Format(time.RFC3339),
		UpdatedAt:         review.UpdatedAt.Format(time.RFC3339),
	}
	if review.ReviewedAt != nil {
		detail.ReviewedAt = review.ReviewedAt.Format(time.RFC3339)
	}
	if review.WebhookReceivedAt != nil {
		detail.WebhookReceivedAt = review.WebhookReceivedAt.Format(time.RFC3339)
	}
	if review.SessionExpiresAt != nil {
		detail.SessionExpiresAt = review.SessionExpiresAt.Format(time.RFC3339)
	}

	s.logger.Info("admin review retrieved",
		zap.String("reviewID", review.ReviewID),
		zap.String("status", review.Status),
	)
	return detail, nil
}

// =============================================================================
// OverrideReview
// =============================================================================

func (s *kycServiceImpl) OverrideReview(ctx context.Context, input serviceInterfaces.OverrideReviewInput) (*serviceInterfaces.OverrideResult, error) {
	s.logger.Debug("override review",
		zap.String("userID", input.UserID),
		zap.String("reviewID", input.ReviewID),
		zap.String("decision", input.Decision),
	)

	if input.UserID == "" {
		s.logger.Error("user_id is required")
		return nil, kycErrors.ErrorMissingUserID
	}
	if input.ReviewID == "" {
		s.logger.Error("review_id is required")
		return nil, kycErrors.ErrorMissingReviewID
	}
	if !domain.IsValidDecision(input.Decision) {
		s.logger.Error("invalid decision", zap.String("decision", input.Decision))
		return nil, kycErrors.ErrorInvalidDecision
	}
	if input.Decision == "rejected" && input.ReasonRejection == "" {
		s.logger.Error("reason_rejection is required when decision is rejected")
		return nil, kycErrors.ErrorInvalidDecision
	}

	// Récupérer la review
	review, err := s.fileClient.GetDocumentReview(ctx, input.ReviewID)
	if err != nil {
		s.logger.Error("failed to get review for override", zap.Error(err))
		return nil, kycErrors.ErrorReviewNotFound
	}

	// Seule une review completed peut être overridée
	if review.Status != "completed" {
		s.logger.Warn("review not overridable",
			zap.String("reviewID", input.ReviewID),
			zap.String("status", review.Status),
		)
		return nil, kycErrors.ErrorReviewNotOverridable
	}

	// Appliquer l'override
	now := time.Now().UTC()
	review.Decision = input.Decision
	review.ReasonRejection = input.ReasonRejection
	review.RejectionDetails = input.RejectionDetails
	review.Notes = input.Notes
	review.ReviewedBy = input.UserID
	review.ReviewType = "manual"
	review.ReviewedAt = &now
	review.UpdatedAt = now

	updatedReview, err := s.fileClient.UpdateDocumentReview(ctx, review)
	if err != nil {
		s.logger.Error("failed to update review for override", zap.Error(err))
		return nil, kycErrors.ErrorFileServiceUnavailable
	}

	s.logger.Info("review overridden",
		zap.String("reviewID", updatedReview.ReviewID),
		zap.String("decision", updatedReview.Decision),
		zap.String("reviewedBy", input.UserID),
	)

	return &serviceInterfaces.OverrideResult{
		ReviewID:         updatedReview.ReviewID,
		PersonaInquiryID: updatedReview.PersonaInquiryID,
		Decision:         updatedReview.Decision,
		ReasonRejection:  updatedReview.ReasonRejection,
		RejectionDetails: updatedReview.RejectionDetails,
		ReviewedBy:       updatedReview.ReviewedBy,
		ReviewType:       updatedReview.ReviewType,
		ReviewedAt:       updatedReview.ReviewedAt.Format(time.RFC3339),
		Notes:            updatedReview.Notes,
		UpdatedAt:        updatedReview.UpdatedAt.Format(time.RFC3339),
	}, nil
}
