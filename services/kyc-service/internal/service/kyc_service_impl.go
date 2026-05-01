package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/Kpeewu/tissi-mah/pkg/notification"
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
	userClient        client.UserClient
	personaTemplateID string
	webhookSecret     string
	notifRedis        *redis.Client
	logger            *zap.Logger
}

// NewKYCService crée une nouvelle instance du service KYC
func NewKYCService(
	fileClient client.FileServiceClient,
	personaClient client.PersonaClient,
	userClient client.UserClient,
	personaTemplateID string,
	webhookSecret string,
	notifRedis *redis.Client,
	logger *zap.Logger,
) serviceInterfaces.KYCService {
	return &kycServiceImpl{
		fileClient:        fileClient,
		personaClient:     personaClient,
		userClient:        userClient,
		personaTemplateID: personaTemplateID,
		webhookSecret:     webhookSecret,
		notifRedis:        notifRedis,
		logger:            logger,
	}
}

// resolveInternalUserID résout le Firebase UID reçu depuis l'api-gateway
// en UserID interne MongoDB via le user-service. Le file-service stocke les
// documents avec l'UserID interne, pas avec le Firebase UID.
func (s *kycServiceImpl) resolveInternalUserID(ctx context.Context, firebaseUID string) (string, error) {
	internalID, err := s.userClient.GetUserIDByFirebaseID(ctx, firebaseUID)
	if err != nil {
		s.logger.Error("failed to resolve firebaseUID to internal userID",
			zap.String("firebaseUID", firebaseUID),
			zap.Error(err),
		)
		return "", kycErrors.ErrorUserNotFound
	}
	return internalID, nil
}

// mapToFileDocumentType converts high-level KYC document type aliases to the
// actual types stored by file-service ("IDCard" is decomposed into front/back at upload).
func mapToFileDocumentType(docType string) string {
	switch docType {
	case "IDCard":
		return "idCardFront"
	case "DriverLicence":
		return "driverLicenceFront"
	case "Passport":
		return "passport"
	default:
		return docType // "passport", "idCardFront", "idCardBack", etc.
	}
}

// mapToPersonaKind convertit le type de document interne vers la valeur attendue
// par l'API Persona (POST /api/v1/government-id-documents).
// Cf. https://docs.withpersona.com/reference/create-a-government-id
func mapToPersonaKind(docType string) string {
	switch docType {
	case "IDCard":
		return "id_card"
	case "Passport":
		return "passport"
	case "DriverLicence":
		return "driver_license"
	default:
		return ""
	}
}

// =============================================================================
// CreateInquiry
// =============================================================================

func (s *kycServiceImpl) CreateInquiry(ctx context.Context, input serviceInterfaces.CreateInquiryInput) (*serviceInterfaces.CreateInquiryResult, error) {
	s.logger.Debug("create inquiry",
		zap.String("firebaseUID", input.UserID),
		zap.String("documentID", input.DocumentID),
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
	if input.DocumentID == "" {
		s.logger.Error("document_id is required")
		return nil, kycErrors.ErrorMissingDocumentID
	}

	// Résoudre le Firebase UID reçu en UserID interne MongoDB.
	// Le file-service stocke les documents avec l'UserID interne.
	internalUserID, err := s.resolveInternalUserID(ctx, input.UserID)
	if err != nil {
		return nil, err
	}

	// Récupérer les revues existantes de l'utilisateur
	existingReviews, err := s.fileClient.GetDocumentReviewsByUserID(ctx, internalUserID)
	if err != nil {
		s.logger.Error("failed to get existing reviews", zap.Error(err))
		return nil, kycErrors.ErrorFileServiceUnavailable
	}

	// Vérifier qu'il n'existe pas de review active (pending ou inProgress)
	for _, review := range existingReviews {
		if domain.IsActiveStatus(review.Status) {
			s.logger.Warn("active review already exists",
				zap.String("userID", internalUserID),
				zap.String("existingReviewID", review.ReviewID),
				zap.String("existingStatus", review.Status),
			)
			return nil, kycErrors.ErrorInquiryAlreadyActive
		}
	}

	// Récupérer le document par ID, vérifier qu'il existe et qu'il appartient à l'appelant.
	// Le DocumentType reçu est validé contre celui stocké en DB pour détecter les
	// requêtes incohérentes (ex : DocumentID d'un permis avec DocumentType "Passport").
	var userDocumentID string
	var vehicleDocumentID string
	var frontDocURL string // URL S3/MinIO du recto, pour soumission Persona
	var previousReviewID string
	var attemptNumber int32 = 1

	if input.VehicleID != "" {
		// Document véhicule : récupérer par ID + vérifier qu'il appartient au véhicule fourni.
		doc, err := s.fileClient.GetVehicleDocument(ctx, input.DocumentID)
		if err != nil {
			s.logger.Error("failed to get vehicle document by ID",
				zap.String("documentID", input.DocumentID),
				zap.Error(err),
			)
			return nil, kycErrors.ErrorFileServiceUnavailable
		}
		if doc.OwnerID != input.VehicleID {
			s.logger.Warn("vehicle document does not belong to the provided vehicleID",
				zap.String("documentID", input.DocumentID),
				zap.String("docVehicleID", doc.OwnerID),
				zap.String("requestedVehicleID", input.VehicleID),
			)
			return nil, kycErrors.ErrorUnauthorized
		}
		if doc.DocumentType != input.DocumentType {
			s.logger.Warn("vehicle document type mismatch",
				zap.String("documentID", input.DocumentID),
				zap.String("docType", doc.DocumentType),
				zap.String("requestedType", input.DocumentType),
			)
			return nil, kycErrors.ErrorDocumentMismatch
		}
		vehicleDocumentID = doc.DocumentID
	} else {
		// Document utilisateur : récupérer par ID + vérifier qu'il appartient à l'appelant.
		doc, err := s.fileClient.GetUserDocument(ctx, input.DocumentID)
		if err != nil {
			s.logger.Error("failed to get user document by ID",
				zap.String("documentID", input.DocumentID),
				zap.Error(err),
			)
			return nil, kycErrors.ErrorFileServiceUnavailable
		}
		if doc.OwnerID != internalUserID {
			s.logger.Warn("user document does not belong to the caller",
				zap.String("documentID", input.DocumentID),
				zap.String("docOwnerID", doc.OwnerID),
				zap.String("callerUserID", internalUserID),
			)
			return nil, kycErrors.ErrorUnauthorized
		}
		// Pour les types haut-niveau (IDCard, DriverLicence), le document est stocké
		// sous un sous-type concret côté file-service (idCardFront, driverLicenceFront).
		expectedFileType := mapToFileDocumentType(input.DocumentType)
		if doc.DocumentType != expectedFileType && doc.DocumentType != input.DocumentType {
			s.logger.Warn("user document type mismatch",
				zap.String("documentID", input.DocumentID),
				zap.String("docType", doc.DocumentType),
				zap.String("requestedType", input.DocumentType),
			)
			return nil, kycErrors.ErrorDocumentMismatch
		}
		userDocumentID = doc.DocumentID
		frontDocURL = doc.DocumentURL
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

	// Appeler l'API Persona pour créer l'inquiry.
	// Le referenceID stocké côté Persona est l'UserID interne MongoDB,
	// qui est relu dans ProcessWebhook pour publier les notifications.
	referenceID := internalUserID
	personaInquiry, err := s.personaClient.CreateInquiry(ctx, s.personaTemplateID, referenceID)
	if err != nil {
		s.logger.Error("failed to create persona inquiry", zap.Error(err))
		return nil, kycErrors.ErrorPersonaUnavailable
	}

	// Soumettre les URLs S3/MinIO du document à Persona pour éviter la re-capture
	// côté SDK Android. Uniquement pour les documents utilisateur (passport, IDCard,
	// DriverLicence) — pas pour les documents véhicule (insurance, registrationCard).
	personaKind := mapToPersonaKind(input.DocumentType)
	if personaKind != "" && frontDocURL != "" {
		backDocURL := ""
		if input.DocumentIDBack != "" {
			backDoc, backErr := s.fileClient.GetUserDocument(ctx, input.DocumentIDBack)
			if backErr != nil {
				s.logger.Warn("kyc: back document fetch failed, submitting front only",
					zap.String("documentIDBack", input.DocumentIDBack),
					zap.Error(backErr),
				)
			} else if backDoc.OwnerID != internalUserID {
				s.logger.Warn("kyc: back document does not belong to caller, skipping",
					zap.String("documentIDBack", input.DocumentIDBack),
				)
			} else {
				backDocURL = backDoc.DocumentURL
			}
		}
		// Graceful degradation : si SubmitGovernmentID échoue, on log et on continue.
		// L'inquiry est créée, le SDK Android pourra capturer en fallback.
		if subErr := s.personaClient.SubmitGovernmentID(ctx, personaInquiry.InquiryID, personaKind, frontDocURL, backDocURL); subErr != nil {
			s.logger.Warn("kyc: SubmitGovernmentID failed, fallback to SDK capture",
				zap.String("inquiryID", personaInquiry.InquiryID),
				zap.String("kind", personaKind),
				zap.Error(subErr),
			)
		}
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
		Decision:   "pending", // aucune décision Persona encore — mise à jour après webhook
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
		zap.String("firebaseUID", userID),
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

	// Résoudre le Firebase UID en UserID interne MongoDB.
	internalUserID, err := s.resolveInternalUserID(ctx, userID)
	if err != nil {
		return nil, err
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
		reviews, err := s.fileClient.GetDocumentReviewsByUserID(ctx, internalUserID)
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
		reviews, err := s.fileClient.GetDocumentReviewsByUserID(ctx, internalUserID)
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
			zap.String("userID", internalUserID),
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
	s.logger.Debug("get kyc status", zap.String("firebaseUID", userID))

	if userID == "" {
		s.logger.Error("user_id is required")
		return nil, kycErrors.ErrorMissingUserID
	}

	// Résoudre le Firebase UID en UserID interne MongoDB.
	internalUserID, err := s.resolveInternalUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	reviews, err := s.fileClient.GetDocumentReviewsByUserID(ctx, internalUserID)
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
	userDocs, err := s.fileClient.GetUserDocuments(ctx, internalUserID)
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
		zap.String("userID", internalUserID),
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
		zap.String("firebaseUID", userID),
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

	// Résoudre le Firebase UID en UserID interne MongoDB.
	internalUserID, err := s.resolveInternalUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Récupérer la review par persona_inquiry_id
	review, err := s.fileClient.GetDocumentReviewByPersonaInquiryID(ctx, personaInquiryID)
	if err != nil {
		s.logger.Error("failed to get review by persona_inquiry_id", zap.Error(err))
		return nil, kycErrors.ErrorInquiryNotFound
	}

	// Vérifier l'ownership
	ownershipValid := false
	reviews, err := s.fileClient.GetDocumentReviewsByUserID(ctx, internalUserID)
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
			zap.String("userID", internalUserID),
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

	// Notifier l'utilisateur pour les décisions finales (approuvé ou rejeté)
	if s.notifRedis != nil {
		var kycEventType, docEventType string
		switch input.WebhookEventType {
		case "inquiry.approved":
			kycEventType = notification.KycApproved
			docEventType = notification.DocumentValidated
		case "inquiry.declined":
			kycEventType = notification.KycRejected
			docEventType = notification.DocumentRejected
		}
		if kycEventType != "" {
			// Extraire le userID depuis le champ reference-id du payload Persona
			var personaPayload struct {
				Data struct {
					Attributes struct {
						ReferenceID string `json:"reference-id"`
					} `json:"attributes"`
				} `json:"data"`
			}
			var userID string
			if err := json.Unmarshal(input.PersonaRawPayload, &personaPayload); err == nil {
				userID = personaPayload.Data.Attributes.ReferenceID
			}
			if userID != "" {
				if pubErr := notification.Publish(ctx, s.notifRedis, notification.Event{
					EventType:     kycEventType,
					UserID:        userID,
					ReferenceID:   review.ReviewID,
					ReferenceType: notification.RefDocument,
				}); pubErr != nil {
					s.logger.Error("failed to publish KYC notification", zap.Error(pubErr))
				}
				if pubErr := notification.Publish(ctx, s.notifRedis, notification.Event{
					EventType:     docEventType,
					UserID:        userID,
					ReferenceID:   review.ReviewID,
					ReferenceType: notification.RefDocument,
				}); pubErr != nil {
					s.logger.Error("failed to publish document validation notification", zap.Error(pubErr))
				}
			}
		}
	}

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

	// TODO: publier DOCUMENT_VALIDATED / DOCUMENT_REJECTED pour le propriétaire du document.
	// Le review.UserDocumentID permet d'identifier le document, mais le file-service ne expose pas
	// encore de méthode GetDocumentOwnerByDocumentID. À implémenter quand cette méthode sera disponible.

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
