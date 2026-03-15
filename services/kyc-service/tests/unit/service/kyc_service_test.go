package service_test

import (
	"context"
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
func newTestService() (*mocks.MockFileServiceClient, *mocks.MockPersonaClient, serviceInterfaces.KYCService) {
	mockFileClient := new(mocks.MockFileServiceClient)
	mockPersonaClient := new(mocks.MockPersonaClient)
	svc := service.NewKYCService(mockFileClient, mockPersonaClient, testTemplateID, zap.NewNop())
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
			ReviewID:         "review-001",
			UserDocumentID:   "doc-passport-001",
			PersonaInquiryID: "inq_abc123",
			PersonaTemplateID: testTemplateID,
			Status:           "pending",
			AttemptNumber:    1,
			CreatedAt:        now,
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
	t.Run("placeholder - service instancié correctement", func(t *testing.T) {
		_, _, svc := newTestService()
		if svc == nil {
			t.Fatal("service should not be nil")
		}
	})
}

// =============================================================================
// GetKYCStatus
// =============================================================================

func TestGetKYCStatus(t *testing.T) {
	t.Run("placeholder - service instancié correctement", func(t *testing.T) {
		_, _, svc := newTestService()
		if svc == nil {
			t.Fatal("service should not be nil")
		}
	})
}

// =============================================================================
// ResumeInquiry
// =============================================================================

func TestResumeInquiry(t *testing.T) {
	t.Run("placeholder - service instancié correctement", func(t *testing.T) {
		_, _, svc := newTestService()
		if svc == nil {
			t.Fatal("service should not be nil")
		}
	})
}

// =============================================================================
// ProcessWebhook
// =============================================================================

func TestProcessWebhook(t *testing.T) {
	t.Run("placeholder - service instancié correctement", func(t *testing.T) {
		_, _, svc := newTestService()
		if svc == nil {
			t.Fatal("service should not be nil")
		}
	})
}

// =============================================================================
// GetAdminReviews
// =============================================================================

func TestGetAdminReviews(t *testing.T) {
	t.Run("placeholder - service instancié correctement", func(t *testing.T) {
		_, _, svc := newTestService()
		if svc == nil {
			t.Fatal("service should not be nil")
		}
	})
}

// =============================================================================
// GetAdminReview
// =============================================================================

func TestGetAdminReview(t *testing.T) {
	t.Run("placeholder - service instancié correctement", func(t *testing.T) {
		_, _, svc := newTestService()
		if svc == nil {
			t.Fatal("service should not be nil")
		}
	})
}

// =============================================================================
// OverrideReview
// =============================================================================

func TestOverrideReview(t *testing.T) {
	t.Run("placeholder - service instancié correctement", func(t *testing.T) {
		_, _, svc := newTestService()
		if svc == nil {
			t.Fatal("service should not be nil")
		}
	})
}
