package grpc_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Kpeewu/tissi-mah/services/file-service/internal/client"
	grpcHandler "github.com/Kpeewu/tissi-mah/services/file-service/internal/grpc"
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

func TestUploadVehicleDocuments_MissingMetadata_ReturnsUnauthenticated(t *testing.T) {
	spy := &spyFileService{}
	mockUser := new(mocks.MockUserClient)
	h := grpcHandler.NewFileHandler(spy, mockUser, new(mocks.MockStorageClient), zap.NewNop())

	_, err := h.UploadVehicleDocuments(context.Background(), &filepb.UploadVehicleDocumentsRequest{
		UserID:    "firebaseXYZ",
		VehicleID: "vehicle-1",
	})

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, st.Code())
	mockUser.AssertNotCalled(t, "GetUserProfileByFirebaseID")
	assert.Nil(t, spy.gotVehicleInput)
}

func TestUploadVehicleDocuments_MissingFirebaseUID_ReturnsUnauthenticated(t *testing.T) {
	spy := &spyFileService{}
	mockUser := new(mocks.MockUserClient)
	h := grpcHandler.NewFileHandler(spy, mockUser, new(mocks.MockStorageClient), zap.NewNop())

	ctx := metadata.NewIncomingContext(context.Background(), metadata.New(map[string]string{}))
	_, err := h.UploadVehicleDocuments(ctx, &filepb.UploadVehicleDocumentsRequest{
		VehicleID: "vehicle-1",
	})

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, st.Code())
	mockUser.AssertNotCalled(t, "GetUserProfileByFirebaseID")
}

func TestUploadVehicleDocuments_UserServiceError_ReturnsErrorMessage(t *testing.T) {
	spy := &spyFileService{}
	mockUser := new(mocks.MockUserClient)
	mockUser.On("GetUserProfileByFirebaseID", mock.Anything, "firebaseXYZ").
		Return((*client.UserProfile)(nil), errors.New("user-service down"))
	h := grpcHandler.NewFileHandler(spy, mockUser, new(mocks.MockStorageClient), zap.NewNop())

	resp, err := h.UploadVehicleDocuments(ctxWithFirebaseUID("firebaseXYZ"), &filepb.UploadVehicleDocumentsRequest{
		VehicleID: "vehicle-1",
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.False(t, resp.Success)
	assert.Equal(t, fileErrors.ErrorUserServiceUnavailable.Error(), resp.ErrorMessage)
	assert.Nil(t, spy.gotVehicleInput, "service ne doit pas etre appele si user-service echoue")
	mockUser.AssertExpectations(t)
}

func TestUploadVehicleDocuments_ResolvesFirebaseAndForwardsProfile(t *testing.T) {
	spy := &spyFileService{}
	mockUser := new(mocks.MockUserClient)
	mockUser.On("GetUserProfileByFirebaseID", mock.Anything, "firebaseXYZ").
		Return(&client.UserProfile{
			UserID:    "uuid-abc",
			FirstName: "Jean",
			LastName:  "Dupont",
		}, nil)
	h := grpcHandler.NewFileHandler(spy, mockUser, new(mocks.MockStorageClient), zap.NewNop())

	resp, err := h.UploadVehicleDocuments(ctxWithFirebaseUID("firebaseXYZ"), &filepb.UploadVehicleDocumentsRequest{
		UserID:              "firebaseXYZ", // body-supplied, doit etre ignore
		VehicleID:           "vehicle-1",
		DriverLicenceImage:  []byte("permis"),
		Assurance:           []byte("assurance"),
		VehicleRegistration: []byte("carte-grise"),
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.True(t, resp.Success)
	require.NotNil(t, spy.gotVehicleInput)
	assert.Equal(t, "uuid-abc", spy.gotVehicleInput.UserID)
	assert.Equal(t, "Jean", spy.gotVehicleInput.FirstName)
	assert.Equal(t, "Dupont", spy.gotVehicleInput.LastName)
	assert.Equal(t, "vehicle-1", spy.gotVehicleInput.VehicleID)
	assert.Equal(t, []byte("permis"), spy.gotVehicleInput.DriverLicence.Data)
	mockUser.AssertExpectations(t)
}

func TestUploadVehicleDocuments_IgnoresBodyUserID(t *testing.T) {
	spy := &spyFileService{}
	mockUser := new(mocks.MockUserClient)
	mockUser.On("GetUserProfileByFirebaseID", mock.Anything, "legitFirebaseUID").
		Return(&client.UserProfile{
			UserID:    "internal-from-legit",
			FirstName: "Ama",
			LastName:  "Mensah",
		}, nil)
	h := grpcHandler.NewFileHandler(spy, mockUser, new(mocks.MockStorageClient), zap.NewNop())

	// Attaquant forge un UserID dans le body mais possede son propre JWT legitime.
	resp, err := h.UploadVehicleDocuments(ctxWithFirebaseUID("legitFirebaseUID"), &filepb.UploadVehicleDocumentsRequest{
		UserID:              "forgedFirebaseUIDOfAnotherUser",
		VehicleID:           "vehicle-2",
		DriverLicenceImage:  []byte("x"),
		Assurance:           []byte("y"),
		VehicleRegistration: []byte("z"),
	})

	require.NoError(t, err)
	require.True(t, resp.Success)
	require.NotNil(t, spy.gotVehicleInput)
	assert.Equal(t, "internal-from-legit", spy.gotVehicleInput.UserID)
	assert.NotEqual(t, "forgedFirebaseUIDOfAnotherUser", spy.gotVehicleInput.UserID)
}

func TestUploadVehicleDocuments_ServiceError_ReturnsErrorMessage(t *testing.T) {
	spy := &spyFileService{uploadVehicleErr: fileErrors.ErrorUploadFailed}
	mockUser := new(mocks.MockUserClient)
	mockUser.On("GetUserProfileByFirebaseID", mock.Anything, "firebaseXYZ").
		Return(&client.UserProfile{UserID: "uuid-abc", FirstName: "Jean", LastName: "Dupont"}, nil)
	h := grpcHandler.NewFileHandler(spy, mockUser, new(mocks.MockStorageClient), zap.NewNop())

	resp, err := h.UploadVehicleDocuments(ctxWithFirebaseUID("firebaseXYZ"), &filepb.UploadVehicleDocumentsRequest{
		VehicleID: "vehicle-1",
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.False(t, resp.Success)
	assert.Equal(t, fileErrors.ErrorUploadFailed.Error(), resp.ErrorMessage)
}

func TestUploadVehicleDocuments_ForwardsPerDocMetadata(t *testing.T) {
	spy := &spyFileService{}
	mockUser := new(mocks.MockUserClient)
	mockUser.On("GetUserProfileByFirebaseID", mock.Anything, "firebaseXYZ").
		Return(&client.UserProfile{UserID: "uuid-abc", FirstName: "Jean", LastName: "Dupont"}, nil)
	h := grpcHandler.NewFileHandler(spy, mockUser, new(mocks.MockStorageClient), zap.NewNop())

	resp, err := h.UploadVehicleDocuments(ctxWithFirebaseUID("firebaseXYZ"), &filepb.UploadVehicleDocumentsRequest{
		VehicleID:           "vehicle-1",
		DriverLicenceImage:  []byte("permis"),
		Assurance:           []byte("assurance"),
		VehicleRegistration: []byte("carte"),
		DriverLicenceMetadata: &filepb.VehicleDocMetadata{
			DocumentNumber:   "DL-001",
			IssuedAt:         "2022-01-01T00:00:00Z",
			ExpireAt:         "2026-01-01T00:00:00Z",
			IssuingAuthority: "DVLA",
		},
		AssuranceMetadata: &filepb.VehicleDocMetadata{
			DocumentNumber:   "INS-001",
			IssuedAt:         "2024-01-01T00:00:00Z",
			ExpireAt:         "2025-01-01T00:00:00Z",
			IssuingAuthority: "Assureur",
		},
		RegistrationCardMetadata: &filepb.VehicleDocMetadata{
			DocumentNumber:   "RC-001",
			IssuedAt:         "2023-01-01T00:00:00Z",
			ExpireAt:         "",
			IssuingAuthority: "Préfecture",
		},
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, spy.gotVehicleInput)

	dl := spy.gotVehicleInput.DriverLicence
	assert.Equal(t, []byte("permis"), dl.Data)
	assert.Equal(t, "DL-001", dl.DocumentNumber)
	assert.Equal(t, "2022-01-01T00:00:00Z", dl.IssuedAt)
	assert.Equal(t, "2026-01-01T00:00:00Z", dl.ExpireAt)
	assert.Equal(t, "DVLA", dl.IssuingAuthority)

	ass := spy.gotVehicleInput.Assurance
	assert.Equal(t, []byte("assurance"), ass.Data)
	assert.Equal(t, "INS-001", ass.DocumentNumber)

	rc := spy.gotVehicleInput.RegistrationCard
	assert.Equal(t, []byte("carte"), rc.Data)
	assert.Equal(t, "RC-001", rc.DocumentNumber)
	assert.Empty(t, rc.ExpireAt, "expire_at optionnel pour registrationCard")

	mockUser.AssertExpectations(t)
}
