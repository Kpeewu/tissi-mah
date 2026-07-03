package e2e

import (
	"context"
	"testing"

	filepb "github.com/Kpeewu/tissi-mah/services/file-service/proto/gen"
	"github.com/stretchr/testify/require"
)

// cleanTables vide toutes les tables entre chaque test.
func cleanTables(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	_, err := testPool.Exec(ctx, `DELETE FROM document_reviews`)
	require.NoError(t, err)
	_, err = testPool.Exec(ctx, `DELETE FROM user_documents`)
	require.NoError(t, err)
	_, err = testPool.Exec(ctx, `DELETE FROM vehicle_documents`)
	require.NoError(t, err)
}

// fakeJPEG retourne des bytes simulant un JPEG valide (magic bytes + padding).
func fakeJPEG() []byte {
	data := make([]byte, 512)
	data[0] = 0xFF
	data[1] = 0xD8
	data[2] = 0xFF
	return data
}

// uploadUserDoc effectue un upload streaming gRPC d'un document utilisateur.
func uploadUserDoc(t *testing.T, userID, docType string) *filepb.UserDocumentResponse {
	t.Helper()
	ctx := context.Background()

	stream, err := grpcClient.UploadUserDocument(ctx)
	require.NoError(t, err)

	meta := &filepb.UserDocumentMetadata{
		UserId:        userID,
		DocumentName:  docType + "_doc",
		DocumentType:  docType,
		MimeType:      "image/jpeg",
		FileSizeBytes: int64(len(fakeJPEG())),
	}
	// profilePicture est exempt de la contrainte issued_at / expire_at.
	if docType != "profilePicture" {
		meta.DocumentNumber = "TEST-001"
		meta.IssuingCountry = "TG"
		meta.IssuedAt = "2020-01-01T00:00:00Z"
		meta.ExpireAt = "2030-01-01T00:00:00Z"
	}
	err = stream.Send(&filepb.UploadUserDocumentRequest{
		Data: &filepb.UploadUserDocumentRequest_Metadata{Metadata: meta},
	})
	require.NoError(t, err)

	err = stream.Send(&filepb.UploadUserDocumentRequest{
		Data: &filepb.UploadUserDocumentRequest_Chunk{
			Chunk: fakeJPEG(),
		},
	})
	require.NoError(t, err)

	resp, err := stream.CloseAndRecv()
	require.NoError(t, err)
	require.NotNil(t, resp)
	return resp
}

// uploadVehicleDoc effectue un upload streaming gRPC d'un document véhicule.
func uploadVehicleDoc(t *testing.T, vehicleID, docType string) *filepb.VehicleDocumentResponse {
	t.Helper()
	ctx := context.Background()

	stream, err := grpcClient.UploadVehicleDocument(ctx)
	require.NoError(t, err)

	err = stream.Send(&filepb.UploadVehicleDocumentRequest{
		Data: &filepb.UploadVehicleDocumentRequest_Metadata{
			Metadata: &filepb.VehicleDocumentMetadata{
				VehicleId:        vehicleID,
				DocumentName:     docType + "_doc",
				DocumentType:     docType,
				MimeType:         "image/jpeg",
				FileSizeBytes:    int64(len(fakeJPEG())),
				DocumentNumber:   "VEH-001",
				IssuingAuthority: "DVLA-TG",
				IssuedAt:         "2020-01-01T00:00:00Z",
				ExpireAt:         "2030-01-01T00:00:00Z",
			},
		},
	})
	require.NoError(t, err)

	err = stream.Send(&filepb.UploadVehicleDocumentRequest{
		Data: &filepb.UploadVehicleDocumentRequest_Chunk{
			Chunk: fakeJPEG(),
		},
	})
	require.NoError(t, err)

	resp, err := stream.CloseAndRecv()
	require.NoError(t, err)
	require.NotNil(t, resp)
	return resp
}
