package grpc_test

import (
	"context"
	"testing"

	grpcHandler "github.com/Kpeewu/tissi-mah/services/file-service/internal/grpc"
	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/file-service/internal/service/interfaces"
	fileErrors "github.com/Kpeewu/tissi-mah/services/file-service/pkg/errors"
	filepb "github.com/Kpeewu/tissi-mah/services/file-service/proto/gen"
	"github.com/Kpeewu/tissi-mah/services/file-service/tests/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// spyChangeDocument capture l'input reçu par ChangeDocument.
type spyChangeDocument struct {
	spyFileService
	gotChangeInput *serviceInterfaces.ChangeDocumentInput
	changeErr      error
	changeResult   *serviceInterfaces.UploadedDocument
}

func (s *spyChangeDocument) ChangeDocument(_ context.Context, input serviceInterfaces.ChangeDocumentInput) (*serviceInterfaces.UploadedDocument, error) {
	s.gotChangeInput = &input
	return s.changeResult, s.changeErr
}

func newChangeDocHandler(spy *spyChangeDocument) filepb.FileServiceServer {
	return grpcHandler.NewFileHandler(spy, new(mocks.MockUserClient), nil, new(mocks.MockStorageClient), zap.NewNop())
}

// =============================================================================
// Tests toGRPCError — nouveaux codes de retour
// =============================================================================

func TestChangeDocument_ErrorDocumentAlreadySubmitted_ReturnsAlreadyExists(t *testing.T) {
	spy := &spyChangeDocument{changeErr: fileErrors.ErrorDocumentAlreadySubmitted}
	h := newChangeDocHandler(spy)

	resp, err := h.ChangeDocument(context.Background(), &filepb.ChangeDocumentRequest{
		UserID:      "user-1",
		FileID:      "doc-1",
		NewDocument: []byte("data"),
	})

	// ChangeDocument retourne l'erreur via HTTP 200 avec ErrorMessage (même pattern que Upload*)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.False(t, resp.Success)
	assert.Equal(t, fileErrors.ErrorDocumentAlreadySubmitted.Error(), resp.ErrorMessage)
}

func TestChangeDocument_ErrorDocumentNotReplaceable_FailedPrecondition(t *testing.T) {
	spy := &spyChangeDocument{changeErr: fileErrors.ErrorDocumentNotReplaceable}
	h := newChangeDocHandler(spy)

	resp, err := h.ChangeDocument(context.Background(), &filepb.ChangeDocumentRequest{
		UserID:      "user-1",
		FileID:      "doc-1",
		NewDocument: []byte("data"),
	})

	require.NoError(t, err)
	assert.False(t, resp.Success)
	assert.Equal(t, fileErrors.ErrorDocumentNotReplaceable.Error(), resp.ErrorMessage)
}

func TestChangeDocument_ErrorMissingDocumentMetadata_ErrorMessage(t *testing.T) {
	spy := &spyChangeDocument{changeErr: fileErrors.ErrorMissingDocumentMetadata}
	h := newChangeDocHandler(spy)

	resp, err := h.ChangeDocument(context.Background(), &filepb.ChangeDocumentRequest{
		UserID:      "user-1",
		FileID:      "doc-1",
		NewDocument: []byte("data"),
	})

	require.NoError(t, err)
	assert.False(t, resp.Success)
	assert.Equal(t, fileErrors.ErrorMissingDocumentMetadata.Error(), resp.ErrorMessage)
}

// TestChangeDocument_ForwardsMetadataFields vérifie que le handler transmet les champs
// optionnels DocumentNumber, IssuedAt, ExpireAt, IssuingPlace au service.
func TestChangeDocument_ForwardsMetadataFields(t *testing.T) {
	spy := &spyChangeDocument{
		changeResult: &serviceInterfaces.UploadedDocument{
			DocumentID: "new-doc",
		},
	}
	h := newChangeDocHandler(spy)

	_, err := h.ChangeDocument(context.Background(), &filepb.ChangeDocumentRequest{
		UserID:         "user-1",
		FileID:         "doc-1",
		NewDocument:    []byte("data"),
		DocumentNumber: "NUM-123",
		IssuedAt:       "2023-06-01T00:00:00Z",
		ExpireAt:       "2028-06-01T00:00:00Z",
		IssuingPlace:   "TG",
	})

	require.NoError(t, err)
	require.NotNil(t, spy.gotChangeInput)
	assert.Equal(t, "user-1", spy.gotChangeInput.UserID)
	assert.Equal(t, "doc-1", spy.gotChangeInput.FileID)
	assert.Equal(t, "NUM-123", spy.gotChangeInput.DocumentNumber)
	assert.Equal(t, "2023-06-01T00:00:00Z", spy.gotChangeInput.IssuedAt)
	assert.Equal(t, "2028-06-01T00:00:00Z", spy.gotChangeInput.ExpireAt)
	assert.Equal(t, "TG", spy.gotChangeInput.IssuingPlace)
}

// TestChangeDocument_OptionalMetadataEmptyByDefault vérifie que les champs optionnels
// sont vides par défaut (pas de valeurs fantômes).
func TestChangeDocument_OptionalMetadataEmptyByDefault(t *testing.T) {
	spy := &spyChangeDocument{
		changeResult: &serviceInterfaces.UploadedDocument{DocumentID: "new-doc"},
	}
	h := newChangeDocHandler(spy)

	_, err := h.ChangeDocument(context.Background(), &filepb.ChangeDocumentRequest{
		UserID:      "user-1",
		FileID:      "doc-1",
		NewDocument: []byte("data"),
		// Pas de DocumentNumber, IssuedAt, ExpireAt, IssuingPlace
	})

	require.NoError(t, err)
	require.NotNil(t, spy.gotChangeInput)
	assert.Empty(t, spy.gotChangeInput.DocumentNumber)
	assert.Empty(t, spy.gotChangeInput.IssuedAt)
	assert.Empty(t, spy.gotChangeInput.ExpireAt)
	assert.Empty(t, spy.gotChangeInput.IssuingPlace)
}

// spyKycError permet de déclencher toGRPCError via ListKycDocuments.
type spyKycError struct {
	spyFileService
	kycErr error
}

func (s *spyKycError) ListKycDocuments(_ context.Context, _ []string) ([]*serviceInterfaces.KycDocument, error) {
	return nil, s.kycErr
}

// =============================================================================
// Tests codes gRPC via toGRPCError (nouveaux codes ajoutés)
// =============================================================================

func TestToGRPCError_AlreadyExists(t *testing.T) {
	spy := &spyKycError{kycErr: fileErrors.ErrorDocumentAlreadySubmitted}
	h := grpcHandler.NewFileHandler(spy, new(mocks.MockUserClient), nil, new(mocks.MockStorageClient), zap.NewNop())

	_, err := h.ListKycDocuments(context.Background(), &filepb.ListKycDocumentsRequest{})

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.AlreadyExists, st.Code())
}

func TestToGRPCError_FailedPrecondition(t *testing.T) {
	spy := &spyKycError{kycErr: fileErrors.ErrorDocumentNotReplaceable}
	h := grpcHandler.NewFileHandler(spy, new(mocks.MockUserClient), nil, new(mocks.MockStorageClient), zap.NewNop())

	_, err := h.ListKycDocuments(context.Background(), &filepb.ListKycDocumentsRequest{})

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.FailedPrecondition, st.Code())
}

func TestToGRPCError_InvalidArgument_MissingMetadata(t *testing.T) {
	spy := &spyKycError{kycErr: fileErrors.ErrorMissingDocumentMetadata}
	h := grpcHandler.NewFileHandler(spy, new(mocks.MockUserClient), nil, new(mocks.MockStorageClient), zap.NewNop())

	_, err := h.ListKycDocuments(context.Background(), &filepb.ListKycDocumentsRequest{})

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}
