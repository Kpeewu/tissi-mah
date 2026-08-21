package e2e

import (
	"context"
	"testing"

	"github.com/Kpeewu/tissi-mah/services/file-service/internal/client"
	filepb "github.com/Kpeewu/tissi-mah/services/file-service/proto/gen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
)

// withFirebaseUID injecte x-firebase-uid dans le contexte sortant, comme le ferait
// l'api-gateway après validation du JWT, et enregistre la résolution de profil
// correspondante sur mockUserClient (UploadIdDocument/UploadVehicleDocuments résolvent
// l'UserID interne à partir de cet UID — le champ UserID du body est ignoré). Utilise
// le Firebase UID directement comme UserID interne pour simplifier les assertions.
func withFirebaseUID(ctx context.Context, firebaseUID string) context.Context {
	mockUserClient.On("GetUserProfileByFirebaseID", mock.Anything, firebaseUID).
		Return(&client.UserProfile{UserID: firebaseUID, FirstName: "Jean", LastName: "Dupont"}, nil).Maybe()
	return metadata.NewOutgoingContext(ctx, metadata.Pairs("x-firebase-uid", firebaseUID))
}

// =============================================================================
// UploadIdDocument — création de la review "pending" à l'upload
// =============================================================================

func TestE2E_UploadIdDocument_CreatesPendingReview(t *testing.T) {
	t.Run("IDCard (recto+verso) crée une seule review pending couvrant les deux faces", func(t *testing.T) {
		cleanTables(t)
		ctx := withFirebaseUID(context.Background(), "e2e-pending-idcard")

		resp, err := grpcClient.UploadIdDocument(ctx, &filepb.UploadIdDocumentRequest{
			DocumentType:   "IDCard",
			IDCardRecto:    fakeJPEG(),
			IDCardVerso:    fakeJPEG(),
			DocumentNumber: "ID-PENDING-001",
			IssuedAt:       "2022-01-01T00:00:00Z",
			ExpireAt:       "2027-01-01T00:00:00Z",
			IssuingCountry: "TG",
		})
		require.NoError(t, err)
		require.True(t, resp.Success, resp.ErrorMessage)
		require.Len(t, resp.Documents, 2)

		reviewsResp, err := grpcClient.GetDocumentReviewsByUserID(context.Background(), &filepb.GetDocumentReviewsByUserIDRequest{
			UserId: "e2e-pending-idcard",
		})
		require.NoError(t, err)
		require.Len(t, reviewsResp.Reviews, 1, "une seule review doit couvrir les deux faces idCard")

		review := reviewsResp.Reviews[0]
		assert.Equal(t, "pending", review.Status)
		assert.Equal(t, "pending", review.Decision)
		assert.Equal(t, "idCardFront", review.DocumentType)
		assert.Equal(t, "idCard", review.LogicalDocumentType)
		assert.Equal(t, resp.Documents[0].DocumentID, review.UserDocumentId)
		assert.Equal(t, resp.Documents[1].DocumentID, review.SecondUserDocumentId)
		assert.Empty(t, review.ReviewedAt, "une review pending n'a pas de date de décision")
		assert.NotEmpty(t, review.CreatedAt)
	})

	t.Run("Passport (face unique) crée une review pending sans SecondUserDocumentId", func(t *testing.T) {
		cleanTables(t)
		ctx := withFirebaseUID(context.Background(), "e2e-pending-passport")

		resp, err := grpcClient.UploadIdDocument(ctx, &filepb.UploadIdDocumentRequest{
			DocumentType:   "Passport",
			Passport:       fakeJPEG(),
			DocumentNumber: "PP-PENDING-001",
			IssuedAt:       "2022-01-01T00:00:00Z",
			ExpireAt:       "2032-01-01T00:00:00Z",
			IssuingCountry: "TG",
		})
		require.NoError(t, err)
		require.True(t, resp.Success, resp.ErrorMessage)
		require.Len(t, resp.Documents, 1)

		reviewsResp, err := grpcClient.GetDocumentReviewsByUserID(context.Background(), &filepb.GetDocumentReviewsByUserIDRequest{
			UserId: "e2e-pending-passport",
		})
		require.NoError(t, err)
		require.Len(t, reviewsResp.Reviews, 1)

		review := reviewsResp.Reviews[0]
		assert.Equal(t, "pending", review.Status)
		assert.Equal(t, "passport", review.DocumentType)
		assert.Equal(t, resp.Documents[0].DocumentID, review.UserDocumentId)
		assert.Empty(t, review.SecondUserDocumentId)
	})
}

// =============================================================================
// UploadVehicleDocuments — création des reviews "pending" (permis + assurance + carte grise)
// =============================================================================

func TestE2E_UploadVehicleDocuments_CreatesPendingReviews(t *testing.T) {
	t.Run("permis + assurance + carte grise créent chacun leur review pending", func(t *testing.T) {
		cleanTables(t)
		ctx := withFirebaseUID(context.Background(), "e2e-pending-vehicle-user")

		resp, err := grpcClient.UploadVehicleDocuments(ctx, &filepb.UploadVehicleDocumentsRequest{
			VehicleID:           "e2e-pending-vehicle-1",
			DriverLicenceRecto:  fakeJPEG(),
			DriverLicenceVerso:  fakeJPEG(),
			Assurance:           fakeJPEG(),
			VehicleRegistration: fakeJPEG(),
			DriverLicenceMetadata: &filepb.VehicleDocMetadata{
				DocumentNumber:   "DL-PENDING-001",
				IssuedAt:         "2022-01-01T00:00:00Z",
				ExpireAt:         "2027-01-01T00:00:00Z",
				IssuingAuthority: "DVLA",
			},
			AssuranceMetadata: &filepb.VehicleDocMetadata{
				DocumentNumber:   "INS-PENDING-001",
				IssuedAt:         "2026-01-01T00:00:00Z",
				ExpireAt:         "2027-01-01T00:00:00Z",
				IssuingAuthority: "AXA",
			},
			RegistrationCardMetadata: &filepb.VehicleDocMetadata{
				DocumentNumber:   "REG-PENDING-001",
				IssuedAt:         "2020-01-01T00:00:00Z",
				IssuingAuthority: "DVLA",
			},
		})
		require.NoError(t, err)
		require.True(t, resp.Success, resp.ErrorMessage)
		require.Len(t, resp.Documents, 4) // permis recto + verso + assurance + carte grise

		reviewsResp, err := grpcClient.GetDocumentReviewsByUserID(context.Background(), &filepb.GetDocumentReviewsByUserIDRequest{
			UserId: "e2e-pending-vehicle-user",
		})
		require.NoError(t, err)
		require.Len(t, reviewsResp.Reviews, 3, "une review pending par document logique : permis, assurance, carte grise")

		byType := make(map[string]*filepb.DocumentReviewResponse)
		for _, r := range reviewsResp.Reviews {
			byType[r.DocumentType] = r
			assert.Equal(t, "pending", r.Status)
			assert.Equal(t, "pending", r.Decision)
		}

		licenceReview, ok := byType["driverLicenceFront"]
		require.True(t, ok, "review pending attendue pour le permis")
		assert.NotEmpty(t, licenceReview.UserDocumentId)
		assert.NotEmpty(t, licenceReview.SecondUserDocumentId)
		assert.Empty(t, licenceReview.VehicleDocumentId)

		insuranceReview, ok := byType["insurance"]
		require.True(t, ok, "review pending attendue pour l'assurance")
		assert.NotEmpty(t, insuranceReview.VehicleDocumentId)
		assert.Empty(t, insuranceReview.UserDocumentId)

		registrationReview, ok := byType["registrationCard"]
		require.True(t, ok, "review pending attendue pour la carte grise")
		assert.NotEmpty(t, registrationReview.VehicleDocumentId)
	})

	t.Run("permis déjà soumis : aucune nouvelle review pending pour le permis, seulement assurance+carte grise", func(t *testing.T) {
		cleanTables(t)
		ctx := withFirebaseUID(context.Background(), "e2e-pending-vehicle-user-2")

		// Premier véhicule : soumet le permis (crée sa review pending).
		_, err := grpcClient.UploadVehicleDocuments(ctx, &filepb.UploadVehicleDocumentsRequest{
			VehicleID:           "e2e-pending-vehicle-2a",
			DriverLicenceRecto:  fakeJPEG(),
			DriverLicenceVerso:  fakeJPEG(),
			Assurance:           fakeJPEG(),
			VehicleRegistration: fakeJPEG(),
			DriverLicenceMetadata: &filepb.VehicleDocMetadata{
				DocumentNumber: "DL-PENDING-002", IssuedAt: "2022-01-01T00:00:00Z",
				ExpireAt: "2027-01-01T00:00:00Z", IssuingAuthority: "DVLA",
			},
			AssuranceMetadata: &filepb.VehicleDocMetadata{
				DocumentNumber: "INS-PENDING-002", IssuedAt: "2026-01-01T00:00:00Z",
				ExpireAt: "2027-01-01T00:00:00Z", IssuingAuthority: "AXA",
			},
			RegistrationCardMetadata: &filepb.VehicleDocMetadata{
				DocumentNumber: "REG-PENDING-002", IssuedAt: "2020-01-01T00:00:00Z", IssuingAuthority: "DVLA",
			},
		})
		require.NoError(t, err)

		// Deuxième véhicule, même utilisateur : permis déjà courant → images ignorées.
		resp2, err := grpcClient.UploadVehicleDocuments(ctx, &filepb.UploadVehicleDocumentsRequest{
			VehicleID:           "e2e-pending-vehicle-2b",
			Assurance:           fakeJPEG(),
			VehicleRegistration: fakeJPEG(),
			AssuranceMetadata: &filepb.VehicleDocMetadata{
				DocumentNumber: "INS-PENDING-003", IssuedAt: "2026-01-01T00:00:00Z",
				ExpireAt: "2027-01-01T00:00:00Z", IssuingAuthority: "AXA",
			},
			RegistrationCardMetadata: &filepb.VehicleDocMetadata{
				DocumentNumber: "REG-PENDING-003", IssuedAt: "2020-01-01T00:00:00Z", IssuingAuthority: "DVLA",
			},
		})
		require.NoError(t, err)
		require.True(t, resp2.Success, resp2.ErrorMessage)
		require.Len(t, resp2.Documents, 2) // seulement assurance + carte grise pour ce véhicule

		reviewsResp, err := grpcClient.GetDocumentReviewsByUserID(context.Background(), &filepb.GetDocumentReviewsByUserIDRequest{
			UserId: "e2e-pending-vehicle-user-2",
		})
		require.NoError(t, err)
		// 1 review permis (1er véhicule) + 2 reviews assurance/carte grise (1er véhicule) + 2 reviews assurance/carte grise (2e véhicule) = 5
		require.Len(t, reviewsResp.Reviews, 5)

		licenceCount := 0
		for _, r := range reviewsResp.Reviews {
			if r.DocumentType == "driverLicenceFront" {
				licenceCount++
			}
		}
		assert.Equal(t, 1, licenceCount, "le permis ne doit générer qu'une seule review pending, pas une par véhicule")
	})
}

// =============================================================================
// ChangeDocument — resoumission après rejet : (re)création de la review pending
// =============================================================================

func TestE2E_ChangeDocument_PendingReviewOnResubmission(t *testing.T) {
	ctx := context.Background()

	t.Run("resoumission d'un document à face unique crée une nouvelle review pending", func(t *testing.T) {
		cleanTables(t)

		uploadResp, err := grpcClient.UploadIdDocument(withFirebaseUID(ctx, "e2e-resubmit-passport"), &filepb.UploadIdDocumentRequest{
			DocumentType:   "Passport",
			Passport:       fakeJPEG(),
			DocumentNumber: "PP-RESUB-001",
			IssuedAt:       "2022-01-01T00:00:00Z",
			ExpireAt:       "2032-01-01T00:00:00Z",
			IssuingCountry: "TG",
		})
		require.NoError(t, err)
		passportID := uploadResp.Documents[0].DocumentID

		// La review "pending" créée à l'upload doit être résolue via Update (comme le
		// fait ValidateDocument côté kyc-service), pas via une nouvelle Create : c'est
		// elle que l'agent support fait passer à completed/rejected.
		pendingReviewsResp, err := grpcClient.GetDocumentReviewsByUserID(ctx, &filepb.GetDocumentReviewsByUserIDRequest{
			UserId: "e2e-resubmit-passport",
		})
		require.NoError(t, err)
		require.Len(t, pendingReviewsResp.Reviews, 1)
		originalReviewID := pendingReviewsResp.Reviews[0].ReviewId

		_, err = grpcClient.UpdateDocumentReview(ctx, &filepb.UpdateDocumentReviewRequest{
			ReviewId:        originalReviewID,
			Status:          "completed",
			Decision:        "rejected",
			ReasonRejection: "document_illegible",
			ReviewType:      "manual",
			ReviewedBy:      "agent-e2e",
		})
		require.NoError(t, err)
		_, err = testPool.Exec(ctx, `UPDATE user_documents SET status = 'rejected' WHERE document_id = $1`, passportID)
		require.NoError(t, err)

		// Resoumission via ChangeDocument.
		changeResp, err := grpcClient.ChangeDocument(ctx, &filepb.ChangeDocumentRequest{
			UserID:      "e2e-resubmit-passport",
			FileID:      passportID,
			NewDocument: fakeJPEG(),
		})
		require.NoError(t, err)
		require.True(t, changeResp.Success, changeResp.ErrorMessage)
		newPassportID := changeResp.Document.DocumentID

		reviewsResp, err := grpcClient.GetDocumentReviewsByUserID(ctx, &filepb.GetDocumentReviewsByUserIDRequest{
			UserId: "e2e-resubmit-passport",
		})
		require.NoError(t, err)
		require.Len(t, reviewsResp.Reviews, 2, "la review rejetée d'origine + une nouvelle review pending")

		var pendingCount int
		for _, r := range reviewsResp.Reviews {
			if r.Status == "pending" {
				pendingCount++
				assert.Equal(t, newPassportID, r.UserDocumentId)
			}
		}
		assert.Equal(t, 1, pendingCount)
	})

	t.Run("resoumission recto puis verso ne crée qu'une seule review pending (pas de doublon)", func(t *testing.T) {
		cleanTables(t)

		uploadResp, err := grpcClient.UploadIdDocument(withFirebaseUID(ctx, "e2e-resubmit-idcard"), &filepb.UploadIdDocumentRequest{
			DocumentType:   "IDCard",
			IDCardRecto:    fakeJPEG(),
			IDCardVerso:    fakeJPEG(),
			DocumentNumber: "ID-RESUB-001",
			IssuedAt:       "2022-01-01T00:00:00Z",
			ExpireAt:       "2027-01-01T00:00:00Z",
			IssuingCountry: "TG",
		})
		require.NoError(t, err)
		rectoID := uploadResp.Documents[0].DocumentID
		versoID := uploadResp.Documents[1].DocumentID

		pendingReviewsResp, err := grpcClient.GetDocumentReviewsByUserID(ctx, &filepb.GetDocumentReviewsByUserIDRequest{
			UserId: "e2e-resubmit-idcard",
		})
		require.NoError(t, err)
		require.Len(t, pendingReviewsResp.Reviews, 1)
		originalReviewID := pendingReviewsResp.Reviews[0].ReviewId

		_, err = grpcClient.UpdateDocumentReview(ctx, &filepb.UpdateDocumentReviewRequest{
			ReviewId:        originalReviewID,
			Status:          "completed",
			Decision:        "rejected",
			ReasonRejection: "document_expired",
			ReviewType:      "manual",
			ReviewedBy:      "agent-e2e",
		})
		require.NoError(t, err)
		_, err = testPool.Exec(ctx, `UPDATE user_documents SET status = 'rejected' WHERE document_id IN ($1, $2)`, rectoID, versoID)
		require.NoError(t, err)

		// Recto d'abord.
		changeRecto, err := grpcClient.ChangeDocument(ctx, &filepb.ChangeDocumentRequest{
			UserID: "e2e-resubmit-idcard", FileID: rectoID, NewDocument: fakeJPEG(),
		})
		require.NoError(t, err)
		require.True(t, changeRecto.Success, changeRecto.ErrorMessage)
		newRectoID := changeRecto.Document.DocumentID

		// Puis verso.
		changeVerso, err := grpcClient.ChangeDocument(ctx, &filepb.ChangeDocumentRequest{
			UserID: "e2e-resubmit-idcard", FileID: versoID, NewDocument: fakeJPEG(),
		})
		require.NoError(t, err)
		require.True(t, changeVerso.Success, changeVerso.ErrorMessage)
		newVersoID := changeVerso.Document.DocumentID

		reviewsResp, err := grpcClient.GetDocumentReviewsByUserID(ctx, &filepb.GetDocumentReviewsByUserIDRequest{
			UserId: "e2e-resubmit-idcard",
		})
		require.NoError(t, err)

		var pendingReview *filepb.DocumentReviewResponse
		var pendingCount int
		for _, r := range reviewsResp.Reviews {
			if r.Status == "pending" {
				pendingCount++
				pendingReview = r
			}
		}
		require.Equal(t, 1, pendingCount, "une seule review pending doit couvrir idCard malgré 2 appels ChangeDocument successifs")
		// Le recto étant remplacé en premier, une review est créée référençant le nouveau
		// recto + l'ancien verso (encore courant à ce moment) ; le remplacement du verso
		// juste après ne duplique pas la review (elle existe déjà, non terminale).
		assert.Equal(t, newRectoID, pendingReview.UserDocumentId)
		assert.NotEqual(t, newVersoID, pendingReview.SecondUserDocumentId,
			"la review pending a été créée avant le remplacement du verso, elle référence donc encore l'ancien verso")
	})
}
