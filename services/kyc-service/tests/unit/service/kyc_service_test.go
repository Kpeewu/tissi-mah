package service_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/Kpeewu/tissi-mah/services/kyc-service/internal/domain"
	"github.com/Kpeewu/tissi-mah/services/kyc-service/internal/service"
	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/kyc-service/internal/service/interfaces"
	kycErrors "github.com/Kpeewu/tissi-mah/services/kyc-service/pkg/errors"
	"github.com/Kpeewu/tissi-mah/services/kyc-service/tests/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const testTemplateID = "itmpl_test_template"

// --- Helpers ---

// newTestService crée un triplet (fileClient, personaClient) mockés
// et le service instancié à partir de ces mocks.
const testWebhookSecret = "test-webhook-secret"

func newTestService() (*mocks.MockFileServiceClient, *mocks.MockPersonaClient, serviceInterfaces.KYCService) {
	mockFileClient := new(mocks.MockFileServiceClient)
	mockPersonaClient := new(mocks.MockPersonaClient)
	mockUserClient := new(mocks.MockUserClient)
	// Pass-through : les tests utilisent des IDs "user-XXX" directement comme
	// si le Firebase UID et l'UserID interne étaient identiques. Le mock
	// renvoie simplement l'ID reçu.
	mockUserClient.On("GetUserIDByFirebaseID", mock.Anything, mock.AnythingOfType("string")).
		Return(func(_ context.Context, firebaseUID string) string { return firebaseUID }, nil)
	// supportClient nil : enrichissement prénom/nom désactivé (dégradation gracieuse).
	svc := service.NewKYCService(mockFileClient, mockPersonaClient, mockUserClient, nil, testTemplateID, testWebhookSecret, nil, zap.NewNop())
	return mockFileClient, mockPersonaClient, svc
}

// =============================================================================
// CreateInquiry
// =============================================================================

func TestCreateInquiry(t *testing.T) {
	t.Run("succès - crée une inquiry Persona 100% (sans GetUserDocument)", func(t *testing.T) {
		mockFileClient, mockPersonaClient, svc := newTestService()
		ctx := context.Background()

		// Pas de review active
		mockFileClient.On("GetDocumentReviewsByUserID", mock.Anything, "user-001").
			Return([]*domain.Review{}, nil)

		// Persona crée l'inquiry
		expiresAt := time.Now().Add(30 * time.Minute).UTC()
		mockPersonaClient.On("CreateInquiry", mock.Anything, testTemplateID, "user-001").
			Return(&domain.PersonaInquiry{
				InquiryID:    "inq_abc123",
				TemplateID:   testTemplateID,
				SessionToken: "sess_token_xyz",
				ExpiresAt:    expiresAt,
			}, nil)

		// File-service crée la review avec UserID + DocumentType dénormalisés,
		// pas de FK doc (Persona 100%).
		now := time.Now().UTC()
		mockFileClient.On("CreateDocumentReview", mock.Anything, mock.MatchedBy(func(r *domain.Review) bool {
			return r.UserID == "user-001" &&
				r.DocumentType == "passport" &&
				r.UserDocumentID == "" &&
				r.VehicleDocumentID == "" &&
				r.PersonaInquiryID == "inq_abc123" &&
				r.PersonaTemplateID == testTemplateID &&
				r.PersonaSessionToken == "sess_token_xyz" &&
				r.Status == "pending" &&
				r.ReviewType == "automatic" &&
				r.AttemptNumber == 1 &&
				r.PreviousReviewID == ""
		})).Return(&domain.Review{
			ReviewID:          "review-001",
			UserID:            "user-001",
			DocumentType:      "passport",
			PersonaInquiryID:  "inq_abc123",
			PersonaTemplateID: testTemplateID,
			Status:            "pending",
			AttemptNumber:     1,
			CreatedAt:         now,
		}, nil)

		result, err := svc.CreateInquiry(ctx, serviceInterfaces.CreateInquiryInput{
			UserID:       "user-001",
			DocumentType: "passport",
		})

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "review-001", result.ReviewID)
		assert.Equal(t, "inq_abc123", result.PersonaInquiryID)
		assert.Equal(t, testTemplateID, result.PersonaTemplateID)
		assert.Equal(t, "sess_token_xyz", result.SessionToken)
		assert.Equal(t, "pending", result.Status)
		assert.Equal(t, int32(1), result.AttemptNumber)

		mockFileClient.AssertExpectations(t)
		mockPersonaClient.AssertExpectations(t)
	})

	t.Run("erreur - VehicleID fourni → ErrorVehicleDocumentNotAllowed", func(t *testing.T) {
		_, _, svc := newTestService()
		ctx := context.Background()

		result, err := svc.CreateInquiry(ctx, serviceInterfaces.CreateInquiryInput{
			UserID:       "user-002",
			DocumentType: "insurance",
			VehicleID:    "vehicle-001",
		})

		assert.Nil(t, result)
		assert.ErrorIs(t, err, kycErrors.ErrorVehicleDocumentNotAllowed)
	})

	t.Run("succès - incrémente attempt_number sur un retry du même DocumentType", func(t *testing.T) {
		mockFileClient, mockPersonaClient, svc := newTestService()
		ctx := context.Background()

		// Review précédente terminée pour le même type (idCardFront)
		mockFileClient.On("GetDocumentReviewsByUserID", mock.Anything, "user-003").
			Return([]*domain.Review{
				{
					ReviewID:      "review-old-001",
					UserID:        "user-003",
					DocumentType:  "idCardFront",
					Status:        "completed",
					Decision:      "rejected",
					AttemptNumber: 2,
				},
			}, nil)

		expiresAt := time.Now().Add(30 * time.Minute).UTC()
		mockPersonaClient.On("CreateInquiry", mock.Anything, testTemplateID, "user-003").
			Return(&domain.PersonaInquiry{
				InquiryID:    "inq_retry_123",
				TemplateID:   testTemplateID,
				SessionToken: "sess_retry_token",
				ExpiresAt:    expiresAt,
			}, nil)

		now := time.Now().UTC()
		mockFileClient.On("CreateDocumentReview", mock.Anything, mock.MatchedBy(func(r *domain.Review) bool {
			return r.AttemptNumber == 3 &&
				r.PreviousReviewID == "review-old-001" &&
				r.DocumentType == "idCardFront"
		})).Return(&domain.Review{
			ReviewID:         "review-retry-001",
			UserID:           "user-003",
			DocumentType:     "idCardFront",
			PersonaInquiryID: "inq_retry_123",
			Status:           "pending",
			AttemptNumber:    3,
			CreatedAt:        now,
		}, nil)

		result, err := svc.CreateInquiry(ctx, serviceInterfaces.CreateInquiryInput{
			UserID:       "user-003",
			DocumentType: "idCardFront",
		})

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, int32(3), result.AttemptNumber)

		mockFileClient.AssertExpectations(t)
		mockPersonaClient.AssertExpectations(t)
	})

	t.Run("erreur - user_id vide retourne ErrorMissingUserID", func(t *testing.T) {
		_, _, svc := newTestService()
		ctx := context.Background()

		result, err := svc.CreateInquiry(ctx, serviceInterfaces.CreateInquiryInput{
			UserID:       "",
			DocumentType: "passport",
		})

		assert.Nil(t, result)
		assert.ErrorIs(t, err, kycErrors.ErrorMissingUserID)
	})

	t.Run("erreur - document_type vide retourne ErrorMissingDocumentType", func(t *testing.T) {
		_, _, svc := newTestService()
		ctx := context.Background()

		result, err := svc.CreateInquiry(ctx, serviceInterfaces.CreateInquiryInput{
			UserID:       "user-001",
			DocumentType: "",
		})

		assert.Nil(t, result)
		assert.ErrorIs(t, err, kycErrors.ErrorMissingDocumentType)
	})

	t.Run("erreur 409 - review active pending existe déjà", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		ctx := context.Background()

		mockFileClient.On("GetDocumentReviewsByUserID", mock.Anything, "user-004").
			Return([]*domain.Review{
				{
					ReviewID:       "review-active-001",
					UserDocumentID: "doc-active-001",
					Status:         "pending",
					AttemptNumber:  1,
				},
			}, nil)

		result, err := svc.CreateInquiry(ctx, serviceInterfaces.CreateInquiryInput{
			UserID:       "user-004",
			DocumentID:   "doc-active-001",
			DocumentType: "passport",
		})

		assert.Nil(t, result)
		assert.ErrorIs(t, err, kycErrors.ErrorInquiryAlreadyActive)
		mockFileClient.AssertExpectations(t)
	})

	t.Run("erreur 409 - review active inProgress existe déjà", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		ctx := context.Background()

		mockFileClient.On("GetDocumentReviewsByUserID", mock.Anything, "user-005").
			Return([]*domain.Review{
				{
					ReviewID:       "review-inprogress-001",
					UserDocumentID: "doc-inprogress-001",
					Status:         "inProgress",
					AttemptNumber:  1,
				},
			}, nil)

		result, err := svc.CreateInquiry(ctx, serviceInterfaces.CreateInquiryInput{
			UserID:       "user-005",
			DocumentID:   "doc-inprogress-001",
			DocumentType: "passport",
		})

		assert.Nil(t, result)
		assert.ErrorIs(t, err, kycErrors.ErrorInquiryAlreadyActive)
		mockFileClient.AssertExpectations(t)
	})

	t.Run("erreur - file-service indisponible lors de la récupération des reviews", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		ctx := context.Background()

		mockFileClient.On("GetDocumentReviewsByUserID", mock.Anything, "user-006").
			Return(nil, errors.New("connection refused"))

		result, err := svc.CreateInquiry(ctx, serviceInterfaces.CreateInquiryInput{
			UserID:       "user-006",
			DocumentID:   "doc-006",
			DocumentType: "passport",
		})

		assert.Nil(t, result)
		assert.ErrorIs(t, err, kycErrors.ErrorFileServiceUnavailable)
		mockFileClient.AssertExpectations(t)
	})

	t.Run("erreur - API Persona indisponible", func(t *testing.T) {
		mockFileClient, mockPersonaClient, svc := newTestService()
		ctx := context.Background()

		mockFileClient.On("GetDocumentReviewsByUserID", mock.Anything, "user-009").
			Return([]*domain.Review{}, nil)

		mockPersonaClient.On("CreateInquiry", mock.Anything, testTemplateID, "user-009").
			Return(nil, errors.New("persona API timeout"))

		result, err := svc.CreateInquiry(ctx, serviceInterfaces.CreateInquiryInput{
			UserID:       "user-009",
			DocumentType: "passport",
		})

		assert.Nil(t, result)
		assert.ErrorIs(t, err, kycErrors.ErrorPersonaUnavailable)
		mockFileClient.AssertExpectations(t)
		mockPersonaClient.AssertExpectations(t)
	})

	t.Run("erreur - file-service échoue lors de la création de la review", func(t *testing.T) {
		mockFileClient, mockPersonaClient, svc := newTestService()
		ctx := context.Background()

		mockFileClient.On("GetDocumentReviewsByUserID", mock.Anything, "user-010").
			Return([]*domain.Review{}, nil)

		expiresAt := time.Now().Add(30 * time.Minute).UTC()
		mockPersonaClient.On("CreateInquiry", mock.Anything, testTemplateID, "user-010").
			Return(&domain.PersonaInquiry{
				InquiryID:    "inq_010",
				TemplateID:   testTemplateID,
				SessionToken: "sess_010",
				ExpiresAt:    expiresAt,
			}, nil)

		mockFileClient.On("CreateDocumentReview", mock.Anything, mock.Anything).
			Return(nil, errors.New("db constraint violation"))

		result, err := svc.CreateInquiry(ctx, serviceInterfaces.CreateInquiryInput{
			UserID:       "user-010",
			DocumentType: "passport",
		})

		assert.Nil(t, result)
		assert.ErrorIs(t, err, kycErrors.ErrorFileServiceUnavailable)
		mockFileClient.AssertExpectations(t)
		mockPersonaClient.AssertExpectations(t)
	})

	t.Run("succès - review completed existante ne bloque pas la création", func(t *testing.T) {
		mockFileClient, mockPersonaClient, svc := newTestService()
		ctx := context.Background()

		// Review existante avec statut completed → ne bloque pas
		mockFileClient.On("GetDocumentReviewsByUserID", mock.Anything, "user-011").
			Return([]*domain.Review{
				{
					ReviewID:      "review-completed-001",
					UserID:        "user-011",
					DocumentType:  "passport",
					Status:        "completed",
					Decision:      "approved",
					AttemptNumber: 1,
				},
			}, nil)

		expiresAt := time.Now().Add(30 * time.Minute).UTC()
		mockPersonaClient.On("CreateInquiry", mock.Anything, testTemplateID, "user-011").
			Return(&domain.PersonaInquiry{
				InquiryID:    "inq_011",
				TemplateID:   testTemplateID,
				SessionToken: "sess_011",
				ExpiresAt:    expiresAt,
			}, nil)

		now := time.Now().UTC()
		mockFileClient.On("CreateDocumentReview", mock.Anything, mock.Anything).
			Return(&domain.Review{
				ReviewID:         "review-011",
				PersonaInquiryID: "inq_011",
				Status:           "pending",
				AttemptNumber:    2,
				CreatedAt:        now,
			}, nil)

		result, err := svc.CreateInquiry(ctx, serviceInterfaces.CreateInquiryInput{
			UserID:       "user-011",
			DocumentType: "passport",
		})

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "review-011", result.ReviewID)
		assert.Equal(t, "pending", result.Status)

		mockFileClient.AssertExpectations(t)
		mockPersonaClient.AssertExpectations(t)
	})

	t.Run("normalisation - DocumentType haut-niveau persisté en type file-service", func(t *testing.T) {
		mockFileClient, mockPersonaClient, svc := newTestService()
		ctx := context.Background()

		mockFileClient.On("GetDocumentReviewsByUserID", mock.Anything, "user-norm").
			Return([]*domain.Review{}, nil)

		expiresAt := time.Now().Add(30 * time.Minute).UTC()
		mockPersonaClient.On("CreateInquiry", mock.Anything, testTemplateID, "user-norm").
			Return(&domain.PersonaInquiry{
				InquiryID:    "inq_norm",
				TemplateID:   testTemplateID,
				SessionToken: "sess_norm",
				ExpiresAt:    expiresAt,
			}, nil)

		// L'input "DriverLicence" est normalisé en "driverLicenceFront"
		// pour rester cohérent avec identityDocumentTypes/driverDocumentTypes.
		mockFileClient.On("CreateDocumentReview", mock.Anything, mock.MatchedBy(func(r *domain.Review) bool {
			return r.DocumentType == "driverLicenceFront"
		})).Return(&domain.Review{
			ReviewID:         "review-norm",
			DocumentType:     "driverLicenceFront",
			PersonaInquiryID: "inq_norm",
			Status:           "pending",
			AttemptNumber:    1,
			CreatedAt:        time.Now().UTC(),
		}, nil)

		result, err := svc.CreateInquiry(ctx, serviceInterfaces.CreateInquiryInput{
			UserID:       "user-norm",
			DocumentType: "DriverLicence",
		})

		require.NoError(t, err)
		require.NotNil(t, result)
		mockFileClient.AssertExpectations(t)
	})
}

// =============================================================================
// GetInquiry
// =============================================================================

func TestGetInquiry(t *testing.T) {
	t.Run("succès - retourne le détail d'une inquiry (Persona 100%)", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		ctx := context.Background()

		now := time.Now().UTC()
		reviewedAt := now.Add(-1 * time.Hour)
		review := &domain.Review{
			ReviewID:          "review-get-001",
			UserID:            "user-get-001",
			DocumentType:      "passport",
			PersonaInquiryID:  "inq_get_001",
			PersonaTemplateID: testTemplateID,
			Status:            "completed",
			Decision:          "approved",
			AttemptNumber:     1,
			ReviewType:        "automatic",
			ReviewedAt:        &reviewedAt,
			CreatedAt:         now,
			UpdatedAt:         now,
		}

		mockFileClient.On("GetDocumentReviewByPersonaInquiryID", mock.Anything, "inq_get_001").
			Return(review, nil)

		result, err := svc.GetInquiry(ctx, "user-get-001", "inq_get_001")

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "review-get-001", result.ReviewID)
		assert.Equal(t, "inq_get_001", result.PersonaInquiryID)
		assert.Equal(t, "completed", result.Status)
		assert.Equal(t, "approved", result.Decision)
		assert.Equal(t, int32(1), result.AttemptNumber)
		assert.Equal(t, "automatic", result.ReviewType)
		assert.NotEmpty(t, result.ReviewedAt)
		assert.NotEmpty(t, result.CreatedAt)
		assert.NotEmpty(t, result.UpdatedAt)

		mockFileClient.AssertExpectations(t)
	})

	t.Run("erreur - user_id vide retourne ErrorMissingUserID", func(t *testing.T) {
		_, _, svc := newTestService()
		ctx := context.Background()

		result, err := svc.GetInquiry(ctx, "", "inq_001")

		assert.Nil(t, result)
		assert.ErrorIs(t, err, kycErrors.ErrorMissingUserID)
	})

	t.Run("erreur - persona_inquiry_id vide retourne ErrorMissingInquiryID", func(t *testing.T) {
		_, _, svc := newTestService()
		ctx := context.Background()

		result, err := svc.GetInquiry(ctx, "user-001", "")

		assert.Nil(t, result)
		assert.ErrorIs(t, err, kycErrors.ErrorMissingInquiryID)
	})

	t.Run("erreur - inquiry introuvable retourne ErrorInquiryNotFound", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		ctx := context.Background()

		mockFileClient.On("GetDocumentReviewByPersonaInquiryID", mock.Anything, "inq_unknown").
			Return(nil, errors.New("not found"))

		result, err := svc.GetInquiry(ctx, "user-001", "inq_unknown")

		assert.Nil(t, result)
		assert.ErrorIs(t, err, kycErrors.ErrorInquiryNotFound)
		mockFileClient.AssertExpectations(t)
	})

	t.Run("erreur 403 - inquiry appartient à un autre utilisateur", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		ctx := context.Background()

		now := time.Now().UTC()
		review := &domain.Review{
			ReviewID:         "review-other-user",
			UserID:           "user-owner", // pas l'appelant
			DocumentType:     "passport",
			PersonaInquiryID: "inq_other_user",
			Status:           "pending",
			AttemptNumber:    1,
			CreatedAt:        now,
			UpdatedAt:        now,
		}

		mockFileClient.On("GetDocumentReviewByPersonaInquiryID", mock.Anything, "inq_other_user").
			Return(review, nil)

		result, err := svc.GetInquiry(ctx, "user-impostor", "inq_other_user")

		assert.Nil(t, result)
		assert.ErrorIs(t, err, kycErrors.ErrorUnauthorized)
		mockFileClient.AssertExpectations(t)
	})
}

// =============================================================================
// GetKYCStatus
// =============================================================================

func TestGetKYCStatus(t *testing.T) {
	t.Run("succès - identité vérifiée, permis non vérifié", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		ctx := context.Background()

		now := time.Now().UTC()
		reviewedAt := now.Add(-1 * time.Hour)

		mockFileClient.On("GetDocumentReviewsByUserID", mock.Anything, "user-status-001").
			Return([]*domain.Review{
				{
					ReviewID:         "review-approved-id",
					UserID:           "user-status-001",
					DocumentType:     "passport",
					PersonaInquiryID: "inq_passport",
					Status:           "completed",
					Decision:         "approved",
					AttemptNumber:    1,
					ReviewType:       "automatic",
					ReviewedAt:       &reviewedAt,
					CreatedAt:        now,
					UpdatedAt:        now,
				},
			}, nil)

		result, err := svc.GetKYCStatus(ctx, "user-status-001")

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.IdentityVerified)
		assert.False(t, result.DriverVerified)
		assert.Empty(t, result.PendingReviews)
		assert.Nil(t, result.LatestRejection)

		mockFileClient.AssertExpectations(t)
	})

	t.Run("succès - identité ET permis vérifiés", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		ctx := context.Background()

		now := time.Now().UTC()
		reviewedAt := now.Add(-1 * time.Hour)

		mockFileClient.On("GetDocumentReviewsByUserID", mock.Anything, "user-status-002").
			Return([]*domain.Review{
				{
					ReviewID:      "review-id-approved",
					UserID:        "user-status-002",
					DocumentType:  "idCardFront",
					Status:        "completed",
					Decision:      "approved",
					ReviewType:    "automatic",
					ReviewedAt:    &reviewedAt,
					AttemptNumber: 1,
					CreatedAt:     now,
					UpdatedAt:     now,
				},
				{
					ReviewID:      "review-dl-approved",
					UserID:        "user-status-002",
					DocumentType:  "driverLicenceFront",
					Status:        "completed",
					Decision:      "approved",
					ReviewType:    "automatic",
					ReviewedAt:    &reviewedAt,
					AttemptNumber: 1,
					CreatedAt:     now,
					UpdatedAt:     now,
				},
			}, nil)

		result, err := svc.GetKYCStatus(ctx, "user-status-002")

		require.NoError(t, err)
		assert.True(t, result.IdentityVerified)
		assert.True(t, result.DriverVerified)

		mockFileClient.AssertExpectations(t)
	})

	t.Run("succès - permis approuvé mais identité non vérifiée → driver_verified = false", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		ctx := context.Background()

		now := time.Now().UTC()
		reviewedAt := now.Add(-1 * time.Hour)

		// Uniquement un permis approuvé, pas d'identité
		mockFileClient.On("GetDocumentReviewsByUserID", mock.Anything, "user-status-003").
			Return([]*domain.Review{
				{
					ReviewID:      "review-dl-only",
					UserID:        "user-status-003",
					DocumentType:  "driverLicenceFront",
					Status:        "completed",
					Decision:      "approved",
					ReviewType:    "automatic",
					ReviewedAt:    &reviewedAt,
					AttemptNumber: 1,
					CreatedAt:     now,
					UpdatedAt:     now,
				},
			}, nil)

		result, err := svc.GetKYCStatus(ctx, "user-status-003")

		require.NoError(t, err)
		assert.False(t, result.IdentityVerified)
		assert.False(t, result.DriverVerified) // car identité non vérifiée

		mockFileClient.AssertExpectations(t)
	})

	t.Run("succès - pending reviews retournées", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		ctx := context.Background()

		now := time.Now().UTC()
		expiresAt := now.Add(30 * time.Minute)

		mockFileClient.On("GetDocumentReviewsByUserID", mock.Anything, "user-status-004").
			Return([]*domain.Review{
				{
					ReviewID:         "review-pending-001",
					UserID:           "user-status-004",
					DocumentType:     "passport",
					PersonaInquiryID: "inq_pending",
					Status:           "pending",
					AttemptNumber:    1,
					SessionExpiresAt: &expiresAt,
					CreatedAt:        now,
					UpdatedAt:        now,
				},
				{
					ReviewID:         "review-submitted-001",
					UserID:           "user-status-004",
					DocumentType:     "idCardFront",
					PersonaInquiryID: "inq_submitted",
					Status:           "submitted",
					AttemptNumber:    2,
					CreatedAt:        now,
					UpdatedAt:        now,
				},
			}, nil)

		result, err := svc.GetKYCStatus(ctx, "user-status-004")

		require.NoError(t, err)
		assert.Len(t, result.PendingReviews, 2)
		assert.Equal(t, "review-pending-001", result.PendingReviews[0].ReviewID)
		assert.Equal(t, "pending", result.PendingReviews[0].Status)
		assert.Equal(t, "review-submitted-001", result.PendingReviews[1].ReviewID)
		assert.Equal(t, "submitted", result.PendingReviews[1].Status)

		mockFileClient.AssertExpectations(t)
	})

	t.Run("succès - latest rejection retournée", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		ctx := context.Background()

		now := time.Now().UTC()
		olderReviewedAt := now.Add(-48 * time.Hour)
		newerReviewedAt := now.Add(-1 * time.Hour)

		mockFileClient.On("GetDocumentReviewsByUserID", mock.Anything, "user-status-005").
			Return([]*domain.Review{
				{
					ReviewID:         "review-rejected-old",
					UserID:           "user-status-005",
					DocumentType:     "passport",
					Status:           "completed",
					Decision:         "rejected",
					ReasonRejection:  "document_expired",
					RejectionDetails: "Le passeport est expiré",
					ReviewType:       "automatic",
					ReviewedAt:       &olderReviewedAt,
					AttemptNumber:    1,
					CreatedAt:        now,
					UpdatedAt:        now,
				},
				{
					ReviewID:         "review-rejected-new",
					UserID:           "user-status-005",
					DocumentType:     "passport",
					Status:           "completed",
					Decision:         "rejected",
					ReasonRejection:  "photo_missmatch",
					RejectionDetails: "La photo ne correspond pas",
					ReviewType:       "automatic",
					ReviewedAt:       &newerReviewedAt,
					AttemptNumber:    2,
					CreatedAt:        now,
					UpdatedAt:        now,
				},
			}, nil)

		result, err := svc.GetKYCStatus(ctx, "user-status-005")

		require.NoError(t, err)
		require.NotNil(t, result.LatestRejection)
		assert.Equal(t, "review-rejected-new", result.LatestRejection.ReviewID)
		assert.Equal(t, "photo_missmatch", result.LatestRejection.ReasonRejection)
		assert.Equal(t, "La photo ne correspond pas", result.LatestRejection.RejectionDetails)
		assert.Equal(t, "automatic", result.LatestRejection.ReviewType)

		mockFileClient.AssertExpectations(t)
	})

	t.Run("succès - aucune review → tout à false/vide", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		ctx := context.Background()

		mockFileClient.On("GetDocumentReviewsByUserID", mock.Anything, "user-status-006").
			Return([]*domain.Review{}, nil)

		result, err := svc.GetKYCStatus(ctx, "user-status-006")

		require.NoError(t, err)
		assert.False(t, result.IdentityVerified)
		assert.False(t, result.DriverVerified)
		assert.Empty(t, result.PendingReviews)
		assert.Nil(t, result.LatestRejection)

		mockFileClient.AssertExpectations(t)
	})

	t.Run("erreur - user_id vide retourne ErrorMissingUserID", func(t *testing.T) {
		_, _, svc := newTestService()
		ctx := context.Background()

		result, err := svc.GetKYCStatus(ctx, "")

		assert.Nil(t, result)
		assert.ErrorIs(t, err, kycErrors.ErrorMissingUserID)
	})

	t.Run("erreur - file-service indisponible", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		ctx := context.Background()

		mockFileClient.On("GetDocumentReviewsByUserID", mock.Anything, "user-status-fail").
			Return(nil, errors.New("connection refused"))

		result, err := svc.GetKYCStatus(ctx, "user-status-fail")

		assert.Nil(t, result)
		assert.ErrorIs(t, err, kycErrors.ErrorFileServiceUnavailable)
		mockFileClient.AssertExpectations(t)
	})
}

// =============================================================================
// ResumeInquiry
// =============================================================================

func TestResumeInquiry(t *testing.T) {
	now := time.Now().UTC()
	newExpiry := now.Add(30 * time.Minute)

	// Review de base réutilisable pour les tests
	baseReview := &domain.Review{
		ReviewID:            "review-resume-001",
		UserID:              "user-001",
		DocumentType:        "passport",
		PersonaInquiryID:    "inq_resume_001",
		PersonaTemplateID:   testTemplateID,
		PersonaSessionToken: "old-token",
		SessionExpiresAt:    &now,
		Status:              "pending",
		AttemptNumber:       1,
		CreatedAt:           now,
		UpdatedAt:           now,
	}

	t.Run("succès - reprend une inquiry pending", func(t *testing.T) {
		mockFileClient, mockPersonaClient, svc := newTestService()
		ctx := context.Background()

		mockFileClient.On("GetDocumentReviewByPersonaInquiryID", mock.Anything, "inq_resume_001").
			Return(baseReview, nil)
		mockPersonaClient.On("ResumeInquiry", mock.Anything, "inq_resume_001").
			Return(&domain.PersonaSession{SessionToken: "new-token", ExpiresAt: newExpiry}, nil)
		mockFileClient.On("UpdateDocumentReview", mock.Anything, mock.AnythingOfType("*domain.Review")).
			Return(&domain.Review{
				ReviewID:         "review-resume-001",
				PersonaInquiryID: "inq_resume_001",
				Status:           "inProgress",
				AttemptNumber:    1,
			}, nil)

		result, err := svc.ResumeInquiry(ctx, "user-001", "inq_resume_001")
		require.NoError(t, err)
		assert.Equal(t, "review-resume-001", result.ReviewID)
		assert.Equal(t, "new-token", result.SessionToken)
		assert.Equal(t, "inProgress", result.Status)
		assert.Equal(t, int32(1), result.AttemptNumber)

		// Vérifier que le token et le statut ont été mis à jour avant l'appel UpdateDocumentReview
		mockFileClient.AssertCalled(t, "UpdateDocumentReview", mock.Anything, mock.MatchedBy(func(r *domain.Review) bool {
			return r.PersonaSessionToken == "new-token" && r.Status == "inProgress"
		}))
	})

	t.Run("succès - reprend une inquiry inProgress", func(t *testing.T) {
		mockFileClient, mockPersonaClient, svc := newTestService()
		ctx := context.Background()

		inProgressReview := &domain.Review{
			ReviewID:         "review-resume-002",
			UserID:           "user-001",
			DocumentType:     "passport",
			PersonaInquiryID: "inq_resume_002",
			Status:           "inProgress",
			AttemptNumber:    2,
			CreatedAt:        now,
			UpdatedAt:        now,
		}

		mockFileClient.On("GetDocumentReviewByPersonaInquiryID", mock.Anything, "inq_resume_002").
			Return(inProgressReview, nil)
		mockPersonaClient.On("ResumeInquiry", mock.Anything, "inq_resume_002").
			Return(&domain.PersonaSession{SessionToken: "renewed-token", ExpiresAt: newExpiry}, nil)
		mockFileClient.On("UpdateDocumentReview", mock.Anything, mock.AnythingOfType("*domain.Review")).
			Return(&domain.Review{
				ReviewID:         "review-resume-002",
				PersonaInquiryID: "inq_resume_002",
				Status:           "inProgress",
				AttemptNumber:    2,
			}, nil)

		result, err := svc.ResumeInquiry(ctx, "user-001", "inq_resume_002")
		require.NoError(t, err)
		assert.Equal(t, "review-resume-002", result.ReviewID)
		assert.Equal(t, "renewed-token", result.SessionToken)
		assert.Equal(t, int32(2), result.AttemptNumber)
	})

	t.Run("succès - reprend une inquiry submitted", func(t *testing.T) {
		mockFileClient, mockPersonaClient, svc := newTestService()
		ctx := context.Background()

		submittedReview := &domain.Review{
			ReviewID:         "review-resume-003",
			UserID:           "user-001",
			DocumentType:     "passport",
			PersonaInquiryID: "inq_resume_003",
			Status:           "submitted",
			AttemptNumber:    1,
			CreatedAt:        now,
			UpdatedAt:        now,
		}

		mockFileClient.On("GetDocumentReviewByPersonaInquiryID", mock.Anything, "inq_resume_003").
			Return(submittedReview, nil)
		mockPersonaClient.On("ResumeInquiry", mock.Anything, "inq_resume_003").
			Return(&domain.PersonaSession{SessionToken: "sub-token", ExpiresAt: newExpiry}, nil)
		mockFileClient.On("UpdateDocumentReview", mock.Anything, mock.AnythingOfType("*domain.Review")).
			Return(&domain.Review{
				ReviewID:         "review-resume-003",
				PersonaInquiryID: "inq_resume_003",
				Status:           "inProgress",
				AttemptNumber:    1,
			}, nil)

		result, err := svc.ResumeInquiry(ctx, "user-001", "inq_resume_003")
		require.NoError(t, err)
		assert.Equal(t, "inProgress", result.Status)
	})

	t.Run("erreur - user_id vide retourne ErrorMissingUserID", func(t *testing.T) {
		_, _, svc := newTestService()
		ctx := context.Background()

		result, err := svc.ResumeInquiry(ctx, "", "inq_resume_001")
		assert.Nil(t, result)
		assert.ErrorIs(t, err, kycErrors.ErrorMissingUserID)
	})

	t.Run("erreur - persona_inquiry_id vide retourne ErrorMissingInquiryID", func(t *testing.T) {
		_, _, svc := newTestService()
		ctx := context.Background()

		result, err := svc.ResumeInquiry(ctx, "user-001", "")
		assert.Nil(t, result)
		assert.ErrorIs(t, err, kycErrors.ErrorMissingInquiryID)
	})

	t.Run("erreur - inquiry introuvable retourne ErrorInquiryNotFound", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		ctx := context.Background()

		mockFileClient.On("GetDocumentReviewByPersonaInquiryID", mock.Anything, "inq_unknown").
			Return(nil, errors.New("not found"))

		result, err := svc.ResumeInquiry(ctx, "user-001", "inq_unknown")
		assert.Nil(t, result)
		assert.ErrorIs(t, err, kycErrors.ErrorInquiryNotFound)
	})

	t.Run("erreur 403 - inquiry n'appartient pas à l'utilisateur", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		ctx := context.Background()

		mockFileClient.On("GetDocumentReviewByPersonaInquiryID", mock.Anything, "inq_resume_001").
			Return(baseReview, nil)

		result, err := svc.ResumeInquiry(ctx, "user-other", "inq_resume_001")
		assert.Nil(t, result)
		assert.ErrorIs(t, err, kycErrors.ErrorUnauthorized)
	})

	t.Run("erreur 410 - inquiry completed non resumable", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		ctx := context.Background()

		completedReview := &domain.Review{
			ReviewID:         "review-completed",
			UserID:           "user-001",
			DocumentType:     "passport",
			PersonaInquiryID: "inq_completed",
			Status:           "completed",
			CreatedAt:        now,
			UpdatedAt:        now,
		}

		mockFileClient.On("GetDocumentReviewByPersonaInquiryID", mock.Anything, "inq_completed").
			Return(completedReview, nil)

		result, err := svc.ResumeInquiry(ctx, "user-001", "inq_completed")
		assert.Nil(t, result)
		assert.ErrorIs(t, err, kycErrors.ErrorInquiryNotResumable)
	})

	t.Run("erreur 410 - inquiry expired non resumable", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		ctx := context.Background()

		expiredReview := &domain.Review{
			ReviewID:         "review-expired",
			UserID:           "user-001",
			DocumentType:     "passport",
			PersonaInquiryID: "inq_expired",
			Status:           "expired",
			CreatedAt:        now,
			UpdatedAt:        now,
		}

		mockFileClient.On("GetDocumentReviewByPersonaInquiryID", mock.Anything, "inq_expired").
			Return(expiredReview, nil)

		result, err := svc.ResumeInquiry(ctx, "user-001", "inq_expired")
		assert.Nil(t, result)
		assert.ErrorIs(t, err, kycErrors.ErrorInquiryNotResumable)
	})

	t.Run("erreur 410 - inquiry failed non resumable", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		ctx := context.Background()

		failedReview := &domain.Review{
			ReviewID:         "review-failed",
			UserID:           "user-001",
			DocumentType:     "passport",
			PersonaInquiryID: "inq_failed",
			Status:           "failed",
			CreatedAt:        now,
			UpdatedAt:        now,
		}

		mockFileClient.On("GetDocumentReviewByPersonaInquiryID", mock.Anything, "inq_failed").
			Return(failedReview, nil)

		result, err := svc.ResumeInquiry(ctx, "user-001", "inq_failed")
		assert.Nil(t, result)
		assert.ErrorIs(t, err, kycErrors.ErrorInquiryNotResumable)
	})

	t.Run("erreur - Persona indisponible lors du renouvellement", func(t *testing.T) {
		mockFileClient, mockPersonaClient, svc := newTestService()
		ctx := context.Background()

		mockFileClient.On("GetDocumentReviewByPersonaInquiryID", mock.Anything, "inq_resume_001").
			Return(baseReview, nil)
		mockPersonaClient.On("ResumeInquiry", mock.Anything, "inq_resume_001").
			Return(nil, errors.New("persona down"))

		result, err := svc.ResumeInquiry(ctx, "user-001", "inq_resume_001")
		assert.Nil(t, result)
		assert.ErrorIs(t, err, kycErrors.ErrorPersonaUnavailable)
	})

	t.Run("erreur - file-service indisponible lors de la mise à jour", func(t *testing.T) {
		mockFileClient, mockPersonaClient, svc := newTestService()
		ctx := context.Background()

		mockFileClient.On("GetDocumentReviewByPersonaInquiryID", mock.Anything, "inq_resume_001").
			Return(baseReview, nil)
		mockPersonaClient.On("ResumeInquiry", mock.Anything, "inq_resume_001").
			Return(&domain.PersonaSession{SessionToken: "new-token", ExpiresAt: newExpiry}, nil)
		mockFileClient.On("UpdateDocumentReview", mock.Anything, mock.AnythingOfType("*domain.Review")).
			Return(nil, errors.New("file-service down"))

		result, err := svc.ResumeInquiry(ctx, "user-001", "inq_resume_001")
		assert.Nil(t, result)
		assert.ErrorIs(t, err, kycErrors.ErrorFileServiceUnavailable)
	})
}

// =============================================================================
// ProcessWebhook
// =============================================================================

// computePersonaSignature génère un header Persona-Signature valide pour les tests.
// Produit "t=<ts>,v1=<hmac_sha256(secret, timestamp.body)>" comme Persona l'envoie.
func computePersonaSignature(ts int64, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(testWebhookSecret))
	mac.Write([]byte(strconv.FormatInt(ts, 10)))
	mac.Write([]byte("."))
	mac.Write(payload)
	return fmt.Sprintf("t=%d,v1=%s", ts, hex.EncodeToString(mac.Sum(nil)))
}

func TestProcessWebhook(t *testing.T) {
	now := time.Now().UTC()
	payload := []byte(`{"data":{"id":"inq_wh_001"}}`)
	validSignature := computePersonaSignature(now.Unix(), payload)

	// Crée une copie fraîche pour éviter les mutations inter-tests
	newWebhookReview := func() *domain.Review {
		return &domain.Review{
			ReviewID:         "review-wh-001",
			UserDocumentID:   "doc-001",
			PersonaInquiryID: "inq_wh_001",
			Status:           "pending",
			AttemptNumber:    1,
			CreatedAt:        now,
			UpdatedAt:        now,
		}
	}

	t.Run("succès - inquiry.approved met à jour status=completed decision=approved", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		ctx := context.Background()

		mockFileClient.On("GetDocumentReviewByPersonaInquiryID", mock.Anything, "inq_wh_001").
			Return(newWebhookReview(), nil)
		mockFileClient.On("UpdateDocumentReview", mock.Anything, mock.MatchedBy(func(r *domain.Review) bool {
			return r.Status == "completed" && r.Decision == "approved" && r.ReviewedAt != nil &&
				r.WebhookEventType == "inquiry.approved"
		})).Return(newWebhookReview(), nil)

		err := svc.ProcessWebhook(ctx, serviceInterfaces.WebhookInput{
			Signature:         validSignature,
			PersonaInquiryID:  "inq_wh_001",
			WebhookEventType:  "inquiry.approved",
			PersonaRawPayload: payload,
		})
		assert.NoError(t, err)
		mockFileClient.AssertExpectations(t)
	})

	t.Run("succès - inquiry.declined met à jour status=completed decision=rejected", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		ctx := context.Background()

		mockFileClient.On("GetDocumentReviewByPersonaInquiryID", mock.Anything, "inq_wh_001").
			Return(newWebhookReview(), nil)
		mockFileClient.On("UpdateDocumentReview", mock.Anything, mock.MatchedBy(func(r *domain.Review) bool {
			return r.Status == "completed" && r.Decision == "rejected" && r.ReviewedAt != nil
		})).Return(newWebhookReview(), nil)

		err := svc.ProcessWebhook(ctx, serviceInterfaces.WebhookInput{
			Signature:         validSignature,
			PersonaInquiryID:  "inq_wh_001",
			WebhookEventType:  "inquiry.declined",
			PersonaRawPayload: payload,
		})
		assert.NoError(t, err)
	})

	t.Run("succès - inquiry.submitted met à jour status=submitted et submitted_at", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		ctx := context.Background()

		mockFileClient.On("GetDocumentReviewByPersonaInquiryID", mock.Anything, "inq_wh_001").
			Return(newWebhookReview(), nil)
		mockFileClient.On("UpdateDocumentReview", mock.Anything, mock.MatchedBy(func(r *domain.Review) bool {
			return r.Status == "submitted" && r.SubmittedAt != nil && r.Decision == ""
		})).Return(newWebhookReview(), nil)

		err := svc.ProcessWebhook(ctx, serviceInterfaces.WebhookInput{
			Signature:         validSignature,
			PersonaInquiryID:  "inq_wh_001",
			WebhookEventType:  "inquiry.submitted",
			PersonaRawPayload: payload,
		})
		assert.NoError(t, err)
	})

	t.Run("succès - inquiry.started met à jour status=inProgress", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		ctx := context.Background()

		mockFileClient.On("GetDocumentReviewByPersonaInquiryID", mock.Anything, "inq_wh_001").
			Return(newWebhookReview(), nil)
		mockFileClient.On("UpdateDocumentReview", mock.Anything, mock.MatchedBy(func(r *domain.Review) bool {
			return r.Status == "inProgress"
		})).Return(newWebhookReview(), nil)

		err := svc.ProcessWebhook(ctx, serviceInterfaces.WebhookInput{
			Signature:         validSignature,
			PersonaInquiryID:  "inq_wh_001",
			WebhookEventType:  "inquiry.started",
			PersonaRawPayload: payload,
		})
		assert.NoError(t, err)
	})

	t.Run("succès - inquiry.expired met à jour status=expired", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		ctx := context.Background()

		mockFileClient.On("GetDocumentReviewByPersonaInquiryID", mock.Anything, "inq_wh_001").
			Return(newWebhookReview(), nil)
		mockFileClient.On("UpdateDocumentReview", mock.Anything, mock.MatchedBy(func(r *domain.Review) bool {
			return r.Status == "expired"
		})).Return(newWebhookReview(), nil)

		err := svc.ProcessWebhook(ctx, serviceInterfaces.WebhookInput{
			Signature:         validSignature,
			PersonaInquiryID:  "inq_wh_001",
			WebhookEventType:  "inquiry.expired",
			PersonaRawPayload: payload,
		})
		assert.NoError(t, err)
	})

	t.Run("succès - inquiry.failed met à jour status=failed", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		ctx := context.Background()

		mockFileClient.On("GetDocumentReviewByPersonaInquiryID", mock.Anything, "inq_wh_001").
			Return(newWebhookReview(), nil)
		mockFileClient.On("UpdateDocumentReview", mock.Anything, mock.MatchedBy(func(r *domain.Review) bool {
			return r.Status == "failed"
		})).Return(newWebhookReview(), nil)

		err := svc.ProcessWebhook(ctx, serviceInterfaces.WebhookInput{
			Signature:         validSignature,
			PersonaInquiryID:  "inq_wh_001",
			WebhookEventType:  "inquiry.failed",
			PersonaRawPayload: payload,
		})
		assert.NoError(t, err)
	})

	t.Run("succès - événement inconnu est ignoré sans erreur", func(t *testing.T) {
		_, _, svc := newTestService()
		ctx := context.Background()

		err := svc.ProcessWebhook(ctx, serviceInterfaces.WebhookInput{
			Signature:         validSignature,
			PersonaInquiryID:  "inq_wh_001",
			WebhookEventType:  "inquiry.unknown_event",
			PersonaRawPayload: payload,
		})
		assert.NoError(t, err)
	})

	t.Run("erreur - signature HMAC invalide", func(t *testing.T) {
		_, _, svc := newTestService()
		ctx := context.Background()

		err := svc.ProcessWebhook(ctx, serviceInterfaces.WebhookInput{
			Signature:         "invalid-signature",
			PersonaInquiryID:  "inq_wh_001",
			WebhookEventType:  "inquiry.approved",
			PersonaRawPayload: payload,
		})
		assert.ErrorIs(t, err, kycErrors.ErrorInvalidWebhookSignature)
	})

	t.Run("erreur - persona_inquiry_id vide", func(t *testing.T) {
		_, _, svc := newTestService()
		ctx := context.Background()

		err := svc.ProcessWebhook(ctx, serviceInterfaces.WebhookInput{
			Signature:         computePersonaSignature(time.Now().Unix(), payload),
			PersonaInquiryID:  "",
			WebhookEventType:  "inquiry.approved",
			PersonaRawPayload: payload,
		})
		assert.ErrorIs(t, err, kycErrors.ErrorMissingInquiryID)
	})

	t.Run("erreur - review introuvable", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		ctx := context.Background()

		mockFileClient.On("GetDocumentReviewByPersonaInquiryID", mock.Anything, "inq_unknown").
			Return(nil, errors.New("not found"))

		err := svc.ProcessWebhook(ctx, serviceInterfaces.WebhookInput{
			Signature:         computePersonaSignature(time.Now().Unix(), payload),
			PersonaInquiryID:  "inq_unknown",
			WebhookEventType:  "inquiry.approved",
			PersonaRawPayload: payload,
		})
		assert.ErrorIs(t, err, kycErrors.ErrorInquiryNotFound)
	})

	t.Run("erreur - file-service indisponible lors de la mise à jour", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		ctx := context.Background()

		mockFileClient.On("GetDocumentReviewByPersonaInquiryID", mock.Anything, "inq_wh_001").
			Return(newWebhookReview(), nil)
		mockFileClient.On("UpdateDocumentReview", mock.Anything, mock.AnythingOfType("*domain.Review")).
			Return(nil, errors.New("file-service down"))

		err := svc.ProcessWebhook(ctx, serviceInterfaces.WebhookInput{
			Signature:         validSignature,
			PersonaInquiryID:  "inq_wh_001",
			WebhookEventType:  "inquiry.approved",
			PersonaRawPayload: payload,
		})
		assert.ErrorIs(t, err, kycErrors.ErrorFileServiceUnavailable)
	})

	t.Run("succès - le payload raw est stocké dans persona_raw_payload", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		ctx := context.Background()

		customPayload := []byte(`{"custom":"data","nested":{"key":"value"}}`)
		customSig := computePersonaSignature(time.Now().Unix(), customPayload)

		mockFileClient.On("GetDocumentReviewByPersonaInquiryID", mock.Anything, "inq_wh_001").
			Return(newWebhookReview(), nil)
		mockFileClient.On("UpdateDocumentReview", mock.Anything, mock.MatchedBy(func(r *domain.Review) bool {
			return string(r.PersonaRawPayload) == `{"custom":"data","nested":{"key":"value"}}`
		})).Return(newWebhookReview(), nil)

		err := svc.ProcessWebhook(ctx, serviceInterfaces.WebhookInput{
			Signature:         customSig,
			PersonaInquiryID:  "inq_wh_001",
			WebhookEventType:  "inquiry.completed",
			PersonaRawPayload: customPayload,
		})
		assert.NoError(t, err)
		mockFileClient.AssertExpectations(t)
	})

	t.Run("erreur - header signature vide (pas de t= ni v1=)", func(t *testing.T) {
		_, _, svc := newTestService()

		err := svc.ProcessWebhook(context.Background(), serviceInterfaces.WebhookInput{
			Signature:         "",
			PersonaInquiryID:  "inq_wh_001",
			WebhookEventType:  "inquiry.approved",
			PersonaRawPayload: payload,
		})
		assert.ErrorIs(t, err, kycErrors.ErrorInvalidWebhookSignature)
	})

	t.Run("erreur - timestamp expiré (> 5 minutes)", func(t *testing.T) {
		_, _, svc := newTestService()
		expiredSig := computePersonaSignature(time.Now().Unix()-600, payload)

		err := svc.ProcessWebhook(context.Background(), serviceInterfaces.WebhookInput{
			Signature:         expiredSig,
			PersonaInquiryID:  "inq_wh_001",
			WebhookEventType:  "inquiry.approved",
			PersonaRawPayload: payload,
		})
		assert.ErrorIs(t, err, kycErrors.ErrorInvalidWebhookSignature)
	})

	t.Run("succès - plusieurs v1= (rotation de secret, deuxième valide)", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		ts := time.Now().Unix()
		validSig := computePersonaSignature(ts, payload)
		// Header avec un v1= invalide suivi du v1= correct (simule rotation de secret)
		header := strings.Replace(validSig, "v1=", "v1=deadbeef,v1=", 1)

		mockFileClient.On("GetDocumentReviewByPersonaInquiryID", mock.Anything, "inq_wh_001").
			Return(newWebhookReview(), nil)
		mockFileClient.On("UpdateDocumentReview", mock.Anything, mock.AnythingOfType("*domain.Review")).
			Return(newWebhookReview(), nil)

		err := svc.ProcessWebhook(context.Background(), serviceInterfaces.WebhookInput{
			Signature:         header,
			PersonaInquiryID:  "inq_wh_001",
			WebhookEventType:  "inquiry.approved",
			PersonaRawPayload: payload,
		})
		assert.NoError(t, err)
		mockFileClient.AssertExpectations(t)
	})
}

// =============================================================================
// GetAdminReviews
// =============================================================================

func TestGetAdminReviews(t *testing.T) {
	now := time.Now().UTC()
	reviewedAt := now.Add(-1 * time.Hour)
	webhookReceivedAt := now.Add(-2 * time.Hour)

	t.Run("succès - retourne la liste paginée des reviews", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		ctx := context.Background()

		mockFileClient.On("ListDocumentReviews", mock.Anything, "user-001", "", "", int32(1), int32(20)).
			Return([]*domain.Review{
				{
					ReviewID:          "review-admin-001",
					PersonaInquiryID:  "inq_admin_001",
					UserDocumentID:    "doc-001",
					Status:            "completed",
					Decision:          "approved",
					ReviewType:        "automatic",
					AttemptNumber:     1,
					WebhookEventType:  "inquiry.approved",
					ReviewedAt:        &reviewedAt,
					WebhookReceivedAt: &webhookReceivedAt,
					CreatedAt:         now,
					UpdatedAt:         now,
				},
				{
					ReviewID:         "review-admin-002",
					PersonaInquiryID: "inq_admin_002",
					UserDocumentID:   "doc-002",
					Status:           "pending",
					ReviewType:       "automatic",
					AttemptNumber:    1,
					CreatedAt:        now,
					UpdatedAt:        now,
				},
			}, nil)

		items, err := svc.GetAdminReviews(ctx, serviceInterfaces.GetAdminReviewsInput{
			UserID: "user-001",
			Index:  1,
		})
		require.NoError(t, err)
		assert.Len(t, items, 2)

		// Premier item : avec reviewed_at et webhook_received_at
		assert.Equal(t, "review-admin-001", items[0].ReviewID)
		assert.Equal(t, "completed", items[0].Status)
		assert.Equal(t, "approved", items[0].Decision)
		assert.NotEmpty(t, items[0].ReviewedAt)
		assert.NotEmpty(t, items[0].WebhookReceivedAt)

		// Deuxième item : sans reviewed_at
		assert.Equal(t, "review-admin-002", items[1].ReviewID)
		assert.Equal(t, "pending", items[1].Status)
		assert.Empty(t, items[1].ReviewedAt)
	})

	t.Run("succès - filtre par status", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		ctx := context.Background()

		mockFileClient.On("ListDocumentReviews", mock.Anything, "", "pending", "", int32(1), int32(20)).
			Return([]*domain.Review{
				{
					ReviewID:         "review-pending",
					PersonaInquiryID: "inq_pending",
					Status:           "pending",
					AttemptNumber:    1,
					CreatedAt:        now,
					UpdatedAt:        now,
				},
			}, nil)

		items, err := svc.GetAdminReviews(ctx, serviceInterfaces.GetAdminReviewsInput{
			Status: "pending",
			Index:  1,
		})
		require.NoError(t, err)
		assert.Len(t, items, 1)
		assert.Equal(t, "pending", items[0].Status)
	})

	t.Run("succès - filtre par decision", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		ctx := context.Background()

		mockFileClient.On("ListDocumentReviews", mock.Anything, "", "", "rejected", int32(1), int32(20)).
			Return([]*domain.Review{
				{
					ReviewID:         "review-rejected",
					PersonaInquiryID: "inq_rejected",
					Status:           "completed",
					Decision:         "rejected",
					AttemptNumber:    2,
					CreatedAt:        now,
					UpdatedAt:        now,
				},
			}, nil)

		items, err := svc.GetAdminReviews(ctx, serviceInterfaces.GetAdminReviewsInput{
			Decision: "rejected",
			Index:    1,
		})
		require.NoError(t, err)
		assert.Len(t, items, 1)
		assert.Equal(t, "rejected", items[0].Decision)
	})

	t.Run("succès - liste vide retourne un slice vide", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		ctx := context.Background()

		mockFileClient.On("ListDocumentReviews", mock.Anything, "user-empty", "", "", int32(1), int32(20)).
			Return([]*domain.Review{}, nil)

		items, err := svc.GetAdminReviews(ctx, serviceInterfaces.GetAdminReviewsInput{
			UserID: "user-empty",
			Index:  1,
		})
		require.NoError(t, err)
		assert.Empty(t, items)
	})

	t.Run("erreur - file-service indisponible", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		ctx := context.Background()

		mockFileClient.On("ListDocumentReviews", mock.Anything, "", "", "", int32(1), int32(20)).
			Return(nil, errors.New("file-service down"))

		items, err := svc.GetAdminReviews(ctx, serviceInterfaces.GetAdminReviewsInput{Index: 1})
		assert.Nil(t, items)
		assert.ErrorIs(t, err, kycErrors.ErrorFileServiceUnavailable)
	})
}

// =============================================================================
// GetAdminReview
// =============================================================================

func TestGetAdminReview(t *testing.T) {
	now := time.Now().UTC()
	reviewedAt := now.Add(-1 * time.Hour)
	sessionExpiry := now.Add(30 * time.Minute)
	webhookReceivedAt := now.Add(-2 * time.Hour)

	t.Run("succès - retourne le détail complet d'une review", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		ctx := context.Background()

		mockFileClient.On("GetDocumentReview", mock.Anything, "review-detail-001").
			Return(&domain.Review{
				ReviewID:          "review-detail-001",
				PersonaInquiryID:  "inq_detail_001",
				PersonaTemplateID: testTemplateID,
				UserDocumentID:    "doc-001",
				Status:            "completed",
				Decision:          "approved",
				ReviewedBy:        "admin-001",
				ReviewType:        "manual",
				ReviewedAt:        &reviewedAt,
				Notes:             "Verified manually",
				ExtractedData:     []byte(`{"name":"John"}`),
				WebhookEventType:  "inquiry.approved",
				WebhookReceivedAt: &webhookReceivedAt,
				SessionExpiresAt:  &sessionExpiry,
				AttemptNumber:     2,
				PreviousReviewID:  "review-prev-001",
				CreatedAt:         now,
				UpdatedAt:         now,
			}, nil)

		detail, err := svc.GetAdminReview(ctx, "admin-001", "review-detail-001")
		require.NoError(t, err)
		assert.Equal(t, "review-detail-001", detail.ReviewID)
		assert.Equal(t, "inq_detail_001", detail.PersonaInquiryID)
		assert.Equal(t, testTemplateID, detail.PersonaTemplateID)
		assert.Equal(t, "doc-001", detail.UserDocumentID)
		assert.Equal(t, "completed", detail.Status)
		assert.Equal(t, "approved", detail.Decision)
		assert.Equal(t, "admin-001", detail.ReviewedBy)
		assert.Equal(t, "manual", detail.ReviewType)
		assert.NotEmpty(t, detail.ReviewedAt)
		assert.Equal(t, "Verified manually", detail.Notes)
		assert.Equal(t, []byte(`{"name":"John"}`), detail.ExtractedData)
		assert.NotEmpty(t, detail.WebhookReceivedAt)
		assert.NotEmpty(t, detail.SessionExpiresAt)
		assert.Equal(t, int32(2), detail.AttemptNumber)
		assert.Equal(t, "review-prev-001", detail.PreviousReviewID)
	})

	t.Run("succès - review sans champs optionnels", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		ctx := context.Background()

		mockFileClient.On("GetDocumentReview", mock.Anything, "review-minimal").
			Return(&domain.Review{
				ReviewID:         "review-minimal",
				PersonaInquiryID: "inq_minimal",
				Status:           "pending",
				AttemptNumber:    1,
				CreatedAt:        now,
				UpdatedAt:        now,
			}, nil)

		detail, err := svc.GetAdminReview(ctx, "admin-001", "review-minimal")
		require.NoError(t, err)
		assert.Equal(t, "review-minimal", detail.ReviewID)
		assert.Equal(t, "pending", detail.Status)
		assert.Empty(t, detail.ReviewedAt)
		assert.Empty(t, detail.WebhookReceivedAt)
		assert.Empty(t, detail.SessionExpiresAt)
	})

	t.Run("erreur - review_id vide retourne ErrorMissingReviewID", func(t *testing.T) {
		_, _, svc := newTestService()
		ctx := context.Background()

		detail, err := svc.GetAdminReview(ctx, "admin-001", "")
		assert.Nil(t, detail)
		assert.ErrorIs(t, err, kycErrors.ErrorMissingReviewID)
	})

	t.Run("erreur - review introuvable retourne ErrorReviewNotFound", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		ctx := context.Background()

		mockFileClient.On("GetDocumentReview", mock.Anything, "review-unknown").
			Return(nil, errors.New("not found"))

		detail, err := svc.GetAdminReview(ctx, "admin-001", "review-unknown")
		assert.Nil(t, detail)
		assert.ErrorIs(t, err, kycErrors.ErrorReviewNotFound)
	})
}

// =============================================================================
// OverrideReview
// =============================================================================

func TestOverrideReview(t *testing.T) {
	now := time.Now().UTC()

	// Seule une review « rejected » est overridable (règle métier).
	newCompletedReview := func() *domain.Review {
		return &domain.Review{
			ReviewID:         "review-override-001",
			PersonaInquiryID: "inq_override_001",
			UserDocumentID:   "doc-001",
			Status:           "completed",
			Decision:         "rejected",
			ReviewType:       "automatic",
			AttemptNumber:    1,
			CreatedAt:        now,
			UpdatedAt:        now,
		}
	}

	t.Run("erreur - override d'une approbation interdit", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		ctx := context.Background()

		approvedReview := newCompletedReview()
		approvedReview.Decision = "approved"

		mockFileClient.On("GetDocumentReview", mock.Anything, "review-override-001").
			Return(approvedReview, nil)

		result, err := svc.OverrideReview(ctx, serviceInterfaces.OverrideReviewInput{
			UserID:           "admin-001",
			ReviewID:         "review-override-001",
			Decision:         "rejected",
			ReasonRejection:  "document_expired",
			RejectionDetails: "ID card expired",
		})
		assert.Nil(t, result)
		assert.ErrorIs(t, err, kycErrors.ErrorOnlyRejectionOverridable)
	})

	t.Run("erreur - override d'une resoumission interdit", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		ctx := context.Background()

		resubReview := newCompletedReview()
		resubReview.Decision = "resubmission"

		mockFileClient.On("GetDocumentReview", mock.Anything, "review-override-001").
			Return(resubReview, nil)

		result, err := svc.OverrideReview(ctx, serviceInterfaces.OverrideReviewInput{
			UserID:   "admin-001",
			ReviewID: "review-override-001",
			Decision: "approved",
		})
		assert.Nil(t, result)
		assert.ErrorIs(t, err, kycErrors.ErrorOnlyRejectionOverridable)
	})

	t.Run("succès - override rejected → approved", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		ctx := context.Background()

		rejectedReview := newCompletedReview()

		mockFileClient.On("GetDocumentReview", mock.Anything, "review-override-001").
			Return(rejectedReview, nil)
		// L'override crée une NOUVELLE revue chaînée (historique), il ne mute pas l'ancienne.
		mockFileClient.On("CreateDocumentReview", mock.Anything, mock.MatchedBy(func(r *domain.Review) bool {
			return r.Decision == "approved" &&
				r.ReviewType == "manual" &&
				r.Status == "completed" &&
				r.AttemptNumber == rejectedReview.AttemptNumber+1 &&
				r.PreviousReviewID == "review-override-001"
		})).Return(&domain.Review{
			ReviewID:   "review-override-002",
			Decision:   "approved",
			ReviewedBy: "admin-001",
			ReviewType: "manual",
			Status:     "completed",
			ReviewedAt: &now,
			UpdatedAt:  now,
		}, nil)

		result, err := svc.OverrideReview(ctx, serviceInterfaces.OverrideReviewInput{
			UserID:   "admin-001",
			ReviewID: "review-override-001",
			Decision: "approved",
		})
		require.NoError(t, err)
		assert.Equal(t, "review-override-002", result.ReviewID) // nouvelle revue
		assert.Equal(t, "approved", result.Decision)
		assert.Equal(t, "manual", result.ReviewType)
	})

	t.Run("erreur - user_id vide", func(t *testing.T) {
		_, _, svc := newTestService()
		ctx := context.Background()

		result, err := svc.OverrideReview(ctx, serviceInterfaces.OverrideReviewInput{
			ReviewID: "review-override-001",
			Decision: "approved",
		})
		assert.Nil(t, result)
		assert.ErrorIs(t, err, kycErrors.ErrorMissingUserID)
	})

	t.Run("erreur - review_id vide", func(t *testing.T) {
		_, _, svc := newTestService()
		ctx := context.Background()

		result, err := svc.OverrideReview(ctx, serviceInterfaces.OverrideReviewInput{
			UserID:   "admin-001",
			Decision: "approved",
		})
		assert.Nil(t, result)
		assert.ErrorIs(t, err, kycErrors.ErrorMissingReviewID)
	})

	t.Run("erreur - decision invalide", func(t *testing.T) {
		_, _, svc := newTestService()
		ctx := context.Background()

		result, err := svc.OverrideReview(ctx, serviceInterfaces.OverrideReviewInput{
			UserID:   "admin-001",
			ReviewID: "review-override-001",
			Decision: "invalid_decision",
		})
		assert.Nil(t, result)
		assert.ErrorIs(t, err, kycErrors.ErrorInvalidDecision)
	})

	t.Run("erreur - rejected sans reason_rejection", func(t *testing.T) {
		_, _, svc := newTestService()
		ctx := context.Background()

		result, err := svc.OverrideReview(ctx, serviceInterfaces.OverrideReviewInput{
			UserID:   "admin-001",
			ReviewID: "review-override-001",
			Decision: "rejected",
		})
		assert.Nil(t, result)
		assert.ErrorIs(t, err, kycErrors.ErrorInvalidDecision)
	})

	t.Run("erreur - review introuvable", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		ctx := context.Background()

		mockFileClient.On("GetDocumentReview", mock.Anything, "review-unknown").
			Return(nil, errors.New("not found"))

		result, err := svc.OverrideReview(ctx, serviceInterfaces.OverrideReviewInput{
			UserID:   "admin-001",
			ReviewID: "review-unknown",
			Decision: "approved",
		})
		assert.Nil(t, result)
		assert.ErrorIs(t, err, kycErrors.ErrorReviewNotFound)
	})

	t.Run("erreur - review pending non overridable", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		ctx := context.Background()

		pendingReview := newCompletedReview()
		pendingReview.Status = "pending"

		mockFileClient.On("GetDocumentReview", mock.Anything, "review-override-001").
			Return(pendingReview, nil)

		result, err := svc.OverrideReview(ctx, serviceInterfaces.OverrideReviewInput{
			UserID:   "admin-001",
			ReviewID: "review-override-001",
			Decision: "approved",
		})
		assert.Nil(t, result)
		assert.ErrorIs(t, err, kycErrors.ErrorReviewNotOverridable)
	})

	t.Run("erreur - review inProgress non overridable", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		ctx := context.Background()

		inProgressReview := newCompletedReview()
		inProgressReview.Status = "inProgress"

		mockFileClient.On("GetDocumentReview", mock.Anything, "review-override-001").
			Return(inProgressReview, nil)

		result, err := svc.OverrideReview(ctx, serviceInterfaces.OverrideReviewInput{
			UserID:   "admin-001",
			ReviewID: "review-override-001",
			Decision: "approved",
		})
		assert.Nil(t, result)
		assert.ErrorIs(t, err, kycErrors.ErrorReviewNotOverridable)
	})

	t.Run("erreur - file-service indisponible lors de la mise à jour", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		ctx := context.Background()

		mockFileClient.On("GetDocumentReview", mock.Anything, "review-override-001").
			Return(newCompletedReview(), nil)
		mockFileClient.On("CreateDocumentReview", mock.Anything, mock.AnythingOfType("*domain.Review")).
			Return(nil, errors.New("file-service down"))

		result, err := svc.OverrideReview(ctx, serviceInterfaces.OverrideReviewInput{
			UserID:   "admin-001",
			ReviewID: "review-override-001",
			Decision: "approved",
		})
		assert.Nil(t, result)
		assert.ErrorIs(t, err, kycErrors.ErrorFileServiceUnavailable)
	})
}

// =============================================================================
// GetManualReviewRequests / GetManualReviewRequestDetail (validation manuelle support)
// =============================================================================

func newManualReviewService() (*mocks.MockFileServiceClient, *mocks.MockUserClient, serviceInterfaces.KYCService) {
	mockFileClient := new(mocks.MockFileServiceClient)
	mockPersonaClient := new(mocks.MockPersonaClient)
	mockUserClient := new(mocks.MockUserClient)
	// supportClient nil : enrichissement prénom/nom désactivé (dégradation gracieuse).
	svc := service.NewKYCService(mockFileClient, mockPersonaClient, mockUserClient, nil, testTemplateID, testWebhookSecret, nil, zap.NewNop())
	return mockFileClient, mockUserClient, svc
}

func TestGetManualReviewRequests(t *testing.T) {
	t.Run("groupement par utilisateur + statuts par catégorie", func(t *testing.T) {
		fileClient, userClient, svc := newManualReviewService()
		fileClient.On("ListKycDocuments", mock.Anything, []string(nil)).Return([]*domain.KycDocument{
			// userA : identité pending (passenger) + assurance rejected (driver/vehicle)
			{DocumentID: "d1", UserID: "userA", DocumentType: "idCardFront", Status: "pending", OwnerKind: "user"},
			{DocumentID: "d2", UserID: "userA", VehicleID: "veh1", DocumentType: "insurance", Status: "rejected", OwnerKind: "vehicle"},
			// userB : passport approved (passenger)
			{DocumentID: "d3", UserID: "userB", DocumentType: "passport", Status: "approved", OwnerKind: "user"},
			// profilePicture ignoré (catégorie other)
			{DocumentID: "d4", UserID: "userB", DocumentType: "profilePicture", Status: "approved", OwnerKind: "user"},
		}, nil)
		userClient.On("GetUserByUserID", mock.Anything, "userA").Return(&domain.UserInfo{UserID: "userA", Name: "A"}, nil)
		userClient.On("GetUserByUserID", mock.Anything, "userB").Return(&domain.UserInfo{UserID: "userB", Name: "B"}, nil)

		res, err := svc.GetManualReviewRequests(context.Background(), serviceInterfaces.GetManualReviewRequestsInput{})
		require.NoError(t, err)
		require.Equal(t, int32(2), res.Total)
		require.Len(t, res.Requests, 2)

		// Tri par userID : userA puis userB
		a := res.Requests[0]
		assert.Equal(t, "userA", a.User.UserID)
		assert.Equal(t, "pending", a.PassengerStatus)
		assert.Equal(t, "rejected", a.DriverStatus)
		assert.Equal(t, int32(2), a.TotalDocuments)

		b := res.Requests[1]
		assert.Equal(t, "userB", b.User.UserID)
		assert.Equal(t, "approved", b.PassengerStatus)
		assert.Equal(t, "", b.DriverStatus)
		assert.Equal(t, int32(1), b.TotalDocuments) // profilePicture exclu
	})

	t.Run("filtre statut ne garde que les users correspondants", func(t *testing.T) {
		fileClient, userClient, svc := newManualReviewService()
		fileClient.On("ListKycDocuments", mock.Anything, []string(nil)).Return([]*domain.KycDocument{
			{DocumentID: "d1", UserID: "userA", DocumentType: "idCardFront", Status: "pending", OwnerKind: "user"},
			{DocumentID: "d3", UserID: "userB", DocumentType: "passport", Status: "approved", OwnerKind: "user"},
		}, nil)
		userClient.On("GetUserByUserID", mock.Anything, "userA").Return(&domain.UserInfo{UserID: "userA"}, nil)

		res, err := svc.GetManualReviewRequests(context.Background(), serviceInterfaces.GetManualReviewRequestsInput{Status: "pending"})
		require.NoError(t, err)
		require.Equal(t, int32(1), res.Total)
		require.Len(t, res.Requests, 1)
		assert.Equal(t, "userA", res.Requests[0].User.UserID)
	})

	t.Run("file-service indisponible → erreur", func(t *testing.T) {
		fileClient, _, svc := newManualReviewService()
		fileClient.On("ListKycDocuments", mock.Anything, []string(nil)).Return(nil, errors.New("down"))

		_, err := svc.GetManualReviewRequests(context.Background(), serviceInterfaces.GetManualReviewRequestsInput{})
		assert.ErrorIs(t, err, kycErrors.ErrorFileServiceUnavailable)
	})
}

func TestGetManualReviewRequestDetail(t *testing.T) {
	t.Run("documents + dernière review rattachée", func(t *testing.T) {
		fileClient, userClient, svc := newManualReviewService()
		userClient.On("GetUserByUserID", mock.Anything, "userA").Return(&domain.UserInfo{UserID: "userA", Name: "A"}, nil)
		fileClient.On("GetUserDocumentSummaries", mock.Anything, "userA").Return([]*domain.DocumentSummary{
			{DocumentID: "d1", DocumentType: "idCardFront", Status: "approved", OwnerKind: "user", OwnerID: "userA", Category: "passenger"},
		}, nil)
		fileClient.On("GetVehicleDocumentSummariesByUserID", mock.Anything, "userA").Return([]*domain.DocumentSummary{
			{DocumentID: "v1", DocumentType: "insurance", Status: "rejected", OwnerKind: "vehicle", OwnerID: "veh1", Category: "driver"},
		}, nil)
		older := time.Now().Add(-2 * time.Hour)
		newer := time.Now().Add(-1 * time.Hour)
		fileClient.On("GetDocumentReviewsByUserID", mock.Anything, "userA").Return([]*domain.Review{
			{ReviewID: "r-old", UserDocumentID: "d1", DocumentType: "idCardFront", Decision: "pending", ReviewedAt: &older},
			{ReviewID: "r-new", UserDocumentID: "d1", DocumentType: "idCardFront", Decision: "approved", ReviewedBy: "agent1", ReviewedAt: &newer},
			{ReviewID: "r-veh", VehicleDocumentID: "v1", DocumentType: "insurance", Decision: "rejected", ReasonRejection: "document_illegible", ReviewedAt: &newer},
		}, nil)

		detail, err := svc.GetManualReviewRequestDetail(context.Background(), "userA")
		require.NoError(t, err)
		assert.Equal(t, "userA", detail.User.UserID)
		require.Len(t, detail.Documents, 2)

		// idCardFront : dernière review = r-new (la plus récente)
		idDoc := detail.Documents[0]
		assert.Equal(t, "d1", idDoc.DocumentID)
		require.NotNil(t, idDoc.LatestReview)
		assert.Equal(t, "r-new", idDoc.LatestReview.ReviewID)
		assert.Equal(t, "approved", idDoc.LatestReview.Decision)
		assert.Equal(t, "agent1", idDoc.LatestReview.ReviewedBy)

		// insurance : review véhicule rattachée par VehicleDocumentID
		vehDoc := detail.Documents[1]
		assert.Equal(t, "v1", vehDoc.DocumentID)
		assert.Equal(t, "vehicle", vehDoc.OwnerKind)
		assert.Equal(t, "veh1", vehDoc.OwnerID)
		require.NotNil(t, vehDoc.LatestReview)
		assert.Equal(t, "r-veh", vehDoc.LatestReview.ReviewID)
		assert.Equal(t, "document_illegible", vehDoc.LatestReview.ReasonRejection)
	})

	t.Run("document sans review → LatestReview nil", func(t *testing.T) {
		fileClient, userClient, svc := newManualReviewService()
		userClient.On("GetUserByUserID", mock.Anything, "userA").Return(&domain.UserInfo{UserID: "userA"}, nil)
		fileClient.On("GetUserDocumentSummaries", mock.Anything, "userA").Return([]*domain.DocumentSummary{
			{DocumentID: "d1", DocumentType: "idCardFront", Status: "pending", OwnerKind: "user", OwnerID: "userA", Category: "passenger"},
		}, nil)
		fileClient.On("GetVehicleDocumentSummariesByUserID", mock.Anything, "userA").Return([]*domain.DocumentSummary{}, nil)
		fileClient.On("GetDocumentReviewsByUserID", mock.Anything, "userA").Return([]*domain.Review{}, nil)

		detail, err := svc.GetManualReviewRequestDetail(context.Background(), "userA")
		require.NoError(t, err)
		require.Len(t, detail.Documents, 1)
		assert.Nil(t, detail.Documents[0].LatestReview)
	})

	t.Run("userID vide → ErrorMissingUserID", func(t *testing.T) {
		_, _, svc := newManualReviewService()
		_, err := svc.GetManualReviewRequestDetail(context.Background(), "")
		assert.ErrorIs(t, err, kycErrors.ErrorMissingUserID)
	})
}
