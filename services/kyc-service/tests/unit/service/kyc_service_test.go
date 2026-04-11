package service_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
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
	svc := service.NewKYCService(mockFileClient, mockPersonaClient, mockUserClient, testTemplateID, testWebhookSecret, nil, zap.NewNop())
	return mockFileClient, mockPersonaClient, svc
}

// =============================================================================
// CreateInquiry
// =============================================================================

func TestCreateInquiry(t *testing.T) {
	t.Run("succès - crée une inquiry pour un document utilisateur", func(t *testing.T) {
		mockFileClient, mockPersonaClient, svc := newTestService()
		ctx := context.Background()

		// Pas de review active
		mockFileClient.On("GetDocumentReviewsByUserID", mock.Anything, "user-001").
			Return([]*domain.Review{}, nil)

		// Document utilisateur trouvé
		mockFileClient.On("GetCurrentUserDocument", mock.Anything, "user-001", "passport").
			Return(&domain.DocumentRef{DocumentID: "doc-passport-001", DocumentType: "passport"}, nil)

		// Persona crée l'inquiry
		expiresAt := time.Now().Add(30 * time.Minute).UTC()
		mockPersonaClient.On("CreateInquiry", mock.Anything, testTemplateID, "user-001").
			Return(&domain.PersonaInquiry{
				InquiryID:    "inq_abc123",
				TemplateID:   testTemplateID,
				SessionToken: "sess_token_xyz",
				ExpiresAt:    expiresAt,
			}, nil)

		// File-service crée la review
		now := time.Now().UTC()
		mockFileClient.On("CreateDocumentReview", mock.Anything, mock.MatchedBy(func(r *domain.Review) bool {
			return r.UserDocumentID == "doc-passport-001" &&
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
			UserDocumentID:    "doc-passport-001",
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

	t.Run("succès - crée une inquiry pour un document véhicule", func(t *testing.T) {
		mockFileClient, mockPersonaClient, svc := newTestService()
		ctx := context.Background()

		// Pas de review active
		mockFileClient.On("GetDocumentReviewsByUserID", mock.Anything, "user-002").
			Return([]*domain.Review{}, nil)

		// Documents véhicule trouvés
		mockFileClient.On("GetVehicleDocuments", mock.Anything, "vehicle-001").
			Return([]*domain.DocumentRef{
				{DocumentID: "vdoc-insurance-001", DocumentType: "insurance"},
				{DocumentID: "vdoc-reg-001", DocumentType: "registrationCard"},
			}, nil)

		expiresAt := time.Now().Add(30 * time.Minute).UTC()
		mockPersonaClient.On("CreateInquiry", mock.Anything, testTemplateID, "user-002").
			Return(&domain.PersonaInquiry{
				InquiryID:    "inq_vehicle_123",
				TemplateID:   testTemplateID,
				SessionToken: "sess_vehicle_token",
				ExpiresAt:    expiresAt,
			}, nil)

		now := time.Now().UTC()
		mockFileClient.On("CreateDocumentReview", mock.Anything, mock.MatchedBy(func(r *domain.Review) bool {
			return r.VehicleDocumentID == "vdoc-insurance-001" &&
				r.UserDocumentID == "" &&
				r.PersonaInquiryID == "inq_vehicle_123" &&
				r.Status == "pending" &&
				r.AttemptNumber == 1
		})).Return(&domain.Review{
			ReviewID:          "review-vehicle-001",
			VehicleDocumentID: "vdoc-insurance-001",
			PersonaInquiryID:  "inq_vehicle_123",
			Status:            "pending",
			AttemptNumber:     1,
			CreatedAt:         now,
		}, nil)

		result, err := svc.CreateInquiry(ctx, serviceInterfaces.CreateInquiryInput{
			UserID:       "user-002",
			DocumentType: "insurance",
			VehicleID:    "vehicle-001",
		})

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "review-vehicle-001", result.ReviewID)
		assert.Equal(t, "inq_vehicle_123", result.PersonaInquiryID)
		assert.Equal(t, int32(1), result.AttemptNumber)

		mockFileClient.AssertExpectations(t)
		mockPersonaClient.AssertExpectations(t)
	})

	t.Run("succès - incrémente attempt_number sur un retry", func(t *testing.T) {
		mockFileClient, mockPersonaClient, svc := newTestService()
		ctx := context.Background()

		// Review précédente terminée (completed) pour le même document
		mockFileClient.On("GetDocumentReviewsByUserID", mock.Anything, "user-003").
			Return([]*domain.Review{
				{
					ReviewID:       "review-old-001",
					UserDocumentID: "doc-id-front-003",
					Status:         "completed",
					Decision:       "rejected",
					AttemptNumber:  2,
				},
			}, nil)

		mockFileClient.On("GetCurrentUserDocument", mock.Anything, "user-003", "idCardFront").
			Return(&domain.DocumentRef{DocumentID: "doc-id-front-003", DocumentType: "idCardFront"}, nil)

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
				r.PreviousReviewID == "review-old-001"
		})).Return(&domain.Review{
			ReviewID:         "review-retry-001",
			UserDocumentID:   "doc-id-front-003",
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
			DocumentType: "passport",
		})

		assert.Nil(t, result)
		assert.ErrorIs(t, err, kycErrors.ErrorFileServiceUnavailable)
		mockFileClient.AssertExpectations(t)
	})

	t.Run("erreur - document utilisateur introuvable", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		ctx := context.Background()

		mockFileClient.On("GetDocumentReviewsByUserID", mock.Anything, "user-007").
			Return([]*domain.Review{}, nil)
		mockFileClient.On("GetCurrentUserDocument", mock.Anything, "user-007", "passport").
			Return(nil, errors.New("document not found"))

		result, err := svc.CreateInquiry(ctx, serviceInterfaces.CreateInquiryInput{
			UserID:       "user-007",
			DocumentType: "passport",
		})

		assert.Nil(t, result)
		assert.ErrorIs(t, err, kycErrors.ErrorFileServiceUnavailable)
		mockFileClient.AssertExpectations(t)
	})

	t.Run("erreur - document véhicule du type demandé introuvable", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		ctx := context.Background()

		mockFileClient.On("GetDocumentReviewsByUserID", mock.Anything, "user-008").
			Return([]*domain.Review{}, nil)
		// Véhicule a des documents mais pas du bon type
		mockFileClient.On("GetVehicleDocuments", mock.Anything, "vehicle-002").
			Return([]*domain.DocumentRef{
				{DocumentID: "vdoc-001", DocumentType: "insurance"},
			}, nil)

		result, err := svc.CreateInquiry(ctx, serviceInterfaces.CreateInquiryInput{
			UserID:       "user-008",
			DocumentType: "registrationCard",
			VehicleID:    "vehicle-002",
		})

		assert.Nil(t, result)
		assert.ErrorIs(t, err, kycErrors.ErrorReviewNotFound)
		mockFileClient.AssertExpectations(t)
	})

	t.Run("erreur - API Persona indisponible", func(t *testing.T) {
		mockFileClient, mockPersonaClient, svc := newTestService()
		ctx := context.Background()

		mockFileClient.On("GetDocumentReviewsByUserID", mock.Anything, "user-009").
			Return([]*domain.Review{}, nil)
		mockFileClient.On("GetCurrentUserDocument", mock.Anything, "user-009", "passport").
			Return(&domain.DocumentRef{DocumentID: "doc-009", DocumentType: "passport"}, nil)

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
		mockFileClient.On("GetCurrentUserDocument", mock.Anything, "user-010", "passport").
			Return(&domain.DocumentRef{DocumentID: "doc-010", DocumentType: "passport"}, nil)

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
					ReviewID:       "review-completed-001",
					UserDocumentID: "doc-011",
					Status:         "completed",
					Decision:       "approved",
					AttemptNumber:  1,
				},
			}, nil)

		mockFileClient.On("GetCurrentUserDocument", mock.Anything, "user-011", "passport").
			Return(&domain.DocumentRef{DocumentID: "doc-011-new", DocumentType: "passport"}, nil)

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
				AttemptNumber:    1,
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
}

// =============================================================================
// GetInquiry
// =============================================================================

func TestGetInquiry(t *testing.T) {
	t.Run("succès - retourne le détail d'une inquiry utilisateur", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		ctx := context.Background()

		now := time.Now().UTC()
		reviewedAt := now.Add(-1 * time.Hour)
		review := &domain.Review{
			ReviewID:          "review-get-001",
			UserDocumentID:    "doc-passport-get",
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

		// Ownership check — la review est dans les reviews de l'utilisateur
		mockFileClient.On("GetDocumentReviewsByUserID", mock.Anything, "user-get-001").
			Return([]*domain.Review{review}, nil)

		result, err := svc.GetInquiry(ctx, "user-get-001", "inq_get_001")

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "review-get-001", result.ReviewID)
		assert.Equal(t, "inq_get_001", result.PersonaInquiryID)
		assert.Equal(t, "doc-passport-get", result.UserDocumentID)
		assert.Equal(t, "completed", result.Status)
		assert.Equal(t, "approved", result.Decision)
		assert.Equal(t, int32(1), result.AttemptNumber)
		assert.Equal(t, "automatic", result.ReviewType)
		assert.NotEmpty(t, result.ReviewedAt)
		assert.NotEmpty(t, result.CreatedAt)
		assert.NotEmpty(t, result.UpdatedAt)

		mockFileClient.AssertExpectations(t)
	})

	t.Run("succès - retourne une inquiry véhicule", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		ctx := context.Background()

		now := time.Now().UTC()
		review := &domain.Review{
			ReviewID:          "review-vehicle-get",
			VehicleDocumentID: "vdoc-insurance-get",
			PersonaInquiryID:  "inq_vehicle_get",
			PersonaTemplateID: testTemplateID,
			Status:            "pending",
			AttemptNumber:     1,
			ReviewType:        "automatic",
			CreatedAt:         now,
			UpdatedAt:         now,
		}

		mockFileClient.On("GetDocumentReviewByPersonaInquiryID", mock.Anything, "inq_vehicle_get").
			Return(review, nil)

		mockFileClient.On("GetDocumentReviewsByUserID", mock.Anything, "user-vehicle-get").
			Return([]*domain.Review{review}, nil)

		result, err := svc.GetInquiry(ctx, "user-vehicle-get", "inq_vehicle_get")

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "vdoc-insurance-get", result.VehicleDocumentID)
		assert.Empty(t, result.UserDocumentID)
		assert.Empty(t, result.ReviewedAt) // pas encore reviewé

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

	t.Run("erreur 403 - inquiry n'appartient pas à l'utilisateur", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		ctx := context.Background()

		now := time.Now().UTC()
		review := &domain.Review{
			ReviewID:         "review-other-user",
			UserDocumentID:   "doc-other-user",
			PersonaInquiryID: "inq_other_user",
			Status:           "pending",
			AttemptNumber:    1,
			CreatedAt:        now,
			UpdatedAt:        now,
		}

		mockFileClient.On("GetDocumentReviewByPersonaInquiryID", mock.Anything, "inq_other_user").
			Return(review, nil)

		// L'utilisateur n'a pas cette review
		mockFileClient.On("GetDocumentReviewsByUserID", mock.Anything, "user-impostor").
			Return([]*domain.Review{}, nil)

		result, err := svc.GetInquiry(ctx, "user-impostor", "inq_other_user")

		assert.Nil(t, result)
		assert.ErrorIs(t, err, kycErrors.ErrorUnauthorized)
		mockFileClient.AssertExpectations(t)
	})

	t.Run("erreur - file-service indisponible lors du ownership check", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		ctx := context.Background()

		now := time.Now().UTC()
		review := &domain.Review{
			ReviewID:         "review-fs-fail",
			UserDocumentID:   "doc-fs-fail",
			PersonaInquiryID: "inq_fs_fail",
			Status:           "pending",
			CreatedAt:        now,
			UpdatedAt:        now,
		}

		mockFileClient.On("GetDocumentReviewByPersonaInquiryID", mock.Anything, "inq_fs_fail").
			Return(review, nil)

		mockFileClient.On("GetDocumentReviewsByUserID", mock.Anything, "user-fs-fail").
			Return(nil, errors.New("connection refused"))

		result, err := svc.GetInquiry(ctx, "user-fs-fail", "inq_fs_fail")

		assert.Nil(t, result)
		assert.ErrorIs(t, err, kycErrors.ErrorFileServiceUnavailable)
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
					UserDocumentID:   "doc-passport-001",
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

		mockFileClient.On("GetUserDocuments", mock.Anything, "user-status-001").
			Return([]*domain.DocumentRef{
				{DocumentID: "doc-passport-001", DocumentType: "passport"},
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
					ReviewID:       "review-id-approved",
					UserDocumentID: "doc-idfront-002",
					Status:         "completed",
					Decision:       "approved",
					ReviewType:     "automatic",
					ReviewedAt:     &reviewedAt,
					AttemptNumber:  1,
					CreatedAt:      now,
					UpdatedAt:      now,
				},
				{
					ReviewID:       "review-dl-approved",
					UserDocumentID: "doc-dlfront-002",
					Status:         "completed",
					Decision:       "approved",
					ReviewType:     "automatic",
					ReviewedAt:     &reviewedAt,
					AttemptNumber:  1,
					CreatedAt:      now,
					UpdatedAt:      now,
				},
			}, nil)

		mockFileClient.On("GetUserDocuments", mock.Anything, "user-status-002").
			Return([]*domain.DocumentRef{
				{DocumentID: "doc-idfront-002", DocumentType: "idCardFront"},
				{DocumentID: "doc-dlfront-002", DocumentType: "driverLicenceFront"},
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
					ReviewID:       "review-dl-only",
					UserDocumentID: "doc-dl-003",
					Status:         "completed",
					Decision:       "approved",
					ReviewType:     "automatic",
					ReviewedAt:     &reviewedAt,
					AttemptNumber:  1,
					CreatedAt:      now,
					UpdatedAt:      now,
				},
			}, nil)

		mockFileClient.On("GetUserDocuments", mock.Anything, "user-status-003").
			Return([]*domain.DocumentRef{
				{DocumentID: "doc-dl-003", DocumentType: "driverLicenceFront"},
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
					UserDocumentID:   "doc-pending-004",
					PersonaInquiryID: "inq_pending",
					Status:           "pending",
					AttemptNumber:    1,
					SessionExpiresAt: &expiresAt,
					CreatedAt:        now,
					UpdatedAt:        now,
				},
				{
					ReviewID:         "review-submitted-001",
					UserDocumentID:   "doc-submitted-004",
					PersonaInquiryID: "inq_submitted",
					Status:           "submitted",
					AttemptNumber:    2,
					CreatedAt:        now,
					UpdatedAt:        now,
				},
			}, nil)

		mockFileClient.On("GetUserDocuments", mock.Anything, "user-status-004").
			Return([]*domain.DocumentRef{}, nil)

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
					UserDocumentID:   "doc-005",
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
					UserDocumentID:   "doc-005",
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

		mockFileClient.On("GetUserDocuments", mock.Anything, "user-status-005").
			Return([]*domain.DocumentRef{}, nil)

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
		mockFileClient.On("GetUserDocuments", mock.Anything, "user-status-006").
			Return([]*domain.DocumentRef{}, nil)

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
		UserDocumentID:      "doc-001",
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
		mockFileClient.On("GetDocumentReviewsByUserID", mock.Anything, "user-001").
			Return([]*domain.Review{baseReview}, nil)
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
			UserDocumentID:   "doc-001",
			PersonaInquiryID: "inq_resume_002",
			Status:           "inProgress",
			AttemptNumber:    2,
			CreatedAt:        now,
			UpdatedAt:        now,
		}

		mockFileClient.On("GetDocumentReviewByPersonaInquiryID", mock.Anything, "inq_resume_002").
			Return(inProgressReview, nil)
		mockFileClient.On("GetDocumentReviewsByUserID", mock.Anything, "user-001").
			Return([]*domain.Review{inProgressReview}, nil)
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
			UserDocumentID:   "doc-001",
			PersonaInquiryID: "inq_resume_003",
			Status:           "submitted",
			AttemptNumber:    1,
			CreatedAt:        now,
			UpdatedAt:        now,
		}

		mockFileClient.On("GetDocumentReviewByPersonaInquiryID", mock.Anything, "inq_resume_003").
			Return(submittedReview, nil)
		mockFileClient.On("GetDocumentReviewsByUserID", mock.Anything, "user-001").
			Return([]*domain.Review{submittedReview}, nil)
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
		// L'utilisateur n'a pas cette review
		mockFileClient.On("GetDocumentReviewsByUserID", mock.Anything, "user-other").
			Return([]*domain.Review{}, nil)

		result, err := svc.ResumeInquiry(ctx, "user-other", "inq_resume_001")
		assert.Nil(t, result)
		assert.ErrorIs(t, err, kycErrors.ErrorUnauthorized)
	})

	t.Run("erreur 410 - inquiry completed non resumable", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		ctx := context.Background()

		completedReview := &domain.Review{
			ReviewID:         "review-completed",
			UserDocumentID:   "doc-001",
			PersonaInquiryID: "inq_completed",
			Status:           "completed",
			CreatedAt:        now,
			UpdatedAt:        now,
		}

		mockFileClient.On("GetDocumentReviewByPersonaInquiryID", mock.Anything, "inq_completed").
			Return(completedReview, nil)
		mockFileClient.On("GetDocumentReviewsByUserID", mock.Anything, "user-001").
			Return([]*domain.Review{completedReview}, nil)

		result, err := svc.ResumeInquiry(ctx, "user-001", "inq_completed")
		assert.Nil(t, result)
		assert.ErrorIs(t, err, kycErrors.ErrorInquiryNotResumable)
	})

	t.Run("erreur 410 - inquiry expired non resumable", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		ctx := context.Background()

		expiredReview := &domain.Review{
			ReviewID:         "review-expired",
			UserDocumentID:   "doc-001",
			PersonaInquiryID: "inq_expired",
			Status:           "expired",
			CreatedAt:        now,
			UpdatedAt:        now,
		}

		mockFileClient.On("GetDocumentReviewByPersonaInquiryID", mock.Anything, "inq_expired").
			Return(expiredReview, nil)
		mockFileClient.On("GetDocumentReviewsByUserID", mock.Anything, "user-001").
			Return([]*domain.Review{expiredReview}, nil)

		result, err := svc.ResumeInquiry(ctx, "user-001", "inq_expired")
		assert.Nil(t, result)
		assert.ErrorIs(t, err, kycErrors.ErrorInquiryNotResumable)
	})

	t.Run("erreur 410 - inquiry failed non resumable", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		ctx := context.Background()

		failedReview := &domain.Review{
			ReviewID:         "review-failed",
			UserDocumentID:   "doc-001",
			PersonaInquiryID: "inq_failed",
			Status:           "failed",
			CreatedAt:        now,
			UpdatedAt:        now,
		}

		mockFileClient.On("GetDocumentReviewByPersonaInquiryID", mock.Anything, "inq_failed").
			Return(failedReview, nil)
		mockFileClient.On("GetDocumentReviewsByUserID", mock.Anything, "user-001").
			Return([]*domain.Review{failedReview}, nil)

		result, err := svc.ResumeInquiry(ctx, "user-001", "inq_failed")
		assert.Nil(t, result)
		assert.ErrorIs(t, err, kycErrors.ErrorInquiryNotResumable)
	})

	t.Run("erreur - Persona indisponible lors du renouvellement", func(t *testing.T) {
		mockFileClient, mockPersonaClient, svc := newTestService()
		ctx := context.Background()

		mockFileClient.On("GetDocumentReviewByPersonaInquiryID", mock.Anything, "inq_resume_001").
			Return(baseReview, nil)
		mockFileClient.On("GetDocumentReviewsByUserID", mock.Anything, "user-001").
			Return([]*domain.Review{baseReview}, nil)
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
		mockFileClient.On("GetDocumentReviewsByUserID", mock.Anything, "user-001").
			Return([]*domain.Review{baseReview}, nil)
		mockPersonaClient.On("ResumeInquiry", mock.Anything, "inq_resume_001").
			Return(&domain.PersonaSession{SessionToken: "new-token", ExpiresAt: newExpiry}, nil)
		mockFileClient.On("UpdateDocumentReview", mock.Anything, mock.AnythingOfType("*domain.Review")).
			Return(nil, errors.New("file-service down"))

		result, err := svc.ResumeInquiry(ctx, "user-001", "inq_resume_001")
		assert.Nil(t, result)
		assert.ErrorIs(t, err, kycErrors.ErrorFileServiceUnavailable)
	})

	t.Run("erreur - file-service indisponible lors du ownership check", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		ctx := context.Background()

		mockFileClient.On("GetDocumentReviewByPersonaInquiryID", mock.Anything, "inq_resume_001").
			Return(baseReview, nil)
		mockFileClient.On("GetDocumentReviewsByUserID", mock.Anything, "user-001").
			Return(nil, errors.New("file-service down"))

		result, err := svc.ResumeInquiry(ctx, "user-001", "inq_resume_001")
		assert.Nil(t, result)
		assert.ErrorIs(t, err, kycErrors.ErrorFileServiceUnavailable)
	})
}

// =============================================================================
// ProcessWebhook
// =============================================================================

// computeHMAC calcule la signature HMAC-SHA256 pour les tests de webhook
func computeHMAC(payload []byte) string {
	mac := hmac.New(sha256.New, []byte(testWebhookSecret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

func TestProcessWebhook(t *testing.T) {
	now := time.Now().UTC()
	payload := []byte(`{"data":{"id":"inq_wh_001"}}`)
	validSignature := computeHMAC(payload)

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
			Signature:         computeHMAC(payload),
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
			Signature:         computeHMAC(payload),
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
		customSig := computeHMAC(customPayload)

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

	newCompletedReview := func() *domain.Review {
		return &domain.Review{
			ReviewID:         "review-override-001",
			PersonaInquiryID: "inq_override_001",
			UserDocumentID:   "doc-001",
			Status:           "completed",
			Decision:         "approved",
			ReviewType:       "automatic",
			AttemptNumber:    1,
			CreatedAt:        now,
			UpdatedAt:        now,
		}
	}

	t.Run("succès - override approved → rejected", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		ctx := context.Background()

		mockFileClient.On("GetDocumentReview", mock.Anything, "review-override-001").
			Return(newCompletedReview(), nil)
		mockFileClient.On("UpdateDocumentReview", mock.Anything, mock.MatchedBy(func(r *domain.Review) bool {
			return r.Decision == "rejected" &&
				r.ReasonRejection == "document_expired" &&
				r.RejectionDetails == "ID card expired" &&
				r.ReviewedBy == "admin-001" &&
				r.ReviewType == "manual" &&
				r.ReviewedAt != nil &&
				r.Notes == "Override after manual check"
		})).Return(&domain.Review{
			ReviewID:         "review-override-001",
			PersonaInquiryID: "inq_override_001",
			Decision:         "rejected",
			ReasonRejection:  "document_expired",
			RejectionDetails: "ID card expired",
			ReviewedBy:       "admin-001",
			ReviewType:       "manual",
			ReviewedAt:       &now,
			Notes:            "Override after manual check",
			UpdatedAt:        now,
		}, nil)

		result, err := svc.OverrideReview(ctx, serviceInterfaces.OverrideReviewInput{
			UserID:           "admin-001",
			ReviewID:         "review-override-001",
			Decision:         "rejected",
			ReasonRejection:  "document_expired",
			RejectionDetails: "ID card expired",
			Notes:            "Override after manual check",
		})
		require.NoError(t, err)
		assert.Equal(t, "review-override-001", result.ReviewID)
		assert.Equal(t, "rejected", result.Decision)
		assert.Equal(t, "document_expired", result.ReasonRejection)
		assert.Equal(t, "manual", result.ReviewType)
		assert.Equal(t, "admin-001", result.ReviewedBy)
		assert.NotEmpty(t, result.ReviewedAt)
	})

	t.Run("succès - override rejected → approved", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		ctx := context.Background()

		rejectedReview := newCompletedReview()
		rejectedReview.Decision = "rejected"

		mockFileClient.On("GetDocumentReview", mock.Anything, "review-override-001").
			Return(rejectedReview, nil)
		mockFileClient.On("UpdateDocumentReview", mock.Anything, mock.MatchedBy(func(r *domain.Review) bool {
			return r.Decision == "approved" && r.ReviewType == "manual"
		})).Return(&domain.Review{
			ReviewID:         "review-override-001",
			PersonaInquiryID: "inq_override_001",
			Decision:         "approved",
			ReviewedBy:       "admin-001",
			ReviewType:       "manual",
			ReviewedAt:       &now,
			UpdatedAt:        now,
		}, nil)

		result, err := svc.OverrideReview(ctx, serviceInterfaces.OverrideReviewInput{
			UserID:   "admin-001",
			ReviewID: "review-override-001",
			Decision: "approved",
		})
		require.NoError(t, err)
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
		mockFileClient.On("UpdateDocumentReview", mock.Anything, mock.AnythingOfType("*domain.Review")).
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
