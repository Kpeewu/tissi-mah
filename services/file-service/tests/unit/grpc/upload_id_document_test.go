package grpc_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Kpeewu/tissi-mah/services/file-service/internal/client"
	"github.com/Kpeewu/tissi-mah/services/file-service/internal/domain"
	grpcHandler "github.com/Kpeewu/tissi-mah/services/file-service/internal/grpc"
	"github.com/Kpeewu/tissi-mah/services/file-service/internal/middleware"
	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/file-service/internal/service/interfaces"
	fileErrors "github.com/Kpeewu/tissi-mah/services/file-service/pkg/errors"
	filepb "github.com/Kpeewu/tissi-mah/services/file-service/proto/gen"
	"github.com/Kpeewu/tissi-mah/services/file-service/tests/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
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
	gotSelfieInput   *serviceInterfaces.UploadSelfieInput
	uploadSelfieErr  error
	uploadSelfieRet  *serviceInterfaces.UploadedDocument
}

func (s *spyFileService) UploadIdDocument(_ context.Context, input serviceInterfaces.UploadIdDocumentInput) ([]*serviceInterfaces.UploadedDocument, error) {
	s.gotInput = &input
	return s.uploadIdReturn, s.uploadIdErr
}

func (s *spyFileService) UploadSelfie(_ context.Context, input serviceInterfaces.UploadSelfieInput) (*serviceInterfaces.UploadedDocument, error) {
	s.gotSelfieInput = &input
	return s.uploadSelfieRet, s.uploadSelfieErr
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

// ctxWithUserID simule un appelant authentifié : l'intercepteur a résolu son UserID interne.
func ctxWithUserID(userID string) context.Context {
	return context.WithValue(context.Background(), middleware.UserIDKey, userID)
}

// callerCtx simule un appelant authentifié avec UserID "user-1".
func callerCtx() context.Context {
	return ctxWithUserID("user-1")
}

func TestUploadIdDocument_MissingUserID_ReturnsUnauthenticated(t *testing.T) {
	spy := &spyFileService{}
	mockUser := new(mocks.MockUserClient)
	h := grpcHandler.NewFileHandler(spy, mockUser, nil, new(mocks.MockStorageClient), zap.NewNop())

	_, err := h.UploadIdDocument(context.Background(), &filepb.UploadIdDocumentRequest{
		DocumentType: "DriverLicence",
	})

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, st.Code())
	mockUser.AssertNotCalled(t, "GetUserProfileByUserID")
	assert.Nil(t, spy.gotInput)
}

func TestUploadIdDocument_EmptyUserID_ReturnsUnauthenticated(t *testing.T) {
	spy := &spyFileService{}
	mockUser := new(mocks.MockUserClient)
	h := grpcHandler.NewFileHandler(spy, mockUser, nil, new(mocks.MockStorageClient), zap.NewNop())

	ctx := ctxWithUserID("")
	_, err := h.UploadIdDocument(ctx, &filepb.UploadIdDocumentRequest{
		DocumentType: "DriverLicence",
	})

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, st.Code())
	mockUser.AssertNotCalled(t, "GetUserProfileByUserID")
	assert.Nil(t, spy.gotInput)
}

func TestUploadIdDocument_UserServiceError_ReturnsErrorMessage(t *testing.T) {
	spy := &spyFileService{}
	mockUser := new(mocks.MockUserClient)
	mockUser.On("GetUserProfileByUserID", mock.Anything, "user-1").
		Return((*client.UserProfile)(nil), errors.New("user-service down"))
	h := grpcHandler.NewFileHandler(spy, mockUser, nil, new(mocks.MockStorageClient), zap.NewNop())

	resp, err := h.UploadIdDocument(callerCtx(), &filepb.UploadIdDocumentRequest{
		DocumentType: "DriverLicence",
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.False(t, resp.Success)
	assert.Equal(t, fileErrors.ErrorUserServiceUnavailable.Error(), resp.ErrorMessage)
	assert.Nil(t, spy.gotInput, "service ne doit pas etre appele si user-service echoue")
	mockUser.AssertExpectations(t)
}

func TestUploadIdDocument_UsesInterceptorUserIDAndLoadsProfile(t *testing.T) {
	spy := &spyFileService{}
	mockUser := new(mocks.MockUserClient)
	mockUser.On("GetUserProfileByUserID", mock.Anything, "user-1").
		Return(&client.UserProfile{UserID: "user-1", FirstName: "Jean", LastName: "Dupont"}, nil)
	h := grpcHandler.NewFileHandler(spy, mockUser, nil, new(mocks.MockStorageClient), zap.NewNop())

	resp, err := h.UploadIdDocument(callerCtx(), &filepb.UploadIdDocumentRequest{
		UserID:             "injected-body-id", // body-supplied, doit etre ignore
		DocumentType:       "DriverLicence",
		DriverLicenceRecto: []byte("licence-recto-bytes"),
		DriverLicenceVerso: []byte("licence-verso-bytes"),
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.True(t, resp.Success)
	require.NotNil(t, spy.gotInput)
	assert.Equal(t, "user-1", spy.gotInput.UserID, "le service doit recevoir l'UserID interne du contexte")
	assert.NotEqual(t, "injected-body-id", spy.gotInput.UserID, "le champ req.UserID du body doit etre ignore")
	assert.Equal(t, "Jean", spy.gotInput.FirstName, "le service doit recevoir le prenom du profil user-service")
	assert.Equal(t, "Dupont", spy.gotInput.LastName, "le service doit recevoir le nom du profil user-service")
	assert.Equal(t, "DriverLicence", spy.gotInput.DocumentType)
	assert.Equal(t, []byte("licence-recto-bytes"), spy.gotInput.DriverLicenceRecto)
	assert.Equal(t, []byte("licence-verso-bytes"), spy.gotInput.DriverLicenceVerso)
	mockUser.AssertExpectations(t)
}

func TestUploadIdDocument_IgnoresBodyUserIDEvenWhenDifferentFromContext(t *testing.T) {
	spy := &spyFileService{}
	mockUser := new(mocks.MockUserClient)
	mockUser.On("GetUserProfileByUserID", mock.Anything, "legit-internal-id").
		Return(&client.UserProfile{UserID: "legit-internal-id", FirstName: "Anne", LastName: "Martin"}, nil)
	h := grpcHandler.NewFileHandler(spy, mockUser, nil, new(mocks.MockStorageClient), zap.NewNop())

	// Un attaquant met un UserID étranger dans le body mais possède son propre JWT légitime.
	// Le handler doit utiliser l'UserID du contexte (résolu par l'intercepteur) et ignorer le body.
	resp, err := h.UploadIdDocument(ctxWithUserID("legit-internal-id"), &filepb.UploadIdDocumentRequest{
		UserID:       "forged-user-id-of-another-user",
		DocumentType: "Passport",
		Passport:     []byte("passport-bytes"),
	})

	require.NoError(t, err)
	require.True(t, resp.Success)
	require.NotNil(t, spy.gotInput)
	assert.Equal(t, "legit-internal-id", spy.gotInput.UserID)
	mockUser.AssertExpectations(t)
}

func TestUploadIdDocument_ServiceError_ReturnsErrorMessage(t *testing.T) {
	spy := &spyFileService{uploadIdErr: fileErrors.ErrorUploadFailed}
	mockUser := new(mocks.MockUserClient)
	mockUser.On("GetUserProfileByUserID", mock.Anything, "user-1").
		Return(&client.UserProfile{UserID: "user-1", FirstName: "Jean", LastName: "Dupont"}, nil)
	h := grpcHandler.NewFileHandler(spy, mockUser, nil, new(mocks.MockStorageClient), zap.NewNop())

	resp, err := h.UploadIdDocument(callerCtx(), &filepb.UploadIdDocumentRequest{
		DocumentType: "DriverLicence",
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.False(t, resp.Success)
	assert.Equal(t, fileErrors.ErrorUploadFailed.Error(), resp.ErrorMessage)
	require.NotNil(t, spy.gotInput)
	assert.Equal(t, "user-1", spy.gotInput.UserID)
}

func TestUploadIdDocument_ForwardsMetadataFields(t *testing.T) {
	spy := &spyFileService{}
	mockUser := new(mocks.MockUserClient)
	mockUser.On("GetUserProfileByUserID", mock.Anything, "user-1").
		Return(&client.UserProfile{UserID: "user-1", FirstName: "Jean", LastName: "Dupont"}, nil)
	h := grpcHandler.NewFileHandler(spy, mockUser, nil, new(mocks.MockStorageClient), zap.NewNop())

	_, err := h.UploadIdDocument(callerCtx(), &filepb.UploadIdDocumentRequest{
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
