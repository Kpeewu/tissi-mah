package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strconv"
	"strings"
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
	if input.VehicleID != "" {
		s.logger.Error("vehicle documents must be validated manually", zap.String("vehicleID", input.VehicleID))
		return nil, kycErrors.ErrorVehicleDocumentNotAllowed
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

	// En Persona 100%, l'app n'upload plus la pièce d'identité localement.
	// DocumentID arrive vide dans la requête et le file-service n'a aucun document
	// utilisateur à récupérer — Persona collecte et stocke la pièce directement.
	// On normalise le type haut-niveau ("IDCard", "DriverLicence") vers le type
	// concret stocké côté file-service ("idCardFront", "driverLicenceFront"),
	// pour rester cohérent avec ValidateDocument (qui persiste doc.DocumentType
	// déjà au format file-service) et avec identityDocumentTypes/driverDocumentTypes.
	storedDocumentType := mapToFileDocumentType(input.DocumentType)

	var previousReviewID string
	var attemptNumber int32 = 1

	// Calculer l'attempt_number et le previous_review_id en regroupant par
	// DocumentType : une nouvelle inquiry sur un type donné incrémente la chaîne
	// des tentatives passées sur ce même type.
	for _, review := range existingReviews {
		if review.DocumentType == storedDocumentType && review.AttemptNumber >= attemptNumber {
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

	// Créer la review dans le file-service
	now := time.Now().UTC()
	review := &domain.Review{
		UserID:       internalUserID,
		DocumentType: storedDocumentType,

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
		CreatedAt:         now.Format(time.RFC3339),
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

	// Ownership : depuis migration 000008, user_id est dénormalisé directement
	// sur document_reviews — comparaison directe sans round-trip GetByUserID.
	if review.UserID != internalUserID {
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

	// Analyser les reviews — review.DocumentType est dénormalisé depuis la
	// migration 000008, plus besoin de joindre avec user_documents.
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

		if review.Decision == "approved" {
			if identityDocumentTypes[review.DocumentType] {
				identityVerified = true
			}
			if driverDocumentTypes[review.DocumentType] {
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

	// Ownership : depuis migration 000008, user_id est dénormalisé sur la review.
	if review.UserID != internalUserID {
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

// verifyWebhookSignature valide la signature HMAC-SHA256 du webhook Persona.
// Le header a la forme "t=<unix>,v1=<hex>[,v1=<hex2>...]".
// Le HMAC est calculé sur "<timestamp>.<body>" (spec Persona).
// Fenêtre anti-replay : 5 minutes.
func (s *kycServiceImpl) verifyWebhookSignature(signatureHeader string, payload []byte) bool {
	var timestamp string
	var signatures []string
	for _, part := range strings.Split(signatureHeader, ",") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "t":
			timestamp = kv[1]
		case "v1":
			signatures = append(signatures, kv[1])
		}
	}
	if timestamp == "" || len(signatures) == 0 {
		return false
	}

	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil || time.Since(time.Unix(ts, 0)) > 5*time.Minute {
		return false
	}

	mac := hmac.New(sha256.New, []byte(s.webhookSecret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))

	for _, sig := range signatures {
		if hmac.Equal([]byte(sig), []byte(expected)) {
			return true
		}
	}
	return false
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

// =============================================================================
// ValidateDocument
// =============================================================================

func (s *kycServiceImpl) ValidateDocument(ctx context.Context, input serviceInterfaces.ValidateDocumentInput) (*serviceInterfaces.ValidateDocumentResult, error) {
	s.logger.Debug("validate document manually",
		zap.String("supportAgentID", input.SupportAgentID),
		zap.String("documentID", input.DocumentID),
		zap.String("vehicleID", input.VehicleID),
		zap.String("decision", input.Decision),
	)

	if input.SupportAgentID == "" {
		s.logger.Error("support_agent_id is required")
		return nil, kycErrors.ErrorMissingUserID
	}
	if input.DocumentID == "" {
		s.logger.Error("document_id is required")
		return nil, kycErrors.ErrorMissingDocumentID
	}
	if !domain.IsValidDecision(input.Decision) {
		s.logger.Error("invalid decision", zap.String("decision", input.Decision))
		return nil, kycErrors.ErrorInvalidDecision
	}
	if input.Decision == "rejected" && input.ReasonRejection == "" {
		s.logger.Error("reason_rejection is required when decision is rejected")
		return nil, kycErrors.ErrorInvalidDecision
	}

	// Récupérer le document pour vérifier qu'il existe et lire user_id + type
	// (dénormalisés sur la review depuis migration 000008).
	var userDocumentID string
	var vehicleDocumentID string
	var ownerUserID string
	var documentType string

	if input.VehicleID != "" {
		doc, err := s.fileClient.GetVehicleDocument(ctx, input.DocumentID)
		if err != nil {
			s.logger.Error("failed to get vehicle document", zap.String("documentID", input.DocumentID), zap.Error(err))
			return nil, kycErrors.ErrorFileServiceUnavailable
		}
		if doc.OwnerID != input.VehicleID {
			s.logger.Warn("vehicle document does not belong to provided vehicleID",
				zap.String("documentID", input.DocumentID),
				zap.String("vehicleID", input.VehicleID),
			)
			return nil, kycErrors.ErrorDocumentMismatch
		}
		vehicleDocumentID = doc.DocumentID
		ownerUserID = doc.UserID // exposé par le proto VehicleDocumentResponse depuis migration 000005
		documentType = doc.DocumentType
	} else {
		doc, err := s.fileClient.GetUserDocument(ctx, input.DocumentID)
		if err != nil {
			s.logger.Error("failed to get user document", zap.String("documentID", input.DocumentID), zap.Error(err))
			return nil, kycErrors.ErrorFileServiceUnavailable
		}
		userDocumentID = doc.DocumentID
		ownerUserID = doc.OwnerID // pour user docs, OwnerID = user_id directement
		documentType = doc.DocumentType
	}

	// Auto-découverte du document compagnon pour les documents recto-verso
	var secondUserDocumentID string
	if userDocumentID != "" {
		if companionType := domain.CompanionDocumentType(documentType); companionType != "" {
			if companion, err := s.fileClient.GetCurrentUserDocument(ctx, ownerUserID, companionType); err == nil {
				secondUserDocumentID = companion.DocumentID
			}
		}
	}

	now := time.Now().UTC()
	review := &domain.Review{
		UserID:               ownerUserID,
		DocumentType:         documentType,
		LogicalDocumentType:  domain.ToLogicalDocumentType(documentType),
		PersonaInquiryID:     "",
		ReviewType:           "manual",
		Status:               "completed",
		Decision:             input.Decision,
		ReasonRejection:      input.ReasonRejection,
		RejectionDetails:     input.RejectionDetails,
		Notes:                input.Notes,
		ReviewedBy:           input.SupportAgentID,
		ReviewedAt:           &now,
		UserDocumentID:       userDocumentID,
		SecondUserDocumentID: secondUserDocumentID,
		VehicleDocumentID:    vehicleDocumentID,
	}

	created, err := s.fileClient.CreateDocumentReview(ctx, review)
	if err != nil {
		s.logger.Error("failed to create manual document review", zap.Error(err))
		return nil, kycErrors.ErrorFileServiceUnavailable
	}

	s.logger.Info("document validated manually",
		zap.String("reviewID", created.ReviewID),
		zap.String("decision", created.Decision),
		zap.String("supportAgentID", input.SupportAgentID),
	)

	return &serviceInterfaces.ValidateDocumentResult{
		ReviewID:   created.ReviewID,
		Decision:   created.Decision,
		ReviewedBy: created.ReviewedBy,
		ReviewType: created.ReviewType,
		ReviewedAt: created.ReviewedAt.Format(time.RFC3339),
		Notes:      created.Notes,
	}, nil
}

// =============================================================================
// GetManualReviewRequests — demandes de validation groupées par utilisateur
// =============================================================================

const defaultManualReviewPageSize int32 = 20

// userBucket agrège les statuts de documents d'un utilisateur par catégorie.
type userBucket struct {
	passengerStatuses []string
	driverStatuses    []string
	total             int32
}

func (s *kycServiceImpl) GetManualReviewRequests(ctx context.Context, input serviceInterfaces.GetManualReviewRequestsInput) (*serviceInterfaces.GetManualReviewRequestsResult, error) {
	s.logger.Debug("get manual review requests",
		zap.String("statusFilter", input.Status),
		zap.Int32("page", input.Page),
		zap.Int32("pageSize", input.PageSize),
	)

	// On récupère tous les documents KYC (tous statuts), le tri/filtre statut se fait
	// après agrégation pour que le statut par catégorie reste calculé sur l'ensemble.
	docs, err := s.fileClient.ListKycDocuments(ctx, nil)
	if err != nil {
		s.logger.Error("failed to list kyc documents", zap.Error(err))
		return nil, kycErrors.ErrorFileServiceUnavailable
	}

	buckets := make(map[string]*userBucket)
	for _, d := range docs {
		category := domain.DocumentCategory(d.DocumentType, d.OwnerKind)
		if category == domain.CategoryOther {
			continue
		}
		b := buckets[d.UserID]
		if b == nil {
			b = &userBucket{}
			buckets[d.UserID] = b
		}
		if category == domain.CategoryPassenger {
			b.passengerStatuses = append(b.passengerStatuses, d.Status)
		} else {
			b.driverStatuses = append(b.driverStatuses, d.Status)
		}
		b.total++
	}

	// Calcul des statuts agrégés + filtre statut optionnel.
	type entry struct {
		userID          string
		passengerStatus string
		driverStatus    string
		total           int32
	}
	entries := make([]entry, 0, len(buckets))
	for userID, b := range buckets {
		ps := domain.AggregateStatus(b.passengerStatuses)
		ds := domain.AggregateStatus(b.driverStatuses)
		if input.Status != "" && ps != input.Status && ds != input.Status {
			continue
		}
		entries = append(entries, entry{userID: userID, passengerStatus: ps, driverStatus: ds, total: b.total})
	}

	// Tri déterministe pour une pagination stable.
	sort.Slice(entries, func(i, j int) bool { return entries[i].userID < entries[j].userID })

	total := int32(len(entries))
	pageSize := input.PageSize
	if pageSize <= 0 {
		pageSize = defaultManualReviewPageSize
	}
	start := input.Page * pageSize
	if start < 0 || start >= total {
		return &serviceInterfaces.GetManualReviewRequestsResult{Requests: []*domain.ManualReviewRequest{}, Total: total}, nil
	}
	end := start + pageSize
	if end > total {
		end = total
	}

	requests := make([]*domain.ManualReviewRequest, 0, end-start)
	for _, e := range entries[start:end] {
		userInfo, err := s.userClient.GetUserByUserID(ctx, e.userID)
		if err != nil {
			// L'utilisateur peut avoir été supprimé : on remonte au moins l'ID.
			s.logger.Warn("failed to fetch user info for manual review request",
				zap.String("userID", e.userID), zap.Error(err))
			userInfo = &domain.UserInfo{UserID: e.userID}
		}
		requests = append(requests, &domain.ManualReviewRequest{
			User:            userInfo,
			PassengerStatus: e.passengerStatus,
			DriverStatus:    e.driverStatus,
			TotalDocuments:  e.total,
		})
	}

	s.logger.Info("manual review requests retrieved",
		zap.Int32("total", total), zap.Int("page", len(requests)))
	return &serviceInterfaces.GetManualReviewRequestsResult{Requests: requests, Total: total}, nil
}

// =============================================================================
// GetManualReviewRequestDetail — documents soumis d'un utilisateur + dernière review
// =============================================================================

func (s *kycServiceImpl) GetManualReviewRequestDetail(ctx context.Context, userID string) (*domain.ManualReviewRequestDetail, error) {
	s.logger.Debug("get manual review request detail", zap.String("userID", userID))

	if userID == "" {
		return nil, kycErrors.ErrorMissingUserID
	}

	userInfo, err := s.userClient.GetUserByUserID(ctx, userID)
	if err != nil {
		s.logger.Error("failed to fetch user info", zap.String("userID", userID), zap.Error(err))
		return nil, kycErrors.ErrorUserNotFound
	}

	userDocs, err := s.fileClient.GetUserDocumentSummaries(ctx, userID)
	if err != nil {
		s.logger.Error("failed to fetch user documents", zap.Error(err))
		return nil, kycErrors.ErrorFileServiceUnavailable
	}
	vehicleDocs, err := s.fileClient.GetVehicleDocumentSummariesByUserID(ctx, userID)
	if err != nil {
		s.logger.Error("failed to fetch vehicle documents", zap.Error(err))
		return nil, kycErrors.ErrorFileServiceUnavailable
	}
	reviews, err := s.fileClient.GetDocumentReviewsByUserID(ctx, userID)
	if err != nil {
		s.logger.Error("failed to fetch reviews", zap.Error(err))
		return nil, kycErrors.ErrorFileServiceUnavailable
	}

	byUserDoc, byVehicleDoc, byLogicalType, byType := indexLatestReviews(reviews)

	docs := make([]*domain.DocumentSummary, 0, len(userDocs)+len(vehicleDocs))
	for _, d := range userDocs {
		d.LogicalDocumentType = domain.ToLogicalDocumentType(d.DocumentType)
		d.LatestReview = pickReview(byUserDoc[d.DocumentID], byLogicalType[d.LogicalDocumentType], byType[d.DocumentType])
		docs = append(docs, d)
	}
	for _, d := range vehicleDocs {
		d.LogicalDocumentType = domain.ToLogicalDocumentType(d.DocumentType)
		d.LatestReview = pickReview(byVehicleDoc[d.DocumentID], byLogicalType[d.LogicalDocumentType], byType[d.DocumentType])
		docs = append(docs, d)
	}

	s.logger.Info("manual review request detail retrieved",
		zap.String("userID", userID), zap.Int("documents", len(docs)))
	return &domain.ManualReviewRequestDetail{User: userInfo, Documents: docs}, nil
}

// reviewTime retourne l'instant de référence d'une review (reviewed_at sinon updated_at).
func reviewTime(r *domain.Review) time.Time {
	if r.ReviewedAt != nil {
		return *r.ReviewedAt
	}
	return r.UpdatedAt
}

// indexLatestReviews indexe la dernière review par user_document_id, vehicle_document_id,
// logical_document_type et par document_type (fallback pour les anciennes revues).
func indexLatestReviews(reviews []*domain.Review) (byUserDoc, byVehicleDoc, byLogicalType, byType map[string]*domain.Review) {
	byUserDoc = make(map[string]*domain.Review)
	byVehicleDoc = make(map[string]*domain.Review)
	byLogicalType = make(map[string]*domain.Review)
	byType = make(map[string]*domain.Review)
	keepLatest := func(m map[string]*domain.Review, key string, r *domain.Review) {
		if key == "" {
			return
		}
		if cur, ok := m[key]; !ok || reviewTime(r).After(reviewTime(cur)) {
			m[key] = r
		}
	}
	for _, r := range reviews {
		keepLatest(byUserDoc, r.UserDocumentID, r)
		keepLatest(byVehicleDoc, r.VehicleDocumentID, r)
		keepLatest(byLogicalType, r.LogicalDocumentType, r)
		keepLatest(byType, r.DocumentType, r)
	}
	return byUserDoc, byVehicleDoc, byLogicalType, byType
}

// pickReview retourne le ReviewSummary de la review prioritaire :
// 1. match direct par document_id, 2. fallback par logical_document_type, 3. fallback par document_type.
func pickReview(direct, byLogical, byType *domain.Review) *domain.ReviewSummary {
	r := direct
	if r == nil {
		r = byLogical
	}
	if r == nil {
		r = byType
	}
	if r == nil {
		return nil
	}
	return &domain.ReviewSummary{
		ReviewID:         r.ReviewID,
		Status:           r.Status,
		Decision:         r.Decision,
		ReasonRejection:  r.ReasonRejection,
		RejectionDetails: r.RejectionDetails,
		ReviewType:       r.ReviewType,
		ReviewedBy:       r.ReviewedBy,
		ReviewedAt:       r.ReviewedAt,
	}
}

func (s *kycServiceImpl) GetDocumentHistory(ctx context.Context, userID string, logicalDocumentType string) ([]*domain.DocumentHistoryEntry, error) {
	s.logger.Debug("get document history",
		zap.String("userID", userID),
		zap.String("logicalDocumentType", logicalDocumentType),
	)

	if userID == "" || logicalDocumentType == "" {
		return nil, kycErrors.ErrorMissingUserID
	}

	reviews, err := s.fileClient.GetDocumentReviewHistory(ctx, userID, logicalDocumentType)
	if err != nil {
		s.logger.Error("failed to get document review history", zap.Error(err))
		return nil, kycErrors.ErrorFileServiceUnavailable
	}

	entries := make([]*domain.DocumentHistoryEntry, 0, len(reviews))
	for _, r := range reviews {
		entry := &domain.DocumentHistoryEntry{
			ReviewID:            r.ReviewID,
			Status:              r.Status,
			Decision:            r.Decision,
			ReasonRejection:     r.ReasonRejection,
			RejectionDetails:    r.RejectionDetails,
			Notes:               r.Notes,
			ReviewType:          r.ReviewType,
			ReviewedBy:          r.ReviewedBy,
			ReviewedAt:          r.ReviewedAt,
			AttemptNumber:       r.AttemptNumber,
			DocumentID:          r.UserDocumentID,
			SecondDocumentID:    r.SecondUserDocumentID,
			LogicalDocumentType: r.LogicalDocumentType,
			UpdatedAt:           r.UpdatedAt,
			CreatedAt:           r.CreatedAt,
		}
		entries = append(entries, entry)
	}
	return entries, nil
}
