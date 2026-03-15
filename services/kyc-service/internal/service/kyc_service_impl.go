package service

import (
	"context"
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
	logger            *zap.Logger
}

// NewKYCService crée une nouvelle instance du service KYC
func NewKYCService(
	fileClient client.FileServiceClient,
	personaClient client.PersonaClient,
	personaTemplateID string,
	logger *zap.Logger,
) serviceInterfaces.KYCService {
	return &kycServiceImpl{
		fileClient:        fileClient,
		personaClient:     personaClient,
		personaTemplateID: personaTemplateID,
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

func (s *kycServiceImpl) GetInquiry(ctx context.Context, userID string, personaInquiryID string) (*serviceInterfaces.InquiryDetail, error) {
	// TODO: implémenter
	return nil, nil
}

func (s *kycServiceImpl) GetKYCStatus(ctx context.Context, userID string) (*serviceInterfaces.KYCStatus, error) {
	// TODO: implémenter
	return nil, nil
}

func (s *kycServiceImpl) ResumeInquiry(ctx context.Context, userID string, personaInquiryID string) (*serviceInterfaces.ResumeResult, error) {
	// TODO: implémenter
	return nil, nil
}

func (s *kycServiceImpl) ProcessWebhook(ctx context.Context, input serviceInterfaces.WebhookInput) error {
	// TODO: implémenter
	return nil
}

func (s *kycServiceImpl) GetAdminReviews(ctx context.Context, input serviceInterfaces.GetAdminReviewsInput) ([]*serviceInterfaces.AdminReviewItem, error) {
	// TODO: implémenter
	return nil, nil
}

func (s *kycServiceImpl) GetAdminReview(ctx context.Context, userID string, reviewID string) (*serviceInterfaces.AdminReviewDetail, error) {
	// TODO: implémenter
	return nil, nil
}

func (s *kycServiceImpl) OverrideReview(ctx context.Context, input serviceInterfaces.OverrideReviewInput) (*serviceInterfaces.OverrideResult, error) {
	// TODO: implémenter
	return nil, nil
}
