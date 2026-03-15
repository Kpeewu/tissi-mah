package e2e

import (
	"context"
	"testing"
	"time"

	filepb "github.com/Kpeewu/tissi-mah/services/file-service/proto/gen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// =============================================================================
// Health
// =============================================================================

func TestE2E_Health(t *testing.T) {
	ctx := context.Background()

	resp, err := grpcClient.Health(ctx, &filepb.HealthRequest{})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "healthy", resp.Status)
	assert.NotEmpty(t, resp.Version)
	assert.Greater(t, resp.Timestamp, int64(0))
}

// =============================================================================
// UploadUserDocument (streaming)
// =============================================================================

func TestE2E_UploadUserDocument(t *testing.T) {
	ctx := context.Background()

	t.Run("succès - upload et vérification des champs", func(t *testing.T) {
		cleanTables(t)

		resp := uploadUserDoc(t, "e2e-user-1", "idCardFront")

		assert.NotEmpty(t, resp.DocumentId)
		assert.Equal(t, "e2e-user-1", resp.UserId)
		assert.Equal(t, "idCardFront", resp.DocumentType)
		assert.Equal(t, "image/jpeg", resp.MimeType)
		assert.Equal(t, "pending", resp.Status)
		assert.True(t, resp.IsCurrent)
		assert.Equal(t, "https://storage.example.com/file.jpg", resp.DocumentUrl)
		_, err := time.Parse(time.RFC3339, resp.UploadedAt)
		assert.NoError(t, err, "UploadedAt doit être RFC3339")
	})

	t.Run("erreur - type de document invalide", func(t *testing.T) {
		cleanTables(t)

		stream, err := grpcClient.UploadUserDocument(ctx)
		require.NoError(t, err)

		_ = stream.Send(&filepb.UploadUserDocumentRequest{
			Data: &filepb.UploadUserDocumentRequest_Metadata{
				Metadata: &filepb.UserDocumentMetadata{
					UserId:        "e2e-user-2",
					DocumentType:  "invalidType",
					MimeType:      "image/jpeg",
					FileSizeBytes: 512,
				},
			},
		})
		_ = stream.Send(&filepb.UploadUserDocumentRequest{
			Data: &filepb.UploadUserDocumentRequest_Chunk{Chunk: fakeJPEG()},
		})

		_, err = stream.CloseAndRecv()

		require.Error(t, err)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("erreur - MIME type invalide", func(t *testing.T) {
		cleanTables(t)

		stream, err := grpcClient.UploadUserDocument(ctx)
		require.NoError(t, err)

		_ = stream.Send(&filepb.UploadUserDocumentRequest{
			Data: &filepb.UploadUserDocumentRequest_Metadata{
				Metadata: &filepb.UserDocumentMetadata{
					UserId:        "e2e-user-3",
					DocumentType:  "passport",
					MimeType:      "text/plain",
					FileSizeBytes: 512,
				},
			},
		})
		_ = stream.Send(&filepb.UploadUserDocumentRequest{
			Data: &filepb.UploadUserDocumentRequest_Chunk{Chunk: fakeJPEG()},
		})

		_, err = stream.CloseAndRecv()

		require.Error(t, err)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("erreur - fichier trop grand", func(t *testing.T) {
		cleanTables(t)

		stream, err := grpcClient.UploadUserDocument(ctx)
		require.NoError(t, err)

		_ = stream.Send(&filepb.UploadUserDocumentRequest{
			Data: &filepb.UploadUserDocumentRequest_Metadata{
				Metadata: &filepb.UserDocumentMetadata{
					UserId:        "e2e-user-4",
					DocumentType:  "passport",
					MimeType:      "image/jpeg",
					FileSizeBytes: 11 * 1024 * 1024, // 11 Mo > limite 10 Mo
				},
			},
		})
		_ = stream.Send(&filepb.UploadUserDocumentRequest{
			Data: &filepb.UploadUserDocumentRequest_Chunk{Chunk: fakeJPEG()},
		})

		_, err = stream.CloseAndRecv()

		require.Error(t, err)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.ResourceExhausted, st.Code())
	})
}

// =============================================================================
// GetUserDocuments
// =============================================================================

func TestE2E_GetUserDocuments(t *testing.T) {
	ctx := context.Background()

	t.Run("succès - retourne tous les documents de l'utilisateur", func(t *testing.T) {
		cleanTables(t)

		uploadUserDoc(t, "e2e-docs-user", "idCardFront")
		uploadUserDoc(t, "e2e-docs-user", "passport")

		resp, err := grpcClient.GetUserDocuments(ctx, &filepb.GetUserDocumentsRequest{
			UserId: "e2e-docs-user",
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Len(t, resp.Documents, 2)
	})

	t.Run("succès - liste vide quand aucun document", func(t *testing.T) {
		cleanTables(t)

		resp, err := grpcClient.GetUserDocuments(ctx, &filepb.GetUserDocumentsRequest{
			UserId: "user-no-docs",
		})

		require.NoError(t, err)
		assert.Empty(t, resp.Documents)
	})

	t.Run("isolation - ne retourne pas les documents des autres utilisateurs", func(t *testing.T) {
		cleanTables(t)

		uploadUserDoc(t, "user-A", "idCardFront")
		uploadUserDoc(t, "user-B", "passport")

		resp, err := grpcClient.GetUserDocuments(ctx, &filepb.GetUserDocumentsRequest{
			UserId: "user-A",
		})

		require.NoError(t, err)
		assert.Len(t, resp.Documents, 1)
		assert.Equal(t, "user-A", resp.Documents[0].UserId)
	})
}

// =============================================================================
// GetUserDocument
// =============================================================================

func TestE2E_GetUserDocument(t *testing.T) {
	ctx := context.Background()

	t.Run("succès - récupère un document par ID", func(t *testing.T) {
		cleanTables(t)

		uploaded := uploadUserDoc(t, "e2e-get-user", "idCardFront")

		resp, err := grpcClient.GetUserDocument(ctx, &filepb.GetDocumentByIDRequest{
			DocumentId: uploaded.DocumentId,
		})

		require.NoError(t, err)
		assert.Equal(t, uploaded.DocumentId, resp.DocumentId)
		assert.Equal(t, "e2e-get-user", resp.UserId)
	})

	t.Run("erreur - document inexistant", func(t *testing.T) {
		cleanTables(t)

		_, err := grpcClient.GetUserDocument(ctx, &filepb.GetDocumentByIDRequest{
			DocumentId: "nonexistent-id",
		})

		require.Error(t, err)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.NotFound, st.Code())
	})
}

// =============================================================================
// GetCurrentUserDocument
// =============================================================================

func TestE2E_GetCurrentUserDocument(t *testing.T) {
	ctx := context.Background()

	t.Run("succès - retourne le document courant", func(t *testing.T) {
		cleanTables(t)

		uploaded := uploadUserDoc(t, "e2e-current-user", "idCardFront")

		resp, err := grpcClient.GetCurrentUserDocument(ctx, &filepb.GetCurrentUserDocumentRequest{
			UserId:       "e2e-current-user",
			DocumentType: "idCardFront",
		})

		require.NoError(t, err)
		assert.Equal(t, uploaded.DocumentId, resp.DocumentId)
		assert.True(t, resp.IsCurrent)
	})

	t.Run("erreur - aucun document courant", func(t *testing.T) {
		cleanTables(t)

		_, err := grpcClient.GetCurrentUserDocument(ctx, &filepb.GetCurrentUserDocumentRequest{
			UserId:       "user-no-current",
			DocumentType: "passport",
		})

		require.Error(t, err)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.NotFound, st.Code())
	})

	t.Run("erreur - type de document invalide", func(t *testing.T) {
		cleanTables(t)

		_, err := grpcClient.GetCurrentUserDocument(ctx, &filepb.GetCurrentUserDocumentRequest{
			UserId:       "some-user",
			DocumentType: "invalidType",
		})

		require.Error(t, err)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("remplacement - le deuxième upload devient courant", func(t *testing.T) {
		cleanTables(t)

		first := uploadUserDoc(t, "e2e-replace-user", "idCardFront")
		second := uploadUserDoc(t, "e2e-replace-user", "idCardFront")

		resp, err := grpcClient.GetCurrentUserDocument(ctx, &filepb.GetCurrentUserDocumentRequest{
			UserId:       "e2e-replace-user",
			DocumentType: "idCardFront",
		})

		require.NoError(t, err)
		assert.Equal(t, second.DocumentId, resp.DocumentId)
		assert.NotEqual(t, first.DocumentId, resp.DocumentId)
	})

	t.Run("document sans date d expiration - ExpiredAt est vide", func(t *testing.T) {
		cleanTables(t)

		uploadUserDoc(t, "e2e-no-expiry", "idCardFront")

		resp, err := grpcClient.GetCurrentUserDocument(ctx, &filepb.GetCurrentUserDocumentRequest{
			UserId:       "e2e-no-expiry",
			DocumentType: "idCardFront",
		})

		require.NoError(t, err)
		assert.Equal(t, "", resp.ExpiredAt, "ExpiredAt doit être vide si non défini")
	})

	t.Run("document avec date d expiration - ExpiredAt est rempli", func(t *testing.T) {
		cleanTables(t)

		uploaded := uploadUserDoc(t, "e2e-with-expiry", "driverLicenceFront")

		// Mettre à jour expire_at directement en base
		expiry := time.Date(2032, 3, 1, 0, 0, 0, 0, time.UTC)
		_, err := testPool.Exec(ctx,
			`UPDATE user_documents SET expire_at = $1 WHERE document_id = $2`,
			expiry, uploaded.DocumentId,
		)
		require.NoError(t, err)

		resp, err := grpcClient.GetCurrentUserDocument(ctx, &filepb.GetCurrentUserDocumentRequest{
			UserId:       "e2e-with-expiry",
			DocumentType: "driverLicenceFront",
		})

		require.NoError(t, err)
		assert.NotEmpty(t, resp.ExpiredAt, "ExpiredAt doit être non-vide")
		assert.Contains(t, resp.ExpiredAt, "2032", "ExpiredAt doit contenir l'année 2032")
	})
}

// =============================================================================
// GetDocument (avec contrôle d'accès, réponse HTTP-style)
// =============================================================================

func TestE2E_GetDocument(t *testing.T) {
	ctx := context.Background()

	t.Run("succès - accès propriétaire", func(t *testing.T) {
		cleanTables(t)

		uploaded := uploadUserDoc(t, "e2e-owner", "idCardFront")

		resp, err := grpcClient.GetDocument(ctx, &filepb.GetDocumentRequest{
			FileID: uploaded.DocumentId,
			UserID: "e2e-owner",
		})

		require.NoError(t, err)
		require.NotNil(t, resp.File)
		assert.Equal(t, uploaded.DocumentId, resp.File.FileID)
		assert.Empty(t, resp.ErrorMessage)
	})

	t.Run("succès - accès support (bypass propriétaire)", func(t *testing.T) {
		cleanTables(t)

		uploaded := uploadUserDoc(t, "e2e-owner-2", "passport")

		resp, err := grpcClient.GetDocument(ctx, &filepb.GetDocumentRequest{
			FileID:    uploaded.DocumentId,
			SupportID: "support-agent-1",
		})

		require.NoError(t, err)
		require.NotNil(t, resp.File)
		assert.Equal(t, uploaded.DocumentId, resp.File.FileID)
	})

	t.Run("erreur - accès non autorisé (mauvais UserID)", func(t *testing.T) {
		cleanTables(t)

		uploaded := uploadUserDoc(t, "real-owner", "idCardFront")

		resp, err := grpcClient.GetDocument(ctx, &filepb.GetDocumentRequest{
			FileID: uploaded.DocumentId,
			UserID: "other-user",
		})

		require.NoError(t, err) // handler retourne 200 avec ErrorMessage
		assert.Nil(t, resp.File)
		assert.NotEmpty(t, resp.ErrorMessage)
	})

	t.Run("erreur - document inexistant", func(t *testing.T) {
		cleanTables(t)

		resp, err := grpcClient.GetDocument(ctx, &filepb.GetDocumentRequest{
			FileID: "nonexistent-file-id",
			UserID: "some-user",
		})

		require.NoError(t, err) // handler retourne 200 avec ErrorMessage
		assert.Nil(t, resp.File)
		assert.NotEmpty(t, resp.ErrorMessage)
	})
}

// =============================================================================
// DeleteFile (avec contrôle d'accès)
// =============================================================================

func TestE2E_DeleteFile(t *testing.T) {
	ctx := context.Background()

	t.Run("succès - suppression par le propriétaire", func(t *testing.T) {
		cleanTables(t)

		uploaded := uploadUserDoc(t, "e2e-del-owner", "idCardFront")

		resp, err := grpcClient.DeleteFile(ctx, &filepb.DeleteFileRequest{
			UserID: "e2e-del-owner",
			FileID: uploaded.DocumentId,
		})

		require.NoError(t, err)
		assert.True(t, resp.Success)
		assert.Empty(t, resp.ErrorMessage)
	})

	t.Run("erreur - suppression par un autre utilisateur", func(t *testing.T) {
		cleanTables(t)

		uploaded := uploadUserDoc(t, "e2e-real-owner", "passport")

		resp, err := grpcClient.DeleteFile(ctx, &filepb.DeleteFileRequest{
			UserID: "other-user",
			FileID: uploaded.DocumentId,
		})

		require.NoError(t, err) // handler retourne 200 avec Success=false
		assert.False(t, resp.Success)
		assert.NotEmpty(t, resp.ErrorMessage)
	})

	t.Run("erreur - document inexistant", func(t *testing.T) {
		cleanTables(t)

		resp, err := grpcClient.DeleteFile(ctx, &filepb.DeleteFileRequest{
			UserID: "some-user",
			FileID: "nonexistent-id",
		})

		require.NoError(t, err)
		assert.False(t, resp.Success)
		assert.NotEmpty(t, resp.ErrorMessage)
	})
}

// =============================================================================
// DeleteUserDocument (inter-service)
// =============================================================================

func TestE2E_DeleteUserDocument(t *testing.T) {
	ctx := context.Background()

	t.Run("succès - supprime le document", func(t *testing.T) {
		cleanTables(t)

		uploaded := uploadUserDoc(t, "e2e-del-user-doc", "idCardFront")

		resp, err := grpcClient.DeleteUserDocument(ctx, &filepb.DeleteDocumentRequest{
			DocumentId: uploaded.DocumentId,
		})

		require.NoError(t, err)
		assert.True(t, resp.Success)

		// Vérification : le document n'existe plus
		_, err = grpcClient.GetUserDocument(ctx, &filepb.GetDocumentByIDRequest{
			DocumentId: uploaded.DocumentId,
		})
		require.Error(t, err)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.NotFound, st.Code())
	})

	t.Run("erreur - document inexistant", func(t *testing.T) {
		cleanTables(t)

		_, err := grpcClient.DeleteUserDocument(ctx, &filepb.DeleteDocumentRequest{
			DocumentId: "nonexistent-doc",
		})

		require.Error(t, err)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.NotFound, st.Code())
	})
}

// =============================================================================
// UploadVehicleDocument (streaming)
// =============================================================================

func TestE2E_UploadVehicleDocument(t *testing.T) {
	ctx := context.Background()

	t.Run("succès - upload document véhicule", func(t *testing.T) {
		cleanTables(t)

		resp := uploadVehicleDoc(t, "vehicle-001", "insurance")

		assert.NotEmpty(t, resp.DocumentId)
		assert.Equal(t, "vehicle-001", resp.VehicleId)
		assert.Equal(t, "insurance", resp.DocumentType)
		assert.Equal(t, "pending", resp.Status)
		assert.True(t, resp.IsCurrent)
	})

	t.Run("erreur - type de document véhicule invalide", func(t *testing.T) {
		cleanTables(t)

		stream, err := grpcClient.UploadVehicleDocument(ctx)
		require.NoError(t, err)

		_ = stream.Send(&filepb.UploadVehicleDocumentRequest{
			Data: &filepb.UploadVehicleDocumentRequest_Metadata{
				Metadata: &filepb.VehicleDocumentMetadata{
					VehicleId:     "vehicle-002",
					DocumentType:  "invalidType",
					MimeType:      "image/jpeg",
					FileSizeBytes: 512,
				},
			},
		})
		_ = stream.Send(&filepb.UploadVehicleDocumentRequest{
			Data: &filepb.UploadVehicleDocumentRequest_Chunk{Chunk: fakeJPEG()},
		})

		_, err = stream.CloseAndRecv()

		require.Error(t, err)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})
}

// =============================================================================
// GetVehicleDocuments / GetVehicleDocument
// =============================================================================

func TestE2E_GetVehicleDocuments(t *testing.T) {
	ctx := context.Background()

	t.Run("succès - retourne tous les documents du véhicule", func(t *testing.T) {
		cleanTables(t)

		uploadVehicleDoc(t, "veh-list-1", "insurance")
		uploadVehicleDoc(t, "veh-list-1", "registrationCard")

		resp, err := grpcClient.GetVehicleDocuments(ctx, &filepb.GetVehicleDocumentsRequest{
			VehicleId: "veh-list-1",
		})

		require.NoError(t, err)
		assert.Len(t, resp.Documents, 2)
	})

	t.Run("succès - liste vide quand aucun document", func(t *testing.T) {
		cleanTables(t)

		resp, err := grpcClient.GetVehicleDocuments(ctx, &filepb.GetVehicleDocumentsRequest{
			VehicleId: "veh-no-docs",
		})

		require.NoError(t, err)
		assert.Empty(t, resp.Documents)
	})
}

func TestE2E_GetVehicleDocument(t *testing.T) {
	ctx := context.Background()

	t.Run("succès - récupère un document véhicule par ID", func(t *testing.T) {
		cleanTables(t)

		uploaded := uploadVehicleDoc(t, "veh-get-1", "insurance")

		resp, err := grpcClient.GetVehicleDocument(ctx, &filepb.GetDocumentByIDRequest{
			DocumentId: uploaded.DocumentId,
		})

		require.NoError(t, err)
		assert.Equal(t, uploaded.DocumentId, resp.DocumentId)
		assert.Equal(t, "veh-get-1", resp.VehicleId)
	})

	t.Run("erreur - document véhicule inexistant", func(t *testing.T) {
		cleanTables(t)

		_, err := grpcClient.GetVehicleDocument(ctx, &filepb.GetDocumentByIDRequest{
			DocumentId: "nonexistent-vehicle-doc",
		})

		require.Error(t, err)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.NotFound, st.Code())
	})
}

// =============================================================================
// DeleteVehicleDocument
// =============================================================================

func TestE2E_DeleteVehicleDocument(t *testing.T) {
	ctx := context.Background()

	t.Run("succès - supprime le document véhicule", func(t *testing.T) {
		cleanTables(t)

		uploaded := uploadVehicleDoc(t, "veh-del-1", "registrationCard")

		resp, err := grpcClient.DeleteVehicleDocument(ctx, &filepb.DeleteDocumentRequest{
			DocumentId: uploaded.DocumentId,
		})

		require.NoError(t, err)
		assert.True(t, resp.Success)

		// Vérification : le document n'existe plus
		_, err = grpcClient.GetVehicleDocument(ctx, &filepb.GetDocumentByIDRequest{
			DocumentId: uploaded.DocumentId,
		})
		require.Error(t, err)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.NotFound, st.Code())
	})

	t.Run("erreur - document véhicule inexistant", func(t *testing.T) {
		cleanTables(t)

		_, err := grpcClient.DeleteVehicleDocument(ctx, &filepb.DeleteDocumentRequest{
			DocumentId: "nonexistent-vehicle-doc",
		})

		require.Error(t, err)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.NotFound, st.Code())
	})
}

// =============================================================================
// CreateDocumentReview
// =============================================================================

func TestE2E_CreateDocumentReview(t *testing.T) {
	ctx := context.Background()

	t.Run("succès - revue pour document utilisateur", func(t *testing.T) {
		cleanTables(t)

		doc := uploadUserDoc(t, "e2e-review-user", "idCardFront")

		resp, err := grpcClient.CreateDocumentReview(ctx, &filepb.CreateDocumentReviewRequest{
			UserDocumentId: doc.DocumentId,
			Decision:       "approved",
			ReviewType:     "manual",
			ReviewedBy:     "admin-1",
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.NotEmpty(t, resp.ReviewId)
		assert.Equal(t, doc.DocumentId, resp.UserDocumentId)
		assert.Equal(t, "approved", resp.Decision)
		assert.Equal(t, "manual", resp.ReviewType)
		assert.Equal(t, int32(1), resp.AttemptNumber) // valeur par défaut
	})

	t.Run("succès - revue pour document véhicule", func(t *testing.T) {
		cleanTables(t)

		doc := uploadVehicleDoc(t, "veh-review-1", "insurance")

		resp, err := grpcClient.CreateDocumentReview(ctx, &filepb.CreateDocumentReviewRequest{
			VehicleDocumentId: doc.DocumentId,
			Decision:          "rejected",
			ReviewType:        "automatic",
			ReviewedBy:        "bot-1",
			ReasonRejection:   "document_expired",
		})

		require.NoError(t, err)
		assert.Equal(t, doc.DocumentId, resp.VehicleDocumentId)
		assert.Equal(t, "rejected", resp.Decision)
	})

	t.Run("erreur - décision invalide", func(t *testing.T) {
		cleanTables(t)

		doc := uploadUserDoc(t, "e2e-review-err", "passport")

		_, err := grpcClient.CreateDocumentReview(ctx, &filepb.CreateDocumentReviewRequest{
			UserDocumentId: doc.DocumentId,
			Decision:       "maybe",
			ReviewType:     "manual",
		})

		require.Error(t, err)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("erreur - aucune référence de document", func(t *testing.T) {
		cleanTables(t)

		_, err := grpcClient.CreateDocumentReview(ctx, &filepb.CreateDocumentReviewRequest{
			Decision:   "approved",
			ReviewType: "manual",
		})

		require.Error(t, err)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("erreur - les deux références renseignées", func(t *testing.T) {
		cleanTables(t)

		userDoc := uploadUserDoc(t, "e2e-both-refs", "idCardFront")
		vehicleDoc := uploadVehicleDoc(t, "veh-both-refs", "insurance")

		_, err := grpcClient.CreateDocumentReview(ctx, &filepb.CreateDocumentReviewRequest{
			UserDocumentId:    userDoc.DocumentId,
			VehicleDocumentId: vehicleDoc.DocumentId,
			Decision:          "approved",
			ReviewType:        "manual",
		})

		require.Error(t, err)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("erreur - document référencé inexistant", func(t *testing.T) {
		cleanTables(t)

		_, err := grpcClient.CreateDocumentReview(ctx, &filepb.CreateDocumentReviewRequest{
			UserDocumentId: "nonexistent-doc-id",
			Decision:       "approved",
			ReviewType:     "manual",
		})

		require.Error(t, err)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.NotFound, st.Code())
	})
}

// =============================================================================
// GetDocumentReviews
// =============================================================================

func TestE2E_GetDocumentReviews(t *testing.T) {
	ctx := context.Background()

	t.Run("succès - récupère les revues d'un document utilisateur", func(t *testing.T) {
		cleanTables(t)

		doc := uploadUserDoc(t, "e2e-reviews-get", "passport")

		_, err := grpcClient.CreateDocumentReview(ctx, &filepb.CreateDocumentReviewRequest{
			UserDocumentId: doc.DocumentId,
			Decision:       "approved",
			ReviewType:     "manual",
		})
		require.NoError(t, err)
		_, err = grpcClient.CreateDocumentReview(ctx, &filepb.CreateDocumentReviewRequest{
			UserDocumentId: doc.DocumentId,
			Decision:       "rejected",
			ReviewType:     "manual",
			ReasonRejection: "document_illegible",
		})
		require.NoError(t, err)

		resp, err := grpcClient.GetDocumentReviews(ctx, &filepb.GetDocumentReviewsRequest{
			UserDocumentId: doc.DocumentId,
		})

		require.NoError(t, err)
		assert.Len(t, resp.Reviews, 2)
		for _, r := range resp.Reviews {
			assert.Equal(t, doc.DocumentId, r.UserDocumentId)
		}
	})

	t.Run("succès - récupère les revues d'un document véhicule", func(t *testing.T) {
		cleanTables(t)

		doc := uploadVehicleDoc(t, "veh-reviews-get", "registrationCard")

		_, err := grpcClient.CreateDocumentReview(ctx, &filepb.CreateDocumentReviewRequest{
			VehicleDocumentId: doc.DocumentId,
			Decision:          "approved",
			ReviewType:        "automatic",
		})
		require.NoError(t, err)

		resp, err := grpcClient.GetDocumentReviews(ctx, &filepb.GetDocumentReviewsRequest{
			VehicleDocumentId: doc.DocumentId,
		})

		require.NoError(t, err)
		assert.Len(t, resp.Reviews, 1)
		assert.Equal(t, doc.DocumentId, resp.Reviews[0].VehicleDocumentId)
	})

	t.Run("succès - liste vide quand aucune revue", func(t *testing.T) {
		cleanTables(t)

		doc := uploadUserDoc(t, "e2e-no-reviews", "idCardFront")

		resp, err := grpcClient.GetDocumentReviews(ctx, &filepb.GetDocumentReviewsRequest{
			UserDocumentId: doc.DocumentId,
		})

		require.NoError(t, err)
		assert.Empty(t, resp.Reviews)
	})
}

// =============================================================================
// Scénario complet
// =============================================================================

func TestE2E_FullScenario(t *testing.T) {
	ctx := context.Background()
	cleanTables(t)

	userID := "e2e-full-user"
	vehicleID := "e2e-full-vehicle"

	// 1. Health check
	health, err := grpcClient.Health(ctx, &filepb.HealthRequest{})
	require.NoError(t, err)
	assert.Equal(t, "healthy", health.Status)

	// 2. Aucun document au départ
	docsResp, err := grpcClient.GetUserDocuments(ctx, &filepb.GetUserDocumentsRequest{UserId: userID})
	require.NoError(t, err)
	assert.Empty(t, docsResp.Documents)

	// 3. Upload carte d'identité (recto)
	idCardFront := uploadUserDoc(t, userID, "idCardFront")
	assert.NotEmpty(t, idCardFront.DocumentId)
	assert.Equal(t, "pending", idCardFront.Status)
	assert.True(t, idCardFront.IsCurrent)

	// 4. Upload passeport
	passport := uploadUserDoc(t, userID, "passport")
	assert.NotEmpty(t, passport.DocumentId)

	// 5. Vérifier 2 documents
	docsResp, err = grpcClient.GetUserDocuments(ctx, &filepb.GetUserDocumentsRequest{UserId: userID})
	require.NoError(t, err)
	assert.Len(t, docsResp.Documents, 2)

	// 6. Créer une revue pour la carte d'identité
	review, err := grpcClient.CreateDocumentReview(ctx, &filepb.CreateDocumentReviewRequest{
		UserDocumentId: idCardFront.DocumentId,
		Decision:       "approved",
		ReviewType:     "manual",
		ReviewedBy:     "admin-1",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, review.ReviewId)
	assert.Equal(t, "approved", review.Decision)

	// 7. Récupérer les revues
	reviewsResp, err := grpcClient.GetDocumentReviews(ctx, &filepb.GetDocumentReviewsRequest{
		UserDocumentId: idCardFront.DocumentId,
	})
	require.NoError(t, err)
	assert.Len(t, reviewsResp.Reviews, 1)

	// 8. Upload un nouveau recto de carte (remplace l'ancien)
	idCardFront2 := uploadUserDoc(t, userID, "idCardFront")
	assert.NotEqual(t, idCardFront.DocumentId, idCardFront2.DocumentId)

	// 9. Le nouveau document est courant
	currentDoc, err := grpcClient.GetCurrentUserDocument(ctx, &filepb.GetCurrentUserDocumentRequest{
		UserId:       userID,
		DocumentType: "idCardFront",
	})
	require.NoError(t, err)
	assert.Equal(t, idCardFront2.DocumentId, currentDoc.DocumentId)

	// 10. GetDocument avec accès propriétaire
	docResp, err := grpcClient.GetDocument(ctx, &filepb.GetDocumentRequest{
		FileID: idCardFront2.DocumentId,
		UserID: userID,
	})
	require.NoError(t, err)
	assert.NotNil(t, docResp.File)
	assert.Equal(t, idCardFront2.DocumentId, docResp.File.FileID)

	// 11. GetDocument avec support bypass
	docRespSupport, err := grpcClient.GetDocument(ctx, &filepb.GetDocumentRequest{
		FileID:    idCardFront2.DocumentId,
		SupportID: "support-agent-001",
	})
	require.NoError(t, err)
	assert.NotNil(t, docRespSupport.File)

	// 12. Upload documents véhicule
	insurance := uploadVehicleDoc(t, vehicleID, "insurance")
	registration := uploadVehicleDoc(t, vehicleID, "registrationCard")
	assert.NotEmpty(t, insurance.DocumentId)
	assert.NotEmpty(t, registration.DocumentId)

	// 13. Vérifier 2 documents véhicule
	vehDocsResp, err := grpcClient.GetVehicleDocuments(ctx, &filepb.GetVehicleDocumentsRequest{
		VehicleId: vehicleID,
	})
	require.NoError(t, err)
	assert.Len(t, vehDocsResp.Documents, 2)

	// 14. Revue véhicule
	vehReview, err := grpcClient.CreateDocumentReview(ctx, &filepb.CreateDocumentReviewRequest{
		VehicleDocumentId: insurance.DocumentId,
		Decision:          "resubmission",
		ReviewType:        "automatic",
		ReasonRejection:   "document_incomplete",
	})
	require.NoError(t, err)
	assert.Equal(t, "resubmission", vehReview.Decision)

	// 15. Suppression d'un document utilisateur (inter-service)
	delResp, err := grpcClient.DeleteUserDocument(ctx, &filepb.DeleteDocumentRequest{
		DocumentId: passport.DocumentId,
	})
	require.NoError(t, err)
	assert.True(t, delResp.Success)

	// 16. Le passeport n'existe plus
	_, err = grpcClient.GetUserDocument(ctx, &filepb.GetDocumentByIDRequest{
		DocumentId: passport.DocumentId,
	})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.NotFound, st.Code())

	// 17. Il reste 2 documents (idCardFront original + idCardFront2)
	docsResp, err = grpcClient.GetUserDocuments(ctx, &filepb.GetUserDocumentsRequest{UserId: userID})
	require.NoError(t, err)
	assert.Len(t, docsResp.Documents, 2)
}
