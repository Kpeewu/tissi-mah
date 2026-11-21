package service_test

import (
	"context"
	"testing"

	"github.com/Kpeewu/tissi-mah/services/file-service/internal/domain"
	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/file-service/internal/service/interfaces"
	fileErrors "github.com/Kpeewu/tissi-mah/services/file-service/pkg/errors"
	"github.com/Kpeewu/tissi-mah/services/file-service/tests/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// UpdateDocumentReview — synchronisation du statut des documents
//
// Régression du bug critique : la validation support passe par Update (la review
// "pending" est pré-créée à l'upload). Sans synchronisation ici, les documents
// restaient "pending" pour toujours — l'utilisateur ne sortait jamais de la file
// support et un document rejeté ne pouvait plus être resoumis.
// =============================================================================

func TestFileService_UpdateDocumentReview_SyncsDocumentStatus(t *testing.T) {
	newPendingReview := func(docID, secondDocID, docType, logicalType string) *domain.DocumentReview {
		r := &domain.DocumentReview{
			ReviewID:            "review-1",
			UserID:              "user-1",
			DocumentType:        docType,
			LogicalDocumentType: logicalType,
			Status:              "pending",
			Decision:            "pending",
			ReviewType:          "manual",
		}
		if docID != "" {
			r.UserDocumentID = &docID
		}
		if secondDocID != "" {
			r.SecondUserDocumentID = &secondDocID
		}
		return r
	}

	t.Run("approbation d'un recto-verso : les DEUX faces passent à approved", func(t *testing.T) {
		userDocRead := &mocks.MockUserDocumentRepositoryRead{}
		userDocWrite := &mocks.MockUserDocumentRepositoryWrite{}
		reviewRead := &mocks.MockDocumentReviewRepositoryRead{}
		reviewWrite := &mocks.MockDocumentReviewRepositoryWrite{}

		front := stubUserDoc("front-1", "user-1", "idCardFront")
		back := stubUserDoc("back-1", "user-1", "idCardBack")
		userDocRead.On("GetByID", mock.Anything, "front-1").Return(front, nil)
		userDocRead.On("GetByID", mock.Anything, "back-1").Return(back, nil)
		// Re-résolution des faces courantes au moment de la décision.
		userDocRead.On("GetCurrentByUserIDAndType", mock.Anything, "user-1", "idCardFront").Return(front, nil)
		userDocRead.On("GetCurrentByUserIDAndType", mock.Anything, "user-1", "idCardBack").Return(back, nil)

		updated := map[string]string{}
		userDocWrite.On("Update", mock.Anything, mock.AnythingOfType("*domain.UserDocument")).
			Run(func(args mock.Arguments) {
				d := args.Get(1).(*domain.UserDocument)
				updated[d.DocumentID] = d.Status
			}).
			Return(front, nil)

		review := newPendingReview("front-1", "back-1", "idCardFront", "idCard")
		review.Status = "completed"
		review.Decision = "approved"

		reviewWrite.On("Update", mock.Anything, mock.AnythingOfType("*domain.DocumentReview")).Return(nil)
		reviewRead.On("GetByID", mock.Anything, "review-1").Return(review, nil)

		svc := newService(userDocRead, userDocWrite,
			&mocks.MockVehicleDocumentRepositoryRead{}, &mocks.MockVehicleDocumentRepositoryWrite{},
			reviewRead, reviewWrite, &mocks.MockStorageClient{})

		_, err := svc.UpdateDocumentReview(context.Background(), review)

		require.NoError(t, err)
		assert.Equal(t, "approved", updated["front-1"], "le recto doit passer à approved")
		assert.Equal(t, "approved", updated["back-1"], "le verso doit passer à approved (même décision logique)")
	})

	t.Run("rejet : le document passe à rejected (resoumission redevient possible)", func(t *testing.T) {
		userDocRead := &mocks.MockUserDocumentRepositoryRead{}
		userDocWrite := &mocks.MockUserDocumentRepositoryWrite{}
		reviewRead := &mocks.MockDocumentReviewRepositoryRead{}
		reviewWrite := &mocks.MockDocumentReviewRepositoryWrite{}

		doc := stubUserDoc("doc-1", "user-1", "passport")
		userDocRead.On("GetByID", mock.Anything, "doc-1").Return(doc, nil)
		userDocRead.On("GetCurrentByUserIDAndType", mock.Anything, "user-1", "passport").Return(doc, nil)

		var updatedStatus string
		userDocWrite.On("Update", mock.Anything, mock.AnythingOfType("*domain.UserDocument")).
			Run(func(args mock.Arguments) {
				updatedStatus = args.Get(1).(*domain.UserDocument).Status
			}).
			Return(doc, nil)

		review := newPendingReview("doc-1", "", "passport", "passport")
		review.Status = "completed"
		review.Decision = "rejected"
		review.ReasonRejection = "document_illegible"

		reviewWrite.On("Update", mock.Anything, mock.AnythingOfType("*domain.DocumentReview")).Return(nil)
		reviewRead.On("GetByID", mock.Anything, "review-1").Return(review, nil)

		svc := newService(userDocRead, userDocWrite,
			&mocks.MockVehicleDocumentRepositoryRead{}, &mocks.MockVehicleDocumentRepositoryWrite{},
			reviewRead, reviewWrite, &mocks.MockStorageClient{})

		_, err := svc.UpdateDocumentReview(context.Background(), review)

		require.NoError(t, err)
		assert.Equal(t, "rejected", updatedStatus)
	})

	t.Run("document véhicule approuvé : le statut suit", func(t *testing.T) {
		vehicleDocRead := &mocks.MockVehicleDocumentRepositoryRead{}
		vehicleDocWrite := &mocks.MockVehicleDocumentRepositoryWrite{}
		reviewRead := &mocks.MockDocumentReviewRepositoryRead{}
		reviewWrite := &mocks.MockDocumentReviewRepositoryWrite{}

		vdoc := stubVehicleDoc("vdoc-1", "veh-1", "insurance")
		vehicleDocRead.On("GetByID", mock.Anything, "vdoc-1").Return(vdoc, nil)

		var updatedStatus string
		vehicleDocWrite.On("Update", mock.Anything, mock.AnythingOfType("*domain.VehicleDocument")).
			Run(func(args mock.Arguments) {
				updatedStatus = args.Get(1).(*domain.VehicleDocument).Status
			}).
			Return(vdoc, nil)

		vehicleDocID := "vdoc-1"
		review := &domain.DocumentReview{
			ReviewID:            "review-1",
			UserID:              "user-1",
			DocumentType:        "insurance",
			LogicalDocumentType: "insurance",
			VehicleDocumentID:   &vehicleDocID,
			Status:              "completed",
			Decision:            "approved",
			ReviewType:          "manual",
		}

		reviewWrite.On("Update", mock.Anything, mock.AnythingOfType("*domain.DocumentReview")).Return(nil)
		reviewRead.On("GetByID", mock.Anything, "review-1").Return(review, nil)

		svc := newService(&mocks.MockUserDocumentRepositoryRead{}, &mocks.MockUserDocumentRepositoryWrite{},
			vehicleDocRead, vehicleDocWrite, reviewRead, reviewWrite, &mocks.MockStorageClient{})

		_, err := svc.UpdateDocumentReview(context.Background(), review)

		require.NoError(t, err)
		assert.Equal(t, "approved", updatedStatus)
	})

	t.Run("review encore pending : aucun statut de document touché", func(t *testing.T) {
		userDocRead := &mocks.MockUserDocumentRepositoryRead{}
		userDocWrite := &mocks.MockUserDocumentRepositoryWrite{}
		reviewRead := &mocks.MockDocumentReviewRepositoryRead{}
		reviewWrite := &mocks.MockDocumentReviewRepositoryWrite{}

		review := newPendingReview("doc-1", "", "passport", "passport")
		reviewWrite.On("Update", mock.Anything, mock.AnythingOfType("*domain.DocumentReview")).Return(nil)
		reviewRead.On("GetByID", mock.Anything, "review-1").Return(review, nil)

		svc := newService(userDocRead, userDocWrite,
			&mocks.MockVehicleDocumentRepositoryRead{}, &mocks.MockVehicleDocumentRepositoryWrite{},
			reviewRead, reviewWrite, &mocks.MockStorageClient{})

		_, err := svc.UpdateDocumentReview(context.Background(), review)

		require.NoError(t, err)
		userDocWrite.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
	})

	t.Run("échec de synchronisation : l'erreur est propagée (décision non annoncée comme réussie)", func(t *testing.T) {
		userDocRead := &mocks.MockUserDocumentRepositoryRead{}
		userDocWrite := &mocks.MockUserDocumentRepositoryWrite{}
		reviewRead := &mocks.MockDocumentReviewRepositoryRead{}
		reviewWrite := &mocks.MockDocumentReviewRepositoryWrite{}

		doc := stubUserDoc("doc-1", "user-1", "passport")
		userDocRead.On("GetByID", mock.Anything, "doc-1").Return(doc, nil)
		userDocRead.On("GetCurrentByUserIDAndType", mock.Anything, "user-1", "passport").Return(doc, nil)
		userDocWrite.On("Update", mock.Anything, mock.AnythingOfType("*domain.UserDocument")).
			Return(nil, fileErrors.ErrorInternalServer)

		review := newPendingReview("doc-1", "", "passport", "passport")
		review.Status = "completed"
		review.Decision = "approved"

		reviewWrite.On("Update", mock.Anything, mock.AnythingOfType("*domain.DocumentReview")).Return(nil)

		svc := newService(userDocRead, userDocWrite,
			&mocks.MockVehicleDocumentRepositoryRead{}, &mocks.MockVehicleDocumentRepositoryWrite{},
			reviewRead, reviewWrite, &mocks.MockStorageClient{})

		_, err := svc.UpdateDocumentReview(context.Background(), review)

		assert.ErrorIs(t, err, fileErrors.ErrorStatusSyncFailed)
	})
}

// =============================================================================
// UploadSelfie
// =============================================================================

func TestFileService_UploadSelfie(t *testing.T) {
	t.Run("succès : selfie créé, ancien remplacé, review pending ouverte", func(t *testing.T) {
		userDocRead := &mocks.MockUserDocumentRepositoryRead{}
		userDocWrite := &mocks.MockUserDocumentRepositoryWrite{}
		reviewRead := &mocks.MockDocumentReviewRepositoryRead{}
		reviewWrite := &mocks.MockDocumentReviewRepositoryWrite{}
		storage := &mocks.MockStorageClient{}

		previous := stubUserDoc("selfie-old", "user-1", "selfie")
		userDocRead.On("GetCurrentByUserIDAndType", mock.Anything, "user-1", "selfie").Return(previous, nil)
		storage.On("Upload", mock.Anything, mock.AnythingOfType("string"), mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("int64")).
			Return("https://storage.example.com/selfie.jpg", nil)
		storage.On("GeneratePresignedURL", mock.Anything, mock.Anything, mock.Anything).
			Return("https://presigned.example.com/selfie", nil)

		var created *domain.UserDocument
		userDocWrite.On("Create", mock.Anything, mock.AnythingOfType("*domain.UserDocument")).
			Run(func(args mock.Arguments) { created = args.Get(1).(*domain.UserDocument) }).
			Return("new-selfie", nil)
		userDocWrite.On("MarkAsReplaced", mock.Anything, "selfie-old", mock.AnythingOfType("string")).Return(nil)

		// Aucune review selfie ouverte → une nouvelle review pending est créée.
		reviewRead.On("GetHistoryByUserIDAndLogicalType", mock.Anything, "user-1", "selfie").
			Return([]*domain.DocumentReview{}, nil)
		var createdReview *domain.DocumentReview
		reviewWrite.On("Create", mock.Anything, mock.AnythingOfType("*domain.DocumentReview")).
			Run(func(args mock.Arguments) { createdReview = args.Get(1).(*domain.DocumentReview) }).
			Return("review-selfie", nil)

		svc := newService(userDocRead, userDocWrite,
			&mocks.MockVehicleDocumentRepositoryRead{}, &mocks.MockVehicleDocumentRepositoryWrite{},
			reviewRead, reviewWrite, storage)

		result, err := svc.UploadSelfie(context.Background(), serviceInterfaces.UploadSelfieInput{
			UserID:    "user-1",
			FirstName: "Amadou",
			LastName:  "Diallo",
			Selfie:    fakeJPEG(),
		})

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "selfie", result.DocumentType)
		require.NotNil(t, created)
		assert.Equal(t, "selfie", created.DocumentType)
		assert.Equal(t, "pending", created.Status, "le selfie entre en validation support")
		assert.True(t, created.IsCurrent)
		assert.Empty(t, created.DocumentNumber, "aucune métadonnée légale sur un selfie")
		userDocWrite.AssertCalled(t, "MarkAsReplaced", mock.Anything, "selfie-old", mock.AnythingOfType("string"))
		require.NotNil(t, createdReview)
		assert.Equal(t, "selfie", createdReview.LogicalDocumentType)
		assert.Equal(t, "pending", createdReview.Decision)
	})

	t.Run("remplacement pendant une review ouverte : pas de review en double", func(t *testing.T) {
		userDocRead := &mocks.MockUserDocumentRepositoryRead{}
		userDocWrite := &mocks.MockUserDocumentRepositoryWrite{}
		reviewRead := &mocks.MockDocumentReviewRepositoryRead{}
		reviewWrite := &mocks.MockDocumentReviewRepositoryWrite{}
		storage := &mocks.MockStorageClient{}

		userDocRead.On("GetCurrentByUserIDAndType", mock.Anything, "user-1", "selfie").
			Return(nil, fileErrors.ErrorDocumentNotFound)
		storage.On("Upload", mock.Anything, mock.AnythingOfType("string"), mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("int64")).
			Return("https://storage.example.com/selfie.jpg", nil)
		storage.On("GeneratePresignedURL", mock.Anything, mock.Anything, mock.Anything).
			Return("https://presigned.example.com/selfie", nil)
		userDocWrite.On("Create", mock.Anything, mock.AnythingOfType("*domain.UserDocument")).Return("new-selfie", nil)

		// Une review selfie non terminée existe déjà.
		reviewRead.On("GetHistoryByUserIDAndLogicalType", mock.Anything, "user-1", "selfie").
			Return([]*domain.DocumentReview{
				{ReviewID: "review-open", Status: "pending", Decision: "pending", LogicalDocumentType: "selfie"},
			}, nil)

		svc := newService(userDocRead, userDocWrite,
			&mocks.MockVehicleDocumentRepositoryRead{}, &mocks.MockVehicleDocumentRepositoryWrite{},
			reviewRead, reviewWrite, storage)

		_, err := svc.UploadSelfie(context.Background(), serviceInterfaces.UploadSelfieInput{
			UserID: "user-1",
			Selfie: fakeJPEG(),
		})

		require.NoError(t, err)
		reviewWrite.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	})

	t.Run("erreur : image absente ou type non supporté", func(t *testing.T) {
		svc := newService(&mocks.MockUserDocumentRepositoryRead{}, &mocks.MockUserDocumentRepositoryWrite{},
			&mocks.MockVehicleDocumentRepositoryRead{}, &mocks.MockVehicleDocumentRepositoryWrite{},
			&mocks.MockDocumentReviewRepositoryRead{}, &mocks.MockDocumentReviewRepositoryWrite{},
			&mocks.MockStorageClient{})

		_, err := svc.UploadSelfie(context.Background(), serviceInterfaces.UploadSelfieInput{UserID: "user-1"})
		assert.ErrorIs(t, err, fileErrors.ErrorInvalidInput)

		_, err = svc.UploadSelfie(context.Background(), serviceInterfaces.UploadSelfieInput{
			UserID: "user-1",
			Selfie: []byte("%PDF-1.4 fake pdf content"),
		})
		assert.ErrorIs(t, err, fileErrors.ErrorInvalidMimeType, "un PDF n'est pas un selfie valide")
	})
}
