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

// --- Helpers ---

// newTestService crée le couple (fileClient, userClient) mocké et le service.
// supportClient et vehicleClient nil : dégradation gracieuse.
func newTestService() (*mocks.MockFileServiceClient, *mocks.MockUserClient, serviceInterfaces.KYCService) {
	mockFileClient := new(mocks.MockFileServiceClient)
	mockUserClient := new(mocks.MockUserClient)
	// Pass-through : les tests utilisent des IDs "user-XXX" directement comme
	// si le Firebase UID et l'UserID interne étaient identiques.
	mockUserClient.On("GetUserIDByFirebaseID", mock.Anything, mock.AnythingOfType("string")).
		Return(func(_ context.Context, firebaseUID string) string { return firebaseUID }, nil).Maybe()
	// Propagation best-effort après ValidateDocument / OverrideReview :
	// recharge les documents courants puis pousse les flags. Optionnelle (.Maybe()).
	mockFileClient.On("GetUserDocumentSummaries", mock.Anything, mock.AnythingOfType("string")).
		Return([]*domain.DocumentSummary{}, nil).Maybe()
	mockFileClient.On("GetVehicleDocumentSummariesByUserID", mock.Anything, mock.AnythingOfType("string")).
		Return([]*domain.DocumentSummary{}, nil).Maybe()
	mockUserClient.On("UpdateProfileVerification", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(false, false, nil).Maybe()
	svc := service.NewKYCService(mockFileClient, mockUserClient, nil, nil, nil, zap.NewNop())
	return mockFileClient, mockUserClient, svc
}

func timePtr(t time.Time) *time.Time {
	return &t
}

// =============================================================================
// GetKYCStatus — vérification calculée depuis les documents COURANTS
// =============================================================================

func TestGetKYCStatus(t *testing.T) {
	userSummary := func(docType, status string) *domain.DocumentSummary {
		return &domain.DocumentSummary{DocumentType: docType, Status: status, OwnerKind: "user", OwnerID: "u", IsCurrent: true}
	}
	vehicleSummary := func(docType, status, vehicleID string) *domain.DocumentSummary {
		return &domain.DocumentSummary{DocumentType: docType, Status: status, OwnerKind: "vehicle", OwnerID: vehicleID, IsCurrent: true}
	}

	// mockDocs installe les réponses documents (écrase les .Maybe() par défaut).
	mockDocs := func(fc *mocks.MockFileServiceClient, userID string, userDocs, vehicleDocs []*domain.DocumentSummary) {
		fc.ExpectedCalls = nil
		fc.On("GetUserDocumentSummaries", mock.Anything, userID).Return(userDocs, nil)
		fc.On("GetVehicleDocumentSummariesByUserID", mock.Anything, userID).Return(vehicleDocs, nil)
	}

	t.Run("selfie + passeport approuvés → identité vérifiée, pas conducteur", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		mockDocs(mockFileClient, "user-status-001",
			[]*domain.DocumentSummary{userSummary("selfie", "approved"), userSummary("passport", "approved")}, nil)
		mockFileClient.On("GetDocumentReviewsByUserID", mock.Anything, "user-status-001").
			Return([]*domain.Review{}, nil)

		result, err := svc.GetKYCStatus(context.Background(), "user-status-001")

		require.NoError(t, err)
		assert.True(t, result.IdentityVerified)
		assert.False(t, result.DriverVerified)
	})

	t.Run("pièce approuvée mais selfie manquant → identité NON vérifiée", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		mockDocs(mockFileClient, "user-status-002",
			[]*domain.DocumentSummary{userSummary("passport", "approved")}, nil)
		mockFileClient.On("GetDocumentReviewsByUserID", mock.Anything, "user-status-002").
			Return([]*domain.Review{}, nil)

		result, err := svc.GetKYCStatus(context.Background(), "user-status-002")

		require.NoError(t, err)
		assert.False(t, result.IdentityVerified)
		assert.False(t, result.DriverVerified)
	})

	t.Run("selfie + permis + véhicule entièrement validé → conducteur vérifié", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		mockDocs(mockFileClient, "user-status-003",
			[]*domain.DocumentSummary{
				userSummary("selfie", "approved"),
				userSummary("driverLicenceFront", "approved"),
				userSummary("driverLicenceBack", "approved"),
			},
			[]*domain.DocumentSummary{
				vehicleSummary("insurance", "approved", "veh1"),
				vehicleSummary("registrationCard", "approved", "veh1"),
			})
		mockFileClient.On("GetDocumentReviewsByUserID", mock.Anything, "user-status-003").
			Return([]*domain.Review{}, nil)

		result, err := svc.GetKYCStatus(context.Background(), "user-status-003")

		require.NoError(t, err)
		assert.True(t, result.IdentityVerified, "le permis vaut pièce d'identité (double rôle)")
		assert.True(t, result.DriverVerified)
	})

	t.Run("permis + véhicule OK mais selfie en attente → rien de vérifié", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		mockDocs(mockFileClient, "user-status-004",
			[]*domain.DocumentSummary{
				userSummary("selfie", "pending"),
				userSummary("driverLicenceFront", "approved"),
				userSummary("driverLicenceBack", "approved"),
			},
			[]*domain.DocumentSummary{
				vehicleSummary("insurance", "approved", "veh1"),
				vehicleSummary("registrationCard", "approved", "veh1"),
			})
		mockFileClient.On("GetDocumentReviewsByUserID", mock.Anything, "user-status-004").
			Return([]*domain.Review{}, nil)

		result, err := svc.GetKYCStatus(context.Background(), "user-status-004")

		require.NoError(t, err)
		assert.False(t, result.IdentityVerified)
		assert.False(t, result.DriverVerified)
	})

	t.Run("pending reviews retournées", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		now := time.Now().UTC()
		mockDocs(mockFileClient, "user-status-005", nil, nil)
		mockFileClient.On("GetDocumentReviewsByUserID", mock.Anything, "user-status-005").
			Return([]*domain.Review{
				{ReviewID: "review-pending-001", DocumentType: "passport", Status: "pending", AttemptNumber: 1, CreatedAt: now, UpdatedAt: now},
				{ReviewID: "review-pending-002", DocumentType: "selfie", Status: "pending", AttemptNumber: 2, CreatedAt: now, UpdatedAt: now},
			}, nil)

		result, err := svc.GetKYCStatus(context.Background(), "user-status-005")

		require.NoError(t, err)
		require.Len(t, result.PendingReviews, 2)
		assert.Equal(t, "review-pending-001", result.PendingReviews[0].ReviewID)
		assert.Equal(t, "passport", result.PendingReviews[0].DocumentType)
		assert.Equal(t, "selfie", result.PendingReviews[1].DocumentType)
	})

	t.Run("latest rejection retournée (la plus récente)", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		now := time.Now().UTC()
		older := now.Add(-48 * time.Hour)
		newer := now.Add(-1 * time.Hour)
		mockDocs(mockFileClient, "user-status-006", nil, nil)
		mockFileClient.On("GetDocumentReviewsByUserID", mock.Anything, "user-status-006").
			Return([]*domain.Review{
				{ReviewID: "review-rejected-old", DocumentType: "passport", Status: "completed", Decision: "rejected",
					ReasonRejection: "document_expired", ReviewedAt: &older, CreatedAt: now, UpdatedAt: now},
				{ReviewID: "review-rejected-new", DocumentType: "passport", Status: "completed", Decision: "rejected",
					ReasonRejection: "photo_missmatch", RejectionDetails: "La photo ne correspond pas",
					ReviewedAt: &newer, CreatedAt: now, UpdatedAt: now},
			}, nil)

		result, err := svc.GetKYCStatus(context.Background(), "user-status-006")

		require.NoError(t, err)
		require.NotNil(t, result.LatestRejection)
		assert.Equal(t, "review-rejected-new", result.LatestRejection.ReviewID)
		assert.Equal(t, "photo_missmatch", result.LatestRejection.ReasonRejection)
	})

	t.Run("user_id vide → ErrorMissingUserID", func(t *testing.T) {
		_, _, svc := newTestService()
		result, err := svc.GetKYCStatus(context.Background(), "")
		assert.Nil(t, result)
		assert.ErrorIs(t, err, kycErrors.ErrorMissingUserID)
	})

	t.Run("file-service indisponible → erreur", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		mockFileClient.ExpectedCalls = nil
		mockFileClient.On("GetDocumentReviewsByUserID", mock.Anything, "user-status-fail").
			Return(nil, errors.New("connection refused"))

		result, err := svc.GetKYCStatus(context.Background(), "user-status-fail")

		assert.Nil(t, result)
		assert.ErrorIs(t, err, kycErrors.ErrorFileServiceUnavailable)
	})
}

// =============================================================================
// ValidateDocument
// =============================================================================

func TestValidateDocument(t *testing.T) {
	t.Run("succès - review pending existante (document utilisateur) mise à jour via Update", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		ctx := context.Background()

		mockFileClient.On("GetUserDocument", mock.Anything, "doc-1").
			Return(&domain.DocumentRef{DocumentID: "doc-1", DocumentType: "passport", OwnerID: "user-1"}, nil)

		pendingReview := &domain.Review{
			ReviewID:            "review-pending-1",
			UserID:              "user-1",
			DocumentType:        "passport",
			LogicalDocumentType: "passport",
			Status:              "pending",
			Decision:            "pending",
			ReviewType:          "manual",
		}
		mockFileClient.On("GetDocumentReviewsByUserID", mock.Anything, "user-1").
			Return([]*domain.Review{pendingReview}, nil)
		mockFileClient.On("UpdateDocumentReview", mock.Anything, mock.MatchedBy(func(r *domain.Review) bool {
			return r.ReviewID == "review-pending-1" &&
				r.Status == "completed" &&
				r.Decision == "approved" &&
				r.ReviewedBy == "agent-001" &&
				r.ReviewType == "manual"
		})).Return(&domain.Review{
			ReviewID:            "review-pending-1",
			Decision:            "approved",
			ReviewedBy:          "agent-001",
			ReviewType:          "manual",
			Status:              "completed",
			UserDocumentID:      "doc-1",
			LogicalDocumentType: "passport",
			ReviewedAt:          timePtr(time.Now().UTC()),
		}, nil)

		result, err := svc.ValidateDocument(ctx, serviceInterfaces.ValidateDocumentInput{
			SupportAgentID: "agent-001",
			DocumentID:     "doc-1",
			Decision:       "approved",
		})

		require.NoError(t, err)
		assert.Equal(t, "review-pending-1", result.ReviewID)
		assert.Equal(t, "approved", result.Decision)
		assert.Equal(t, "doc-1", result.DocumentID)
		assert.Equal(t, "passport", result.LogicalDocumentType)
		mockFileClient.AssertNotCalled(t, "CreateDocumentReview", mock.Anything, mock.Anything)
	})

	t.Run("succès - recto-verso : le compagnon est résolu et transmis", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		ctx := context.Background()

		mockFileClient.On("GetUserDocument", mock.Anything, "front-1").
			Return(&domain.DocumentRef{DocumentID: "front-1", DocumentType: "idCardFront", OwnerID: "user-rv"}, nil)
		mockFileClient.On("GetCurrentUserDocument", mock.Anything, "user-rv", "idCardBack").
			Return(&domain.DocumentRef{DocumentID: "back-1", DocumentType: "idCardBack"}, nil)
		pendingReview := &domain.Review{
			ReviewID:            "review-rv",
			UserID:              "user-rv",
			DocumentType:        "idCardFront",
			LogicalDocumentType: "idCard",
			UserDocumentID:      "front-1",
			Status:              "pending",
			Decision:            "pending",
		}
		mockFileClient.On("GetDocumentReviewsByUserID", mock.Anything, "user-rv").
			Return([]*domain.Review{pendingReview}, nil)
		mockFileClient.On("UpdateDocumentReview", mock.Anything, mock.MatchedBy(func(r *domain.Review) bool {
			return r.ReviewID == "review-rv" && r.SecondUserDocumentID == "back-1"
		})).Return(&domain.Review{
			ReviewID:             "review-rv",
			Decision:             "approved",
			Status:               "completed",
			UserDocumentID:       "front-1",
			SecondUserDocumentID: "back-1",
			LogicalDocumentType:  "idCard",
			ReviewedAt:           timePtr(time.Now().UTC()),
		}, nil)

		result, err := svc.ValidateDocument(ctx, serviceInterfaces.ValidateDocumentInput{
			SupportAgentID: "agent-001",
			DocumentID:     "front-1",
			Decision:       "approved",
		})

		require.NoError(t, err)
		assert.Equal(t, "front-1", result.DocumentID)
		assert.Equal(t, "back-1", result.SecondDocumentID)
		assert.Equal(t, "idCard", result.LogicalDocumentType)
	})

	t.Run("succès - le VERSO fourni est normalisé vers le recto (unité logique)", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		ctx := context.Background()

		mockFileClient.On("GetUserDocument", mock.Anything, "back-2").
			Return(&domain.DocumentRef{DocumentID: "back-2", DocumentType: "driverLicenceBack", OwnerID: "user-vb"}, nil)
		// Normalisation verso → recto
		mockFileClient.On("GetCurrentUserDocument", mock.Anything, "user-vb", "driverLicenceFront").
			Return(&domain.DocumentRef{DocumentID: "front-2", DocumentType: "driverLicenceFront"}, nil)
		// Résolution du compagnon (le verso courant) pour le recto normalisé
		mockFileClient.On("GetCurrentUserDocument", mock.Anything, "user-vb", "driverLicenceBack").
			Return(&domain.DocumentRef{DocumentID: "back-2", DocumentType: "driverLicenceBack"}, nil)
		mockFileClient.On("GetDocumentReviewsByUserID", mock.Anything, "user-vb").
			Return([]*domain.Review{}, nil)
		mockFileClient.On("CreateDocumentReview", mock.Anything, mock.MatchedBy(func(r *domain.Review) bool {
			// La review est créée sur le RECTO, le verso en second
			return r.DocumentType == "driverLicenceFront" &&
				r.UserDocumentID == "front-2" &&
				r.SecondUserDocumentID == "back-2"
		})).Return(&domain.Review{
			ReviewID:             "review-norm",
			Decision:             "approved",
			Status:               "completed",
			UserDocumentID:       "front-2",
			SecondUserDocumentID: "back-2",
			LogicalDocumentType:  "driverLicence",
			ReviewedAt:           timePtr(time.Now().UTC()),
		}, nil)

		result, err := svc.ValidateDocument(ctx, serviceInterfaces.ValidateDocumentInput{
			SupportAgentID: "agent-001",
			DocumentID:     "back-2", // l'agent fournit le verso
			Decision:       "approved",
		})

		require.NoError(t, err)
		assert.Equal(t, "front-2", result.DocumentID)
		assert.Equal(t, "back-2", result.SecondDocumentID)
	})

	t.Run("succès - review pending existante (document véhicule) mise à jour via Update", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		ctx := context.Background()

		mockFileClient.On("GetVehicleDocument", mock.Anything, "vdoc-1").
			Return(&domain.DocumentRef{DocumentID: "vdoc-1", DocumentType: "insurance", OwnerID: "vehicle-1", UserID: "user-3"}, nil)

		pendingReview := &domain.Review{
			ReviewID:            "review-pending-2",
			UserID:              "user-3",
			DocumentType:        "insurance",
			LogicalDocumentType: "insurance",
			VehicleDocumentID:   "vdoc-1",
			Status:              "pending",
			Decision:            "pending",
			ReviewType:          "manual",
		}
		mockFileClient.On("GetDocumentReviewsByUserID", mock.Anything, "user-3").
			Return([]*domain.Review{pendingReview}, nil)
		mockFileClient.On("UpdateDocumentReview", mock.Anything, mock.MatchedBy(func(r *domain.Review) bool {
			return r.ReviewID == "review-pending-2" &&
				r.Status == "completed" &&
				r.Decision == "rejected" &&
				r.ReasonRejection == "document_expired"
		})).Return(&domain.Review{
			ReviewID:          "review-pending-2",
			Decision:          "rejected",
			ReviewedBy:        "agent-001",
			ReviewType:        "manual",
			Status:            "completed",
			VehicleDocumentID: "vdoc-1",
			ReviewedAt:        timePtr(time.Now().UTC()),
		}, nil)

		result, err := svc.ValidateDocument(ctx, serviceInterfaces.ValidateDocumentInput{
			SupportAgentID:  "agent-001",
			DocumentID:      "vdoc-1",
			VehicleID:       "vehicle-1",
			Decision:        "rejected",
			ReasonRejection: "document_expired",
		})

		require.NoError(t, err)
		assert.Equal(t, "review-pending-2", result.ReviewID)
		assert.Equal(t, "vdoc-1", result.DocumentID)
		mockFileClient.AssertNotCalled(t, "CreateDocumentReview", mock.Anything, mock.Anything)
	})

	t.Run("erreur - decision \"pending\" refusée (état cul-de-sac)", func(t *testing.T) {
		_, _, svc := newTestService()
		result, err := svc.ValidateDocument(context.Background(), serviceInterfaces.ValidateDocumentInput{
			SupportAgentID: "agent-001",
			DocumentID:     "doc-x",
			Decision:       "pending",
		})
		assert.Nil(t, result)
		assert.ErrorIs(t, err, kycErrors.ErrorInvalidDecision)
	})

	t.Run("erreur - document inconnu → ErrorDocumentNotFound (pas 503)", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		mockFileClient.On("GetUserDocument", mock.Anything, "doc-unknown").
			Return(nil, kycErrors.ErrorDocumentNotFound)

		result, err := svc.ValidateDocument(context.Background(), serviceInterfaces.ValidateDocumentInput{
			SupportAgentID: "agent-001",
			DocumentID:     "doc-unknown",
			Decision:       "approved",
		})
		assert.Nil(t, result)
		assert.ErrorIs(t, err, kycErrors.ErrorDocumentNotFound)
	})

	t.Run("erreur - review déjà complétée pour ce document renvoie ErrorDocumentAlreadyReviewed", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		ctx := context.Background()

		mockFileClient.On("GetUserDocument", mock.Anything, "doc-3").
			Return(&domain.DocumentRef{DocumentID: "doc-3", DocumentType: "passport", OwnerID: "user-4"}, nil)
		mockFileClient.On("GetDocumentReviewsByUserID", mock.Anything, "user-4").
			Return([]*domain.Review{
				{ReviewID: "review-done", UserID: "user-4", LogicalDocumentType: "passport", Status: "completed", Decision: "approved"},
			}, nil)

		result, err := svc.ValidateDocument(ctx, serviceInterfaces.ValidateDocumentInput{
			SupportAgentID: "agent-001",
			DocumentID:     "doc-3",
			Decision:       "approved",
		})

		assert.Nil(t, result)
		assert.ErrorIs(t, err, kycErrors.ErrorDocumentAlreadyReviewed)
	})

	t.Run("erreur - compagnon manquant pour un recto-verso", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		mockFileClient.On("GetUserDocument", mock.Anything, "front-solo").
			Return(&domain.DocumentRef{DocumentID: "front-solo", DocumentType: "idCardFront", OwnerID: "user-solo"}, nil)
		mockFileClient.On("GetDocumentReviewsByUserID", mock.Anything, "user-solo").
			Return([]*domain.Review{}, nil)
		mockFileClient.On("GetCurrentUserDocument", mock.Anything, "user-solo", "idCardBack").
			Return(nil, kycErrors.ErrorDocumentNotFound)

		result, err := svc.ValidateDocument(context.Background(), serviceInterfaces.ValidateDocumentInput{
			SupportAgentID: "agent-001",
			DocumentID:     "front-solo",
			Decision:       "approved",
		})
		assert.Nil(t, result)
		assert.ErrorIs(t, err, kycErrors.ErrorCompanionDocumentMissing)
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
			ReviewID:       "review-override-001",
			UserDocumentID: "doc-001",
			Status:         "completed",
			Decision:       "rejected",
			ReviewType:     "manual",
			AttemptNumber:  1,
			CreatedAt:      now,
			UpdatedAt:      now,
		}
	}

	t.Run("erreur - override d'une approbation interdit", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()

		approvedReview := newCompletedReview()
		approvedReview.Decision = "approved"
		mockFileClient.On("GetDocumentReview", mock.Anything, "review-override-001").
			Return(approvedReview, nil)

		result, err := svc.OverrideReview(context.Background(), serviceInterfaces.OverrideReviewInput{
			UserID:          "admin-001",
			ReviewID:        "review-override-001",
			Decision:        "rejected",
			ReasonRejection: "document_expired",
		})
		assert.Nil(t, result)
		assert.ErrorIs(t, err, kycErrors.ErrorOnlyRejectionOverridable)
	})

	t.Run("succès - override rejected → approved (nouvelle review chaînée)", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()

		rejectedReview := newCompletedReview()
		rejectedReview.UserID = "user-override-001"

		mockFileClient.On("GetDocumentReview", mock.Anything, "review-override-001").
			Return(rejectedReview, nil)
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

		result, err := svc.OverrideReview(context.Background(), serviceInterfaces.OverrideReviewInput{
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
		result, err := svc.OverrideReview(context.Background(), serviceInterfaces.OverrideReviewInput{
			ReviewID: "review-override-001",
			Decision: "approved",
		})
		assert.Nil(t, result)
		assert.ErrorIs(t, err, kycErrors.ErrorMissingUserID)
	})

	t.Run("erreur - review_id vide", func(t *testing.T) {
		_, _, svc := newTestService()
		result, err := svc.OverrideReview(context.Background(), serviceInterfaces.OverrideReviewInput{
			UserID:   "admin-001",
			Decision: "approved",
		})
		assert.Nil(t, result)
		assert.ErrorIs(t, err, kycErrors.ErrorMissingReviewID)
	})

	t.Run("erreur - decision invalide ou \"pending\"", func(t *testing.T) {
		_, _, svc := newTestService()
		for _, decision := range []string{"invalid_decision", "pending"} {
			result, err := svc.OverrideReview(context.Background(), serviceInterfaces.OverrideReviewInput{
				UserID:   "admin-001",
				ReviewID: "review-override-001",
				Decision: decision,
			})
			assert.Nil(t, result)
			assert.ErrorIs(t, err, kycErrors.ErrorInvalidDecision)
		}
	})

	t.Run("erreur - rejected sans reason_rejection", func(t *testing.T) {
		_, _, svc := newTestService()
		result, err := svc.OverrideReview(context.Background(), serviceInterfaces.OverrideReviewInput{
			UserID:   "admin-001",
			ReviewID: "review-override-001",
			Decision: "rejected",
		})
		assert.Nil(t, result)
		assert.ErrorIs(t, err, kycErrors.ErrorInvalidDecision)
	})

	t.Run("erreur - review introuvable", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		mockFileClient.On("GetDocumentReview", mock.Anything, "review-unknown").
			Return(nil, errors.New("not found"))

		result, err := svc.OverrideReview(context.Background(), serviceInterfaces.OverrideReviewInput{
			UserID:   "admin-001",
			ReviewID: "review-unknown",
			Decision: "approved",
		})
		assert.Nil(t, result)
		assert.ErrorIs(t, err, kycErrors.ErrorReviewNotFound)
	})

	t.Run("erreur - review non complétée non overridable", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		pendingReview := newCompletedReview()
		pendingReview.Status = "pending"
		mockFileClient.On("GetDocumentReview", mock.Anything, "review-override-001").
			Return(pendingReview, nil)

		result, err := svc.OverrideReview(context.Background(), serviceInterfaces.OverrideReviewInput{
			UserID:   "admin-001",
			ReviewID: "review-override-001",
			Decision: "approved",
		})
		assert.Nil(t, result)
		assert.ErrorIs(t, err, kycErrors.ErrorReviewNotOverridable)
	})

	t.Run("erreur - file-service indisponible lors de la création", func(t *testing.T) {
		mockFileClient, _, svc := newTestService()
		mockFileClient.On("GetDocumentReview", mock.Anything, "review-override-001").
			Return(newCompletedReview(), nil)
		mockFileClient.On("CreateDocumentReview", mock.Anything, mock.AnythingOfType("*domain.Review")).
			Return(nil, errors.New("file-service down"))

		result, err := svc.OverrideReview(context.Background(), serviceInterfaces.OverrideReviewInput{
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

func TestGetManualReviewRequests(t *testing.T) {
	t.Run("groupement par utilisateur + statuts par catégorie", func(t *testing.T) {
		fileClient, userClient, svc := newTestService()
		fileClient.On("ListKycDocuments", mock.Anything, []string(nil)).Return([]*domain.KycDocument{
			// userA : identité pending (passenger) + assurance rejected (driver/vehicle)
			{DocumentID: "d1", UserID: "userA", DocumentType: "idCardFront", Status: "pending", OwnerKind: "user"},
			{DocumentID: "d2", UserID: "userA", VehicleID: "veh1", DocumentType: "insurance", Status: "rejected", OwnerKind: "vehicle"},
			// userB : passport approved + selfie pending (passenger)
			{DocumentID: "d3", UserID: "userB", DocumentType: "passport", Status: "approved", OwnerKind: "user"},
			{DocumentID: "d5", UserID: "userB", DocumentType: "selfie", Status: "pending", OwnerKind: "user"},
			// profilePicture ignoré (aucune catégorie)
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
		assert.Equal(t, int32(2), a.TotalDocuments) // idCard (logique) + assurance

		b := res.Requests[1]
		assert.Equal(t, "userB", b.User.UserID)
		assert.Equal(t, "pending", b.PassengerStatus) // selfie pending prime sur passport approved
		assert.Equal(t, "", b.DriverStatus)
		assert.Equal(t, int32(2), b.TotalDocuments) // passport + selfie, profilePicture exclu
	})

	t.Run("permis à double rôle : contribue aux DEUX statuts, compté une fois", func(t *testing.T) {
		fileClient, userClient, svc := newTestService()
		fileClient.On("ListKycDocuments", mock.Anything, []string(nil)).Return([]*domain.KycDocument{
			{DocumentID: "dlf", UserID: "userC", DocumentType: "driverLicenceFront", Status: "pending", OwnerKind: "user"},
			{DocumentID: "dlb", UserID: "userC", DocumentType: "driverLicenceBack", Status: "pending", OwnerKind: "user"},
		}, nil)
		userClient.On("GetUserByUserID", mock.Anything, "userC").Return(&domain.UserInfo{UserID: "userC"}, nil)

		res, err := svc.GetManualReviewRequests(context.Background(), serviceInterfaces.GetManualReviewRequestsInput{})
		require.NoError(t, err)
		require.Len(t, res.Requests, 1)
		c := res.Requests[0]
		assert.Equal(t, "pending", c.PassengerStatus, "le permis vaut pièce d'identité")
		assert.Equal(t, "pending", c.DriverStatus, "le permis prouve le droit de conduire")
		assert.Equal(t, int32(1), c.TotalDocuments, "recto + verso = UN document logique")
	})

	t.Run("recto/verso : les deux faces comptent pour un seul document", func(t *testing.T) {
		fileClient, userClient, svc := newTestService()
		fileClient.On("ListKycDocuments", mock.Anything, []string(nil)).Return([]*domain.KycDocument{
			{DocumentID: "f", UserID: "userD", DocumentType: "idCardFront", Status: "approved", OwnerKind: "user"},
			{DocumentID: "b", UserID: "userD", DocumentType: "idCardBack", Status: "approved", OwnerKind: "user"},
			{DocumentID: "s", UserID: "userD", DocumentType: "selfie", Status: "approved", OwnerKind: "user"},
		}, nil)
		userClient.On("GetUserByUserID", mock.Anything, "userD").Return(&domain.UserInfo{UserID: "userD"}, nil)

		res, err := svc.GetManualReviewRequests(context.Background(), serviceInterfaces.GetManualReviewRequestsInput{})
		require.NoError(t, err)
		require.Len(t, res.Requests, 1)
		assert.Equal(t, int32(2), res.Requests[0].TotalDocuments, "idCard (1 logique) + selfie")
	})

	t.Run("filtre statut ne garde que les users correspondants", func(t *testing.T) {
		fileClient, userClient, svc := newTestService()
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
		fileClient, _, svc := newTestService()
		fileClient.On("ListKycDocuments", mock.Anything, []string(nil)).Return(nil, errors.New("down"))

		_, err := svc.GetManualReviewRequests(context.Background(), serviceInterfaces.GetManualReviewRequestsInput{})
		assert.ErrorIs(t, err, kycErrors.ErrorFileServiceUnavailable)
	})
}

func TestGetManualReviewRequestDetail(t *testing.T) {
	t.Run("documents + dernière review rattachée, profilePicture exclu", func(t *testing.T) {
		fileClient, userClient, svc := newTestService()
		fileClient.ExpectedCalls = nil
		userClient.On("GetUserByUserID", mock.Anything, "userA").Return(&domain.UserInfo{UserID: "userA", Name: "A"}, nil)
		fileClient.On("GetUserDocumentSummaries", mock.Anything, "userA").Return([]*domain.DocumentSummary{
			{DocumentID: "d1", DocumentType: "idCardFront", Status: "approved", OwnerKind: "user", OwnerID: "userA", Category: "passenger", IsCurrent: true},
			{DocumentID: "pp", DocumentType: "profilePicture", Status: "approved", OwnerKind: "user", OwnerID: "userA", IsCurrent: true},
		}, nil)
		fileClient.On("GetVehicleDocumentSummariesByUserID", mock.Anything, "userA").Return([]*domain.DocumentSummary{
			{DocumentID: "v1", DocumentType: "insurance", Status: "rejected", OwnerKind: "vehicle", OwnerID: "veh1", Category: "driver", IsCurrent: true},
		}, nil)
		older := time.Now().Add(-2 * time.Hour)
		newer := time.Now().Add(-1 * time.Hour)
		fileClient.On("GetDocumentReviewsByUserID", mock.Anything, "userA").Return([]*domain.Review{
			{ReviewID: "r-old", UserDocumentID: "d1", DocumentType: "idCardFront", Decision: "pending", ReviewedAt: &older},
			{ReviewID: "r-new", UserDocumentID: "d1", DocumentType: "idCardFront", Decision: "approved", ReviewedBy: "agent1", Notes: "ok", AttemptNumber: 2, ReviewedAt: &newer},
			{ReviewID: "r-veh", VehicleDocumentID: "v1", DocumentType: "insurance", Decision: "rejected", ReasonRejection: "document_illegible", ReviewedAt: &newer},
		}, nil)

		detail, err := svc.GetManualReviewRequestDetail(context.Background(), "userA")
		require.NoError(t, err)
		assert.Equal(t, "userA", detail.User.UserID)
		require.Len(t, detail.Documents, 2, "profilePicture exclu de la vue support")

		// idCardFront : dernière review = r-new (la plus récente), notes visibles
		idDoc := detail.Documents[0]
		assert.Equal(t, "d1", idDoc.DocumentID)
		require.NotNil(t, idDoc.LatestReview)
		assert.Equal(t, "r-new", idDoc.LatestReview.ReviewID)
		assert.Equal(t, "approved", idDoc.LatestReview.Decision)
		assert.Equal(t, "agent1", idDoc.LatestReview.ReviewedBy)
		assert.Equal(t, "ok", idDoc.LatestReview.Notes)
		assert.Equal(t, int32(2), idDoc.LatestReview.AttemptNumber)

		// insurance : review véhicule rattachée par VehicleDocumentID
		vehDoc := detail.Documents[1]
		assert.Equal(t, "v1", vehDoc.DocumentID)
		assert.Equal(t, "vehicle", vehDoc.OwnerKind)
		require.NotNil(t, vehDoc.LatestReview)
		assert.Equal(t, "r-veh", vehDoc.LatestReview.ReviewID)
		assert.Equal(t, "document_illegible", vehDoc.LatestReview.ReasonRejection)
	})

	t.Run("document sans review → LatestReview nil", func(t *testing.T) {
		fileClient, userClient, svc := newTestService()
		fileClient.ExpectedCalls = nil
		userClient.On("GetUserByUserID", mock.Anything, "userA").Return(&domain.UserInfo{UserID: "userA"}, nil)
		fileClient.On("GetUserDocumentSummaries", mock.Anything, "userA").Return([]*domain.DocumentSummary{
			{DocumentID: "d1", DocumentType: "selfie", Status: "pending", OwnerKind: "user", OwnerID: "userA", Category: "passenger", IsCurrent: true},
		}, nil)
		fileClient.On("GetVehicleDocumentSummariesByUserID", mock.Anything, "userA").Return([]*domain.DocumentSummary{}, nil)
		fileClient.On("GetDocumentReviewsByUserID", mock.Anything, "userA").Return([]*domain.Review{}, nil)

		detail, err := svc.GetManualReviewRequestDetail(context.Background(), "userA")
		require.NoError(t, err)
		require.Len(t, detail.Documents, 1)
		assert.Equal(t, "selfie", detail.Documents[0].DocumentType)
		assert.Nil(t, detail.Documents[0].LatestReview)
	})

	t.Run("userID vide → ErrorMissingUserID", func(t *testing.T) {
		_, _, svc := newTestService()
		_, err := svc.GetManualReviewRequestDetail(context.Background(), "")
		assert.ErrorIs(t, err, kycErrors.ErrorMissingUserID)
	})
}
