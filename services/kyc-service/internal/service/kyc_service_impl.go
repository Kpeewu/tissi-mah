package service

import (
	"context"
	"errors"
	"sort"
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

type kycServiceImpl struct {
	fileClient    client.FileServiceClient
	userClient    client.UserClient
	supportClient client.SupportClient
	vehicleClient client.VehicleClient
	notifRedis    *redis.Client
	logger        *zap.Logger
}

// NewKYCService crée une nouvelle instance du service KYC.
// supportClient et vehicleClient peuvent être nil : l'enrichissement des reviews
// (prénom/nom des agents) et la propagation is_verified des véhicules sont alors
// désactivés (dégradation gracieuse).
func NewKYCService(
	fileClient client.FileServiceClient,
	userClient client.UserClient,
	supportClient client.SupportClient,
	vehicleClient client.VehicleClient,
	notifRedis *redis.Client,
	logger *zap.Logger,
) serviceInterfaces.KYCService {
	return &kycServiceImpl{
		fileClient:    fileClient,
		userClient:    userClient,
		supportClient: supportClient,
		vehicleClient: vehicleClient,
		notifRedis:    notifRedis,
		logger:        logger,
	}
}

// summariesToVerificationDocs projette des DocumentSummary (documents COURANTS)
// vers la vue minimale du calcul de vérification. profilePicture (héritage,
// hors KYC) est exclu.
func summariesToVerificationDocs(sums []*domain.DocumentSummary) []*domain.VerificationDocument {
	out := make([]*domain.VerificationDocument, 0, len(sums))
	for _, d := range sums {
		if d.DocumentType == "profilePicture" {
			continue
		}
		vd := &domain.VerificationDocument{
			DocumentType: d.DocumentType,
			Status:       d.Status,
		}
		if d.OwnerKind == "vehicle" {
			vd.VehicleID = d.OwnerID
		}
		out = append(out, vd)
	}
	return out
}

// loadVerificationDocs charge les documents courants (user + véhicule) d'un
// utilisateur pour le calcul de vérification.
func (s *kycServiceImpl) loadVerificationDocs(ctx context.Context, internalUserID string) (userDocs, vehicleDocs []*domain.VerificationDocument, err error) {
	userSums, err := s.fileClient.GetUserDocumentSummaries(ctx, internalUserID)
	if err != nil {
		return nil, nil, err
	}
	vehicleSums, err := s.fileClient.GetVehicleDocumentSummariesByUserID(ctx, internalUserID)
	if err != nil {
		return nil, nil, err
	}
	return summariesToVerificationDocs(userSums), summariesToVerificationDocs(vehicleSums), nil
}

// resolveSupportAgents résout un ensemble d'UID d'agents support en leurs infos
// (prénom/nom) via support-service. Déduplique les UID et tolère les erreurs
// individuelles (dégradation gracieuse). No-op si supportClient est nil.
func (s *kycServiceImpl) resolveSupportAgents(ctx context.Context, uids []string) map[string]*domain.SupportAgent {
	result := make(map[string]*domain.SupportAgent)
	if s.supportClient == nil {
		return result
	}
	seen := make(map[string]bool)
	for _, uid := range uids {
		if uid == "" || seen[uid] {
			continue
		}
		seen[uid] = true
		agent, err := s.supportClient.GetSupportUserByID(ctx, uid)
		if err != nil {
			s.logger.Warn("failed to resolve support agent", zap.String("uid", uid), zap.Error(err))
			continue
		}
		result[uid] = agent
	}
	return result
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

// =============================================================================
// GetKYCStatus
// =============================================================================

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

	var pendingReviews []*domain.PendingReview
	var latestRejection *domain.LatestRejection

	// Analyser les reviews — review.DocumentType est dénormalisé depuis la
	// migration 000008, plus besoin de joindre avec user_documents.
	for _, review := range reviews {
		// Pending reviews (pending + statuts hérités inProgress/submitted)
		if review.Status == "pending" || review.Status == "inProgress" || review.Status == "submitted" {
			pendingReviews = append(pendingReviews, &domain.PendingReview{
				ReviewID:      review.ReviewID,
				Status:        review.Status,
				AttemptNumber: review.AttemptNumber,
				DocumentType:  review.DocumentType,
			})
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

	// Vérification identité (passager) / conducteur — calculée depuis les documents
	// COURANTS (selfie + pièce ; conducteur = selfie + permis + ≥1 véhicule validé).
	userDocs, vehicleDocs, err := s.loadVerificationDocs(ctx, internalUserID)
	if err != nil {
		s.logger.Error("failed to load documents for kyc status", zap.Error(err))
		return nil, kycErrors.ErrorFileServiceUnavailable
	}
	identityVerified, driverVerified := domain.ComputeProfileVerification(userDocs, vehicleDocs)

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
			UserDocumentID:    review.UserDocumentID,
			VehicleDocumentID: review.VehicleDocumentID,
			Status:            review.Status,
			Decision:          review.Decision,
			ReasonRejection:   review.ReasonRejection,
			RejectionDetails:  review.RejectionDetails,
			ReviewType:        review.ReviewType,
			AttemptNumber:     review.AttemptNumber,
			PreviousReviewID:  review.PreviousReviewID,
			CreatedAt:         review.CreatedAt.Format(time.RFC3339),
			UpdatedAt:         review.UpdatedAt.Format(time.RFC3339),
		}
		if review.ReviewedAt != nil {
			item.ReviewedAt = review.ReviewedAt.Format(time.RFC3339)
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
		AttemptNumber:     review.AttemptNumber,
		PreviousReviewID:  review.PreviousReviewID,
		CreatedAt:         review.CreatedAt.Format(time.RFC3339),
		UpdatedAt:         review.UpdatedAt.Format(time.RFC3339),
	}
	if review.ReviewedAt != nil {
		detail.ReviewedAt = review.ReviewedAt.Format(time.RFC3339)
	}

	s.logger.Info("admin review retrieved",
		zap.String("reviewID", review.ReviewID),
		zap.String("status", review.Status),
	)
	return detail, nil
}

// publishDocumentReviewNotification publie l'événement de notification (push + email
// via le notification-service) destiné au propriétaire d'un document après sa revue.
// No-op si le Redis de notification n'est pas configuré ou si userID est vide.
func (s *kycServiceImpl) publishDocumentReviewNotification(ctx context.Context, userID, documentType, decision, reasonRejection, rejectionDetails, reviewID string) {
	if s.notifRedis == nil || userID == "" {
		return
	}

	var eventType, status string
	switch decision {
	case "approved":
		eventType, status = notification.DocumentValidated, "validé"
	case "rejected":
		eventType, status = notification.DocumentRejected, "refusé"
	case "resubmission":
		eventType, status = notification.DocumentRejected, "à resoumettre"
	default:
		return // pending ou autre — pas de notification
	}

	payload := map[string]string{
		"document_type": domain.DocumentTypeLabel(documentType),
		"status":        status,
	}
	if eventType == notification.DocumentRejected {
		payload["reason"] = domain.RejectionReasonText(reasonRejection, rejectionDetails)
	}

	if err := notification.Publish(ctx, s.notifRedis, notification.Event{
		EventType:     eventType,
		UserID:        userID,
		ReferenceID:   reviewID,
		ReferenceType: notification.RefDocument,
		Payload:       payload,
	}); err != nil {
		s.logger.Error("failed to publish document review notification",
			zap.Error(err), zap.String("eventType", eventType), zap.String("userID", userID))
	}
}

// propagateProfileVerification recalcule l'état de vérification KYC complet d'un
// utilisateur à partir de ses documents COURANTS et le pousse vers user-service,
// puis pousse le flag is_verified de chacun de ses véhicules vers vehicle-service.
// Publie les notifications de bascule false→true (identité vérifiée / conducteur
// vérifié).
//
// internalUserID est l'UserID interne MongoDB (celui stocké côté file-service et
// attendu par user-service). Best-effort : une erreur est loggée mais ne fait pas
// échouer l'action support — la review est déjà persistée et le recompte, idempotent,
// est auto-réparateur au prochain passage.
func (s *kycServiceImpl) propagateProfileVerification(ctx context.Context, internalUserID string) {
	userDocs, vehicleDocs, err := s.loadVerificationDocs(ctx, internalUserID)
	if err != nil {
		s.logger.Error("propagate verification: failed to load documents",
			zap.Error(err), zap.String("userID", internalUserID))
		return
	}

	passengerVerified, driverVerified := domain.ComputeProfileVerification(userDocs, vehicleDocs)

	prevDriver, prevPassenger, err := s.userClient.UpdateProfileVerification(ctx, internalUserID, driverVerified, passengerVerified)
	if err != nil {
		s.logger.Error("propagate verification: user-service update failed",
			zap.Error(err), zap.String("userID", internalUserID))
	} else if s.notifRedis != nil {
		// Notifications de bascule false→true uniquement (pas de spam sur recompute).
		if passengerVerified && !prevPassenger {
			if pubErr := notification.Publish(ctx, s.notifRedis, notification.Event{
				EventType:     notification.KycApproved,
				UserID:        internalUserID,
				ReferenceID:   internalUserID,
				ReferenceType: notification.RefDocument,
			}); pubErr != nil {
				s.logger.Error("failed to publish identity verified notification", zap.Error(pubErr))
			}
		}
		if driverVerified && !prevDriver {
			if pubErr := notification.Publish(ctx, s.notifRedis, notification.Event{
				EventType:     notification.DriverProfileVerified,
				UserID:        internalUserID,
				ReferenceID:   internalUserID,
				ReferenceType: notification.RefDocument,
			}); pubErr != nil {
				s.logger.Error("failed to publish driver verified notification", zap.Error(pubErr))
			}
		}
	}

	// Pousser is_verified par véhicule (assurance + carte grise approuvées pour CE
	// véhicule). Couvre vérification ET dé-vérification (rejet/resoumission).
	if s.vehicleClient != nil {
		for vehicleID, ok := range domain.ComputeVehicleVerification(vehicleDocs) {
			if vErr := s.vehicleClient.SetVehicleVerification(ctx, vehicleID, ok); vErr != nil {
				s.logger.Warn("propagate verification: vehicle update failed",
					zap.Error(vErr), zap.String("vehicleID", vehicleID))
			}
		}
	}

	s.logger.Info("profile verification propagated",
		zap.String("userID", internalUserID),
		zap.Bool("passenger", passengerVerified),
		zap.Bool("driver", driverVerified),
	)
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
	// Seules les décisions finales sont prononçables ("pending" refusé : il
	// produirait un état completed/pending impossible à re-valider ou overrider).
	if !domain.IsFinalDecision(input.Decision) {
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

	// Seul un rejet peut être overridé — pas une acceptation ni une demande de resoumission.
	if review.Decision != "rejected" {
		s.logger.Warn("only rejected reviews can be overridden",
			zap.String("reviewID", input.ReviewID),
			zap.String("decision", review.Decision),
		)
		return nil, kycErrors.ErrorOnlyRejectionOverridable
	}

	// L'override ne modifie pas la revue rejetée : il en crée une nouvelle, chaînée à
	// la précédente (AttemptNumber+1 / PreviousReviewID), pour conserver l'historique.
	// L'ancienne revue reste intacte ; la nouvelle devient la décision courante.
	now := time.Now().UTC()
	newReview := &domain.Review{
		UserID:               review.UserID,
		DocumentType:         review.DocumentType,
		LogicalDocumentType:  domain.ToLogicalDocumentType(review.DocumentType),
		UserDocumentID:       review.UserDocumentID,
		SecondUserDocumentID: review.SecondUserDocumentID,
		VehicleDocumentID:    review.VehicleDocumentID,

		AttemptNumber:    review.AttemptNumber + 1,
		PreviousReviewID: review.ReviewID, // chaînage vers la revue overridée

		Status:           "completed",
		Decision:         input.Decision,
		ReasonRejection:  input.ReasonRejection,
		RejectionDetails: input.RejectionDetails,
		Notes:            input.Notes,

		ReviewedBy: input.UserID, // l'agent support
		ReviewType: "manual",
		ReviewedAt: &now,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	created, err := s.fileClient.CreateDocumentReview(ctx, newReview)
	if err != nil {
		s.logger.Error("failed to create override review", zap.Error(err))
		return nil, kycErrors.ErrorFileServiceUnavailable
	}

	s.logger.Info("review overridden",
		zap.String("previousReviewID", review.ReviewID),
		zap.String("newReviewID", created.ReviewID),
		zap.String("decision", created.Decision),
		zap.String("reviewedBy", input.UserID),
	)

	// Notifier le propriétaire du document (push + email via notification-service).
	s.publishDocumentReviewNotification(ctx, review.UserID, review.DocumentType,
		input.Decision, input.ReasonRejection, input.RejectionDetails, created.ReviewID)

	// Propager l'état de vérification KYC recalculé vers user-service (best-effort).
	s.propagateProfileVerification(ctx, review.UserID)

	result := &serviceInterfaces.OverrideResult{
		ReviewID:         created.ReviewID,
		Decision:         created.Decision,
		ReasonRejection:  created.ReasonRejection,
		RejectionDetails: created.RejectionDetails,
		ReviewedBy:       created.ReviewedBy,
		ReviewType:       created.ReviewType,
		Notes:            created.Notes,
		UpdatedAt:        created.UpdatedAt.Format(time.RFC3339),
	}
	if created.ReviewedAt != nil {
		result.ReviewedAt = created.ReviewedAt.Format(time.RFC3339)
	}
	return result, nil
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
	// Seules les décisions finales sont prononçables ("pending" refusé).
	if !domain.IsFinalDecision(input.Decision) {
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
			if errors.Is(err, kycErrors.ErrorDocumentNotFound) {
				s.logger.Warn("vehicle document not found", zap.String("documentID", input.DocumentID))
				return nil, kycErrors.ErrorDocumentNotFound
			}
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
			if errors.Is(err, kycErrors.ErrorDocumentNotFound) {
				s.logger.Warn("user document not found", zap.String("documentID", input.DocumentID))
				return nil, kycErrors.ErrorDocumentNotFound
			}
			s.logger.Error("failed to get user document", zap.String("documentID", input.DocumentID), zap.Error(err))
			return nil, kycErrors.ErrorFileServiceUnavailable
		}
		userDocumentID = doc.DocumentID
		ownerUserID = doc.OwnerID // pour user docs, OwnerID = user_id directement
		documentType = doc.DocumentType

		// Unité de validation = document LOGIQUE : si l'agent a fourni le VERSO
		// d'un recto-verso, normaliser vers le recto (face principale de la review)
		// — la décision couvre toujours les deux faces.
		if strings.HasSuffix(documentType, "Back") {
			frontType := domain.CompanionDocumentType(documentType)
			front, ferr := s.fileClient.GetCurrentUserDocument(ctx, ownerUserID, frontType)
			if ferr != nil {
				s.logger.Error("front face missing for two-sided document",
					zap.String("userID", ownerUserID),
					zap.String("backType", documentType),
					zap.Error(ferr),
				)
				return nil, kycErrors.ErrorCompanionDocumentMissing
			}
			userDocumentID = front.DocumentID
			documentType = frontType
		}
	}

	// Contrôle d'unicité : un document ne peut être validé qu'une seule fois.
	// Toute action ultérieure doit passer par OverrideReview.
	existingReviews, err := s.fileClient.GetDocumentReviewsByUserID(ctx, ownerUserID)
	if err != nil {
		s.logger.Error("failed to fetch existing reviews", zap.Error(err))
		return nil, kycErrors.ErrorFileServiceUnavailable
	}
	logicalType := domain.ToLogicalDocumentType(documentType)
	for _, r := range existingReviews {
		if r.Status != "completed" {
			continue
		}
		// vehicle doc : match par vehicle_document_id ; user doc : match par type logique
		if vehicleDocumentID != "" {
			if r.VehicleDocumentID == vehicleDocumentID {
				s.logger.Warn("vehicle document already reviewed",
					zap.String("vehicleDocumentID", vehicleDocumentID), zap.String("reviewID", r.ReviewID))
				return nil, kycErrors.ErrorDocumentAlreadyReviewed
			}
		} else if r.VehicleDocumentID == "" && r.LogicalDocumentType == logicalType {
			s.logger.Warn("user document already reviewed",
				zap.String("userID", ownerUserID), zap.String("logicalType", logicalType), zap.String("reviewID", r.ReviewID))
			return nil, kycErrors.ErrorDocumentAlreadyReviewed
		}
	}

	// Auto-découverte du document compagnon pour les documents recto-verso.
	// Un document recto-verso (idCard, driverLicence) incomplet ne peut pas être
	// validé : la review couvre les deux faces via SecondUserDocumentID.
	var secondUserDocumentID string
	if userDocumentID != "" {
		if companionType := domain.CompanionDocumentType(documentType); companionType != "" {
			companion, err := s.fileClient.GetCurrentUserDocument(ctx, ownerUserID, companionType)
			if err != nil {
				s.logger.Error("companion document missing for two-sided document",
					zap.String("userID", ownerUserID),
					zap.String("documentType", documentType),
					zap.String("companionType", companionType),
					zap.Error(err),
				)
				return nil, kycErrors.ErrorCompanionDocumentMissing
			}
			secondUserDocumentID = companion.DocumentID
		}
	}

	// Chercher une review non terminale (créée à l'upload) déjà en place pour ce
	// document : si trouvée, la faire passer à "completed" via Update plutôt que
	// d'en créer une nouvelle. Sinon (documents uploadés avant ce correctif, ou
	// tout autre cas limite), fallback vers le comportement historique (Create).
	var existingPendingReview *domain.Review
	for _, r := range existingReviews {
		if r.Status == "completed" {
			continue
		}
		if vehicleDocumentID != "" {
			if r.VehicleDocumentID == vehicleDocumentID {
				existingPendingReview = r
				break
			}
		} else if r.VehicleDocumentID == "" && r.LogicalDocumentType == logicalType {
			existingPendingReview = r
			break
		}
	}

	now := time.Now().UTC()
	var created *domain.Review

	if existingPendingReview != nil {
		existingPendingReview.Status = "completed"
		existingPendingReview.Decision = input.Decision
		existingPendingReview.ReasonRejection = input.ReasonRejection
		existingPendingReview.RejectionDetails = input.RejectionDetails
		existingPendingReview.Notes = input.Notes
		existingPendingReview.ReviewedBy = input.SupportAgentID
		existingPendingReview.ReviewType = "manual"
		existingPendingReview.ReviewedAt = &now // valeur locale ; file-service recalcule reviewed_at server-side
		// Transmettre la face compagnon résolue (le file-service re-résout aussi
		// les faces COURANTES au moment de la décision et synchronise leurs statuts).
		if secondUserDocumentID != "" {
			existingPendingReview.SecondUserDocumentID = secondUserDocumentID
		}

		updated, err := s.fileClient.UpdateDocumentReview(ctx, existingPendingReview)
		if err != nil {
			s.logger.Error("failed to update pending document review to completed", zap.Error(err), zap.String("reviewID", existingPendingReview.ReviewID))
			return nil, kycErrors.ErrorFileServiceUnavailable
		}
		created = updated
	} else {
		review := &domain.Review{
			UserID:               ownerUserID,
			DocumentType:         documentType,
			LogicalDocumentType:  domain.ToLogicalDocumentType(documentType),
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

		c, err := s.fileClient.CreateDocumentReview(ctx, review)
		if err != nil {
			s.logger.Error("failed to create manual document review", zap.Error(err))
			return nil, kycErrors.ErrorFileServiceUnavailable
		}
		created = c
	}

	s.logger.Info("document validated manually",
		zap.String("reviewID", created.ReviewID),
		zap.String("decision", created.Decision),
		zap.String("supportAgentID", input.SupportAgentID),
	)

	// Notifier le propriétaire du document (push + email via notification-service).
	s.publishDocumentReviewNotification(ctx, ownerUserID, documentType,
		input.Decision, input.ReasonRejection, input.RejectionDetails, created.ReviewID)

	// Propager l'état de vérification KYC recalculé vers user-service (best-effort).
	s.propagateProfileVerification(ctx, ownerUserID)

	result := &serviceInterfaces.ValidateDocumentResult{
		ReviewID:            created.ReviewID,
		Decision:            created.Decision,
		ReviewedBy:          created.ReviewedBy,
		ReviewType:          created.ReviewType,
		Notes:               created.Notes,
		LogicalDocumentType: created.LogicalDocumentType,
		// Document logique décidé : recto (ou document principal) + verso éventuel.
		DocumentID:       created.UserDocumentID,
		SecondDocumentID: created.SecondUserDocumentID,
	}
	if result.DocumentID == "" {
		result.DocumentID = created.VehicleDocumentID
	}
	if created.ReviewedAt != nil {
		result.ReviewedAt = created.ReviewedAt.Format(time.RFC3339)
	}
	return result, nil
}

// =============================================================================
// GetManualReviewRequests — demandes de validation groupées par utilisateur
// =============================================================================

const defaultManualReviewPageSize int32 = 20

// userBucket agrège les statuts de documents d'un utilisateur par catégorie.
type userBucket struct {
	passengerStatuses []string
	driverStatuses    []string
	logicalDocs       map[string]bool // clés logiques (recto/verso fusionnés) pour TotalDocuments
	lastDeposit       time.Time       // max(uploaded_at) de tous les documents du user
}

// manualReviewEntry : ligne agrégée par utilisateur avant enrichissement.
type manualReviewEntry struct {
	userID          string
	passengerStatus string
	driverStatus    string
	total           int32
	lastDeposit     time.Time
}

func (s *kycServiceImpl) GetManualReviewRequests(ctx context.Context, input serviceInterfaces.GetManualReviewRequestsInput) (*serviceInterfaces.GetManualReviewRequestsResult, error) {
	s.logger.Debug("get manual review requests",
		zap.String("statusFilter", input.Status),
		zap.String("name", input.Name),
		zap.String("firstName", input.FirstName),
		zap.Int32("page", input.Page),
		zap.Int32("pageSize", input.PageSize),
	)

	// Parsing des bornes de date (optionnelles).
	depositFrom, depositTo, err := parseDepositRange(input.DepositFrom, input.DepositTo)
	if err != nil {
		s.logger.Warn("invalid deposit date range", zap.Error(err))
		return nil, kycErrors.ErrorInvalidDateRange
	}

	// On récupère tous les documents KYC (tous statuts), le tri/filtre statut se fait
	// après agrégation pour que le statut par catégorie reste calculé sur l'ensemble.
	docs, err := s.fileClient.ListKycDocuments(ctx, nil)
	if err != nil {
		s.logger.Error("failed to list kyc documents", zap.Error(err))
		return nil, kycErrors.ErrorFileServiceUnavailable
	}

	buckets := make(map[string]*userBucket)
	for _, d := range docs {
		categories := domain.DocumentCategories(d.DocumentType, d.OwnerKind)
		if len(categories) == 0 {
			continue // profilePicture / type inconnu — hors file de validation
		}
		b := buckets[d.UserID]
		if b == nil {
			b = &userBucket{logicalDocs: make(map[string]bool)}
			buckets[d.UserID] = b
		}
		// Multi-catégories : le permis (double rôle) contribue au statut passager
		// ET au statut conducteur — cohérent avec ComputeProfileVerification.
		for _, category := range categories {
			if category == domain.CategoryPassenger {
				b.passengerStatuses = append(b.passengerStatuses, d.Status)
			} else {
				b.driverStatuses = append(b.driverStatuses, d.Status)
			}
		}
		// TotalDocuments = nombre de documents LOGIQUES (recto/verso = 1),
		// identique à ce que la vue détail affiche.
		if d.OwnerKind == "vehicle" {
			b.logicalDocs["v:"+d.DocumentID] = true
		} else {
			b.logicalDocs["u:"+domain.ToLogicalDocumentType(d.DocumentType)] = true
		}
		if t, perr := time.Parse(time.RFC3339, d.UploadedAt); perr == nil && t.After(b.lastDeposit) {
			b.lastDeposit = t
		}
	}

	// Calcul des statuts agrégés + filtre statut + filtre date de dépôt.
	entries := make([]manualReviewEntry, 0, len(buckets))
	for userID, b := range buckets {
		ps := domain.AggregateStatus(b.passengerStatuses)
		ds := domain.AggregateStatus(b.driverStatuses)
		if input.Status != "" && ps != input.Status && ds != input.Status {
			continue
		}
		if !depositFrom.IsZero() && b.lastDeposit.Before(depositFrom) {
			continue
		}
		if !depositTo.IsZero() && b.lastDeposit.After(depositTo) {
			continue
		}
		entries = append(entries, manualReviewEntry{
			userID: userID, passengerStatus: ps, driverStatus: ds, total: int32(len(b.logicalDocs)), lastDeposit: b.lastDeposit,
		})
	}

	// Filtre nom/prénom : nécessite les infos user (batch léger). Appliqué avant pagination.
	if input.Name != "" || input.FirstName != "" {
		entries, err = s.filterByName(ctx, entries, input.Name, input.FirstName)
		if err != nil {
			return nil, err
		}
	}

	// Tri par date du dernier dépôt décroissante (récent d'abord), userID en tie-breaker.
	sort.Slice(entries, func(i, j int) bool {
		if !entries[i].lastDeposit.Equal(entries[j].lastDeposit) {
			return entries[i].lastDeposit.After(entries[j].lastDeposit)
		}
		return entries[i].userID < entries[j].userID
	})

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

		// ProfileImageURL est résolue fraîche à la lecture par user-service
		// (selfie courant, fallback profilePicture) — plus de re-présignature ici.

		lastDeposit := ""
		if !e.lastDeposit.IsZero() {
			lastDeposit = e.lastDeposit.UTC().Format(time.RFC3339)
		}
		requests = append(requests, &domain.ManualReviewRequest{
			User:            userInfo,
			PassengerStatus: e.passengerStatus,
			DriverStatus:    e.driverStatus,
			TotalDocuments:  e.total,
			LastDepositAt:   lastDeposit,
		})
	}

	s.logger.Info("manual review requests retrieved",
		zap.Int32("total", total), zap.Int("page", len(requests)))
	return &serviceInterfaces.GetManualReviewRequestsResult{Requests: requests, Total: total}, nil
}

// filterByName enrichit les entries via un batch user-service léger et ne garde que
// celles dont le nom/prénom contiennent les termes recherchés (insensible à la casse).
func (s *kycServiceImpl) filterByName(ctx context.Context, entries []manualReviewEntry, name, firstName string) ([]manualReviewEntry, error) {
	ids := make([]string, 0, len(entries))
	for _, e := range entries {
		ids = append(ids, e.userID)
	}
	usersByID, err := s.userClient.GetUsersByUserIDs(ctx, ids)
	if err != nil {
		s.logger.Error("failed to batch-fetch users for name filter", zap.Error(err))
		return nil, kycErrors.ErrorUserNotFound
	}

	nameQ := strings.ToLower(name)
	firstQ := strings.ToLower(firstName)
	filtered := entries[:0]
	for _, e := range entries {
		u, ok := usersByID[e.userID]
		if !ok {
			continue // utilisateur introuvable → exclu de la recherche
		}
		if nameQ != "" && !strings.Contains(strings.ToLower(u.Name), nameQ) {
			continue
		}
		if firstQ != "" && !strings.Contains(strings.ToLower(u.FirstName), firstQ) {
			continue
		}
		filtered = append(filtered, e)
	}
	return filtered, nil
}

// parseDepositRange interprète les bornes de date. Accepte RFC3339 ou YYYY-MM-DD
// (auquel cas la borne haute couvre toute la journée). Bornes vides = pas de limite.
func parseDepositRange(from, to string) (time.Time, time.Time, error) {
	var fromT, toT time.Time
	if from != "" {
		t, err := parseFlexibleDate(from, false)
		if err != nil {
			return fromT, toT, err
		}
		fromT = t
	}
	if to != "" {
		t, err := parseFlexibleDate(to, true)
		if err != nil {
			return fromT, toT, err
		}
		toT = t
	}
	return fromT, toT, nil
}

// parseFlexibleDate parse une date RFC3339 ou YYYY-MM-DD. Pour une date seule en
// borne haute (endOfDay), on couvre la fin de journée (23:59:59).
func parseFlexibleDate(s string, endOfDay bool) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC(), nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, err
	}
	t = t.UTC()
	if endOfDay {
		t = t.Add(24*time.Hour - time.Second)
	}
	return t, nil
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

	// Grouper les user docs par LogicalDocumentType pour fusionner les paires recto-verso
	// (idCardFront+idCardBack → une seule entrée).
	// Deux passages pour être indépendant de l'ordre de tri : les documents sont triés
	// par uploaded_at DESC, donc le verso peut précéder le recto. Un passage unique
	// perdrait alors le verso (recto pas encore enregistré).
	byLogicalUser := make(map[string]*domain.DocumentSummary, len(userDocs))
	var userDocOrder []string

	// 1er passage : enregistrer les rectos / documents principaux.
	// profilePicture (héritage, hors KYC) est exclu de la vue support — cohérent
	// avec la liste (TotalDocuments) qui ne le compte pas.
	for _, d := range userDocs {
		if d.DocumentType == "profilePicture" {
			continue
		}
		d.LogicalDocumentType = domain.ToLogicalDocumentType(d.DocumentType)
		d.LatestReview = pickReview(byUserDoc[d.DocumentID], byLogicalType[d.LogicalDocumentType], byType[d.DocumentType])
		if strings.HasSuffix(d.DocumentType, "Back") {
			continue
		}
		if _, exists := byLogicalUser[d.LogicalDocumentType]; !exists {
			byLogicalUser[d.LogicalDocumentType] = d
			userDocOrder = append(userDocOrder, d.LogicalDocumentType)
		}
	}

	// 2e passage : rattacher les versos au recto correspondant.
	for _, d := range userDocs {
		if !strings.HasSuffix(d.DocumentType, "Back") {
			continue
		}
		logical := domain.ToLogicalDocumentType(d.DocumentType)
		if primary, ok := byLogicalUser[logical]; ok {
			primary.SecondDocumentID = d.DocumentID
			primary.SecondDocumentURL = d.DocumentURL
			primary.SecondFileSizeBytes = d.FileSizeBytes
			primary.SecondMimeType = d.MimeType
			primary.SecondUploadedAt = d.UploadedAt
			primary.SecondUpdatedAt = d.UpdatedAt
		}
		// Si le recto est absent (cas anormal), le verso est ignoré.
	}

	for _, key := range userDocOrder {
		docs = append(docs, byLogicalUser[key])
	}

	// Vehicle docs : pas de recto-verso, append direct.
	for _, d := range vehicleDocs {
		d.LogicalDocumentType = domain.ToLogicalDocumentType(d.DocumentType)
		d.LatestReview = pickReview(byVehicleDoc[d.DocumentID], byLogicalType[d.LogicalDocumentType], byType[d.DocumentType])
		docs = append(docs, d)
	}

	// Enrichir les dernières reviews manuelles avec le prénom/nom de l'agent support.
	var supportUIDs []string
	for _, d := range docs {
		if d.LatestReview != nil && d.LatestReview.ReviewType == "manual" && d.LatestReview.ReviewedBy != "" {
			supportUIDs = append(supportUIDs, d.LatestReview.ReviewedBy)
		}
	}
	agents := s.resolveSupportAgents(ctx, supportUIDs)
	for _, d := range docs {
		if d.LatestReview == nil {
			continue
		}
		if agent, ok := agents[d.LatestReview.ReviewedBy]; ok {
			d.LatestReview.ReviewedByFirstName = agent.FirstName
			d.LatestReview.ReviewedByLastName = agent.LastName
		}
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
		Notes:            r.Notes,
		AttemptNumber:    r.AttemptNumber,
		PreviousReviewID: r.PreviousReviewID,
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
	var supportUIDs []string
	for _, r := range reviews {
		// Pour un document véhicule, l'ID du document est vehicle_document_id
		// (user_document_id est vide) — sans ce fallback, DocumentId sortait vide.
		documentID := r.UserDocumentID
		if documentID == "" {
			documentID = r.VehicleDocumentID
		}
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
			DocumentID:          documentID,
			SecondDocumentID:    r.SecondUserDocumentID,
			LogicalDocumentType: r.LogicalDocumentType,
			UpdatedAt:           r.UpdatedAt,
			CreatedAt:           r.CreatedAt,
		}
		if r.ReviewType == "manual" && r.ReviewedBy != "" {
			supportUIDs = append(supportUIDs, r.ReviewedBy)
		}
		entries = append(entries, entry)
	}

	// Enrichir avec le prénom/nom de l'agent support ayant revu (dégradation gracieuse).
	agents := s.resolveSupportAgents(ctx, supportUIDs)
	for _, entry := range entries {
		if agent, ok := agents[entry.ReviewedBy]; ok {
			entry.ReviewedByFirstName = agent.FirstName
			entry.ReviewedByLastName = agent.LastName
		}
	}

	return entries, nil
}
