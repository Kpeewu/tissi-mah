package grpc_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Kpeewu/tissi-mah/services/file-service/internal/client"
	"github.com/Kpeewu/tissi-mah/services/file-service/internal/domain"
	grpcHandler "github.com/Kpeewu/tissi-mah/services/file-service/internal/grpc"
	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/file-service/internal/service/interfaces"
	fileErrors "github.com/Kpeewu/tissi-mah/services/file-service/pkg/errors"
	filepb "github.com/Kpeewu/tissi-mah/services/file-service/proto/gen"
	"github.com/Kpeewu/tissi-mah/services/file-service/tests/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// spyFileService capture les inputs reçus par UploadIdDocument et UploadVehicleDocuments.
// Les autres méthodes panic car le handler ne les appelle pas.
type spyFileService struct {
	gotInput         *serviceInterfaces.UploadIdDocumentInput
	uploadIdErr      error
	uploadIdReturn   []*serviceInterfaces.UploadedDocument
	gotVehicleInput  *serviceInterfaces.UploadVehicleDocumentsInput
	uploadVehicleErr error
	uploadVehicleRet []*serviceInterfaces.UploadedDocument
}

func (s *spyFileService) UploadIdDocument(_ context.Context, input serviceInterfaces.UploadIdDocumentInput) ([]*serviceInterfaces.UploadedDocument, error) {
	s.gotInput = &input
	return s.uploadIdReturn, s.uploadIdErr
}

func (s *spyFileService) UploadVehicleDocuments(_ context.Context, input serviceInterfaces.UploadVehicleDocumentsInput) ([]*serviceInterfaces.UploadedDocument, error) {
	s.gotVehicleInput = &input
	return s.uploadVehicleRet, s.uploadVehicleErr
}

// --- Stubs pour satisfaire l'interface FileService ---

func (s *spyFileService) UploadUserDocument(context.Context, serviceInterfaces.UploadUserDocumentInput) (*domain.UserDocument, error) {
	panic("not implemented")
}
func (s *spyFileService) GetUserDocuments(context.Context, string) ([]*domain.UserDocument, error) {
	panic("not implemented")
}
func (s *spyFileService) GetUserDocument(context.Context, string) (*domain.UserDocument, error) {
	panic("not implemented")
}
func (s *spyFileService) GetCurrentUserDocument(context.Context, string, string) (*domain.UserDocument, error) {
	panic("not implemented")
}
func (s *spyFileService) GetDocument(context.Context, serviceInterfaces.GetDocumentInput) (*serviceInterfaces.GetDocumentResult, error) {
	panic("not implemented")
}
func (s *spyFileService) DeleteFile(context.Context, serviceInterfaces.DeleteFileInput) error {
	panic("not implemented")
}
func (s *spyFileService) DeleteUserDocument(context.Context, string) error {
	panic("not implemented")
}
func (s *spyFileService) UploadVehicleDocument(context.Context, serviceInterfaces.UploadVehicleDocumentInput) (*domain.VehicleDocument, error) {
	panic("not implemented")
}
func (s *spyFileService) GetVehicleDocuments(context.Context, string) ([]*domain.VehicleDocument, error) {
	panic("not implemented")
}
func (s *spyFileService) GetVehicleDocument(context.Context, string) (*domain.VehicleDocument, error) {
	panic("not implemented")
}
func (s *spyFileService) GetVehicleDocumentsByUserID(context.Context, string) ([]*domain.VehicleDocument, error) {
	panic("not implemented")
}
func (s *spyFileService) ListKycDocuments(context.Context, []string) ([]*serviceInterfaces.KycDocument, error) {
	panic("not implemented")
}
func (s *spyFileService) DeleteVehicleDocument(context.Context, string) error {
	panic("not implemented")
}
func (s *spyFileService) ChangeDocument(context.Context, serviceInterfaces.ChangeDocumentInput) (*serviceInterfaces.UploadedDocument, error) {
	panic("not implemented")
}
func (s *spyFileService) CreateDocumentReview(context.Context, serviceInterfaces.CreateReviewInput) (*domain.DocumentReview, error) {
	panic("not implemented")
}
func (s *spyFileService) GetDocumentReview(context.Context, string) (*domain.DocumentReview, error) {
	panic("not implemented")
}
func (s *spyFileService) GetDocumentReviews(context.Context, string, string) ([]*domain.DocumentReview, error) {
	panic("not implemented")
}
func (s *spyFileService) GetDocumentReviewByPersonaInquiryID(context.Context, string) (*domain.DocumentReview, error) {
	panic("not implemented")
}
func (s *spyFileService) GetDocumentReviewsByUserID(context.Context, string) ([]*domain.DocumentReview, error) {
	panic("not implemented")
}
func (s *spyFileService) UpdateDocumentReview(context.Context, *domain.DocumentReview) (*domain.DocumentReview, error) {
	panic("not implemented")
}
func (s *spyFileService) ListDocumentReviews(context.Context, string, string, string, int32, int32) ([]*domain.DocumentReview, error) {
	panic("not implemented")
}
func (s *spyFileService) GetDocumentReviewHistory(context.Context, string, string) ([]*domain.DocumentReview, error) {
	panic("not implemented")
}
func (s *spyFileService) DeleteAllUserFiles(context.Context, string) error { panic("not implemented") }

func ctxWithFirebaseUID(uid string) context.Context {
	md := metadata.New(map[string]string{"x-firebase-uid": uid})
	return metadata.NewIncomingContext(context.Background(), md)
}

func TestUploadIdDocument_MissingMetadata_ReturnsUnauthenticated(t *testing.T) {
	spy := &spyFileService{}
	mockUser := new(mocks.MockUserClient)
	h := grpcHandler.NewFileHandler(spy, mockUser, nil, new(mocks.MockStorageClient), zap.NewNop())

	_, err := h.UploadIdDocument(context.Background(), &filepb.UploadIdDocumentRequest{
		UserID:       "firebaseXYZ",
		DocumentType: "DriverLicence",
	})

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, st.Code())
	mockUser.AssertNotCalled(t, "GetUserProfileByFirebaseID")
	assert.Nil(t, spy.gotInput)
}

func TestUploadIdDocument_MissingFirebaseUID_ReturnsUnauthenticated(t *testing.T) {
	spy := &spyFileService{}
	mockUser := new(mocks.MockUserClient)
	h := grpcHandler.NewFileHandler(spy, mockUser, nil, new(mocks.MockStorageClient), zap.NewNop())

	ctx := metadata.NewIncomingContext(context.Background(), metadata.New(map[string]string{}))
	_, err := h.UploadIdDocument(ctx, &filepb.UploadIdDocumentRequest{
		UserID:       "firebaseXYZ",
		DocumentType: "DriverLicence",
	})

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, st.Code())
	mockUser.AssertNotCalled(t, "GetUserProfileByFirebaseID")
	assert.Nil(t, spy.gotInput)
}

func TestUploadIdDocument_UserServiceError_ReturnsErrorMessage(t *testing.T) {
	spy := &spyFileService{}
	mockUser := new(mocks.MockUserClient)
	mockUser.On("GetUserProfileByFirebaseID", mock.Anything, "firebaseXYZ").
		Return((*client.UserProfile)(nil), errors.New("user-service down"))
	h := grpcHandler.NewFileHandler(spy, mockUser, nil, new(mocks.MockStorageClient), zap.NewNop())

	resp, err := h.UploadIdDocument(ctxWithFirebaseUID("firebaseXYZ"), &filepb.UploadIdDocumentRequest{
		DocumentType: "DriverLicence",
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.False(t, resp.Success)
	assert.Equal(t, fileErrors.ErrorUserServiceUnavailable.Error(), resp.ErrorMessage)
	assert.Nil(t, spy.gotInput, "service ne doit pas etre appele si user-service echoue")
	mockUser.AssertExpectations(t)
}

func TestUploadIdDocument_ResolvesFirebaseToProfileAndForwardsInternalUUID(t *testing.T) {
	spy := &spyFileService{}
	mockUser := new(mocks.MockUserClient)
	mockUser.On("GetUserProfileByFirebaseID", mock.Anything, "firebaseXYZ").
		Return(&client.UserProfile{UserID: "uuid-abc", FirstName: "Jean", LastName: "Dupont"}, nil)
	h := grpcHandler.NewFileHandler(spy, mockUser, nil, new(mocks.MockStorageClient), zap.NewNop())

	resp, err := h.UploadIdDocument(ctxWithFirebaseUID("firebaseXYZ"), &filepb.UploadIdDocumentRequest{
		UserID:             "firebaseXYZ", // body-supplied, doit etre ignore
		DocumentType:       "DriverLicence",
		DriverLicenceRecto: []byte("recto-bytes"),
		DriverLicenceVerso: []byte("verso-bytes"),
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.True(t, resp.Success)
	require.NotNil(t, spy.gotInput)
	assert.Equal(t, "uuid-abc", spy.gotInput.UserID, "le service doit recevoir l'UUID interne, pas le Firebase UID")
	assert.NotEqual(t, "firebaseXYZ", spy.gotInput.UserID, "le champ req.UserID du body doit etre ignore")
	assert.Equal(t, "Jean", spy.gotInput.FirstName, "le service doit recevoir le prenom du profil user-service")
	assert.Equal(t, "Dupont", spy.gotInput.LastName, "le service doit recevoir le nom du profil user-service")
	assert.Equal(t, "DriverLicence", spy.gotInput.DocumentType)
	assert.Equal(t, []byte("recto-bytes"), spy.gotInput.DriverLicenceRecto)
	assert.Equal(t, []byte("verso-bytes"), spy.gotInput.DriverLicenceVerso)
	mockUser.AssertExpectations(t)
}

func TestUploadIdDocument_IgnoresBodyUserIDEvenWhenDifferentFromMetadata(t *testing.T) {
	spy := &spyFileService{}
	mockUser := new(mocks.MockUserClient)
	mockUser.On("GetUserProfileByFirebaseID", mock.Anything, "legitFirebaseUID").
		Return(&client.UserProfile{UserID: "internal-from-legit", FirstName: "Anne", LastName: "Martin"}, nil)
	h := grpcHandler.NewFileHandler(spy, mockUser, nil, new(mocks.MockStorageClient), zap.NewNop())

	// Un attaquant met un Firebase UID autre dans le body mais possede son propre JWT legitime.
	// Le handler doit utiliser l'UID du JWT (metadata) et ignorer le body.
	resp, err := h.UploadIdDocument(ctxWithFirebaseUID("legitFirebaseUID"), &filepb.UploadIdDocumentRequest{
		UserID:       "forgedFirebaseUIDOfAnotherUser",
		DocumentType: "Passport",
		Passport:     []byte("passport-bytes"),
	})

	require.NoError(t, err)
	require.True(t, resp.Success)
	require.NotNil(t, spy.gotInput)
	assert.Equal(t, "internal-from-legit", spy.gotInput.UserID)
	mockUser.AssertExpectations(t)
}

func TestUploadIdDocument_ServiceError_ReturnsErrorMessage(t *testing.T) {
	spy := &spyFileService{uploadIdErr: fileErrors.ErrorUploadFailed}
	mockUser := new(mocks.MockUserClient)
	mockUser.On("GetUserProfileByFirebaseID", mock.Anything, "firebaseXYZ").
		Return(&client.UserProfile{UserID: "uuid-abc", FirstName: "Jean", LastName: "Dupont"}, nil)
	h := grpcHandler.NewFileHandler(spy, mockUser, nil, new(mocks.MockStorageClient), zap.NewNop())

	resp, err := h.UploadIdDocument(ctxWithFirebaseUID("firebaseXYZ"), &filepb.UploadIdDocumentRequest{
		DocumentType: "DriverLicence",
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.False(t, resp.Success)
	assert.Equal(t, fileErrors.ErrorUploadFailed.Error(), resp.ErrorMessage)
	require.NotNil(t, spy.gotInput)
	assert.Equal(t, "uuid-abc", spy.gotInput.UserID)
}

func TestUploadIdDocument_ForwardsMetadataFields(t *testing.T) {
	spy := &spyFileService{}
	mockUser := new(mocks.MockUserClient)
	mockUser.On("GetUserProfileByFirebaseID", mock.Anything, "firebaseXYZ").
		Return(&client.UserProfile{UserID: "uuid-abc", FirstName: "Jean", LastName: "Dupont"}, nil)
	h := grpcHandler.NewFileHandler(spy, mockUser, nil, new(mocks.MockStorageClient), zap.NewNop())

	_, err := h.UploadIdDocument(ctxWithFirebaseUID("firebaseXYZ"), &filepb.UploadIdDocumentRequest{
		DocumentType:   "Passport",
		Passport:       []byte("passport-bytes"),
		DocumentNumber: "PP-001",
		IssuedAt:       "2022-06-01T00:00:00Z",
		ExpireAt:       "2032-06-01T00:00:00Z",
		IssuingCountry: "TG",
	})

	require.NoError(t, err)
	require.NotNil(t, spy.gotInput)
	assert.Equal(t, "PP-001", spy.gotInput.DocumentNumber)
	assert.Equal(t, "2022-06-01T00:00:00Z", spy.gotInput.IssuedAt)
	assert.Equal(t, "2032-06-01T00:00:00Z", spy.gotInput.ExpireAt)
	assert.Equal(t, "TG", spy.gotInput.IssuingCountry)
	mockUser.AssertExpectations(t)
}
