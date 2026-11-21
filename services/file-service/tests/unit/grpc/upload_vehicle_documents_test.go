package grpc_test

import (
	"context"
	"errors"
	"fmt"
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
	"google.golang.org/grpc/status"
)

func TestUploadVehicleDocuments_MissingUserID_ReturnsUnauthenticated(t *testing.T) {
	spy := &spyFileService{}
	mockUser := new(mocks.MockUserClient)
	h := grpcHandler.NewFileHandler(spy, mockUser, nil, new(mocks.MockStorageClient), zap.NewNop())

	_, err := h.UploadVehicleDocuments(context.Background(), &filepb.UploadVehicleDocumentsRequest{
		VehicleID: "vehicle-1",
	})

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, st.Code())
	mockUser.AssertNotCalled(t, "GetUserProfileByUserID")
	assert.Nil(t, spy.gotVehicleInput)
}

func TestUploadVehicleDocuments_EmptyUserID_ReturnsUnauthenticated(t *testing.T) {
	spy := &spyFileService{}
	mockUser := new(mocks.MockUserClient)
	h := grpcHandler.NewFileHandler(spy, mockUser, nil, new(mocks.MockStorageClient), zap.NewNop())

	_, err := h.UploadVehicleDocuments(ctxWithUserID(""), &filepb.UploadVehicleDocumentsRequest{
		VehicleID: "vehicle-1",
	})

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, st.Code())
	mockUser.AssertNotCalled(t, "GetUserProfileByUserID")
}

func TestUploadVehicleDocuments_UserServiceError_ReturnsErrorMessage(t *testing.T) {
	spy := &spyFileService{}
	mockUser := new(mocks.MockUserClient)
	mockUser.On("GetUserProfileByUserID", mock.Anything, "user-1").
		Return((*client.UserProfile)(nil), errors.New("user-service down"))
	h := grpcHandler.NewFileHandler(spy, mockUser, nil, new(mocks.MockStorageClient), zap.NewNop())

	resp, err := h.UploadVehicleDocuments(callerCtx(), &filepb.UploadVehicleDocumentsRequest{
		VehicleID: "vehicle-1",
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.False(t, resp.Success)
	assert.Equal(t, fileErrors.ErrorUserServiceUnavailable.Error(), resp.ErrorMessage)
	assert.Nil(t, spy.gotVehicleInput, "service ne doit pas etre appele si user-service echoue")
	mockUser.AssertExpectations(t)
}

func TestUploadVehicleDocuments_UsesInterceptorUserIDAndLoadsProfile(t *testing.T) {
	spy := &spyFileService{}
	mockUser := new(mocks.MockUserClient)
	mockUser.On("GetUserProfileByUserID", mock.Anything, "user-1").
		Return(&client.UserProfile{
			UserID:    "user-1",
			FirstName: "Jean",
			LastName:  "Dupont",
		}, nil)
	mockVehicle := ownedVehicle("vehicle-1", "user-1")
	h := grpcHandler.NewFileHandler(spy, mockUser, mockVehicle, new(mocks.MockStorageClient), zap.NewNop())

	resp, err := h.UploadVehicleDocuments(callerCtx(), &filepb.UploadVehicleDocumentsRequest{
		UserID:              "injected-body-id", // body-supplied, doit etre ignore
		VehicleID:           "vehicle-1",
		DriverLicenceRecto:  []byte("permis-recto"),
		DriverLicenceVerso:  []byte("permis-verso"),
		Assurance:           []byte("assurance"),
		VehicleRegistration: []byte("carte-grise"),
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.True(t, resp.Success)
	require.NotNil(t, spy.gotVehicleInput)
	assert.Equal(t, "user-1", spy.gotVehicleInput.UserID)
	assert.Equal(t, "Jean", spy.gotVehicleInput.FirstName)
	assert.Equal(t, "Dupont", spy.gotVehicleInput.LastName)
	assert.Equal(t, "vehicle-1", spy.gotVehicleInput.VehicleID)
	assert.Equal(t, []byte("permis-recto"), spy.gotVehicleInput.DriverLicence.Recto)
	assert.Equal(t, []byte("permis-verso"), spy.gotVehicleInput.DriverLicence.Verso)
	mockUser.AssertExpectations(t)
}

func TestUploadVehicleDocuments_IgnoresBodyUserID(t *testing.T) {
	spy := &spyFileService{}
	mockUser := new(mocks.MockUserClient)
	mockUser.On("GetUserProfileByUserID", mock.Anything, "legit-internal-id").
		Return(&client.UserProfile{
			UserID:    "legit-internal-id",
			FirstName: "Ama",
			LastName:  "Mensah",
		}, nil)
	mockVehicle := ownedVehicle("vehicle-2", "legit-internal-id")
	h := grpcHandler.NewFileHandler(spy, mockUser, mockVehicle, new(mocks.MockStorageClient), zap.NewNop())

	// Un attaquant forge un UserID dans le body mais possède son propre JWT légitime.
	resp, err := h.UploadVehicleDocuments(ctxWithUserID("legit-internal-id"), &filepb.UploadVehicleDocumentsRequest{
		UserID:              "forged-user-id-of-another-user",
		VehicleID:           "vehicle-2",
		DriverLicenceRecto:  []byte("x-recto"),
		DriverLicenceVerso:  []byte("x-verso"),
		Assurance:           []byte("y"),
		VehicleRegistration: []byte("z"),
	})

	require.NoError(t, err)
	require.True(t, resp.Success)
	require.NotNil(t, spy.gotVehicleInput)
	assert.Equal(t, "legit-internal-id", spy.gotVehicleInput.UserID)
	assert.NotEqual(t, "forged-user-id-of-another-user", spy.gotVehicleInput.UserID)
}

func TestUploadVehicleDocuments_ServiceError_ReturnsErrorMessage(t *testing.T) {
	spy := &spyFileService{uploadVehicleErr: fileErrors.ErrorUploadFailed}
	mockUser := new(mocks.MockUserClient)
	mockUser.On("GetUserProfileByUserID", mock.Anything, "user-1").
		Return(&client.UserProfile{UserID: "user-1", FirstName: "Jean", LastName: "Dupont"}, nil)
	mockVehicle := ownedVehicle("vehicle-1", "user-1")
	h := grpcHandler.NewFileHandler(spy, mockUser, mockVehicle, new(mocks.MockStorageClient), zap.NewNop())

	resp, err := h.UploadVehicleDocuments(callerCtx(), &filepb.UploadVehicleDocumentsRequest{
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
	mockUser.On("GetUserProfileByUserID", mock.Anything, "user-1").
		Return(&client.UserProfile{UserID: "user-1", FirstName: "Jean", LastName: "Dupont"}, nil)
	mockVehicle := ownedVehicle("vehicle-1", "user-1")
	h := grpcHandler.NewFileHandler(spy, mockUser, mockVehicle, new(mocks.MockStorageClient), zap.NewNop())

	resp, err := h.UploadVehicleDocuments(callerCtx(), &filepb.UploadVehicleDocumentsRequest{
		VehicleID:           "vehicle-1",
		DriverLicenceRecto:  []byte("permis-recto"),
		DriverLicenceVerso:  []byte("permis-verso"),
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
	assert.Equal(t, []byte("permis-recto"), dl.Recto)
	assert.Equal(t, []byte("permis-verso"), dl.Verso)
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

// ownedVehicle simule vehicle-service : vehicleID appartient à ownerID.
func ownedVehicle(vehicleID, ownerID string) *mocks.MockVehicleClient {
	m := new(mocks.MockVehicleClient)
	m.On("GetVehicleOwnerID", mock.Anything, vehicleID).Return(ownerID, nil)
	return m
}

func TestUploadVehicleDocuments_VehicleOwnershipRefused(t *testing.T) {
	cases := []struct {
		name        string
		ownerID     string
		ownerErr    error
		wantMessage string
	}{
		{"véhicule d'un autre utilisateur", "uuid-other", nil, fileErrors.ErrorVehicleNotFound.Error()},
		{"véhicule introuvable", "", fileErrors.ErrorVehicleNotFound, fileErrors.ErrorVehicleNotFound.Error()},
		{
			"vehicle-service indisponible", "",
			fmt.Errorf("%w: boom", fileErrors.ErrorVehicleServiceUnavailable),
			fileErrors.ErrorVehicleServiceUnavailable.Error(),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spy := &spyFileService{}
			mockUser := new(mocks.MockUserClient)
			mockUser.On("GetUserProfileByUserID", mock.Anything, "user-1").
				Return(&client.UserProfile{UserID: "user-1", FirstName: "Jean", LastName: "Dupont"}, nil)
			mockVehicle := new(mocks.MockVehicleClient)
			mockVehicle.On("GetVehicleOwnerID", mock.Anything, "vehicle-victim").Return(tc.ownerID, tc.ownerErr)
			h := grpcHandler.NewFileHandler(spy, mockUser, mockVehicle, new(mocks.MockStorageClient), zap.NewNop())

			resp, err := h.UploadVehicleDocuments(callerCtx(), &filepb.UploadVehicleDocumentsRequest{
				VehicleID:           "vehicle-victim",
				Assurance:           []byte("assurance"),
				VehicleRegistration: []byte("carte"),
			})

			require.NoError(t, err)
			require.NotNil(t, resp)
			assert.False(t, resp.Success)
			assert.Equal(t, tc.wantMessage, resp.ErrorMessage)
			assert.Nil(t, spy.gotVehicleInput, "aucun upload si le véhicule n'appartient pas à l'appelant")
			mockVehicle.AssertExpectations(t)
		})
	}
}

func TestUploadVehicleDocuments_NoVehicleClient_ReturnsUnavailable(t *testing.T) {
	spy := &spyFileService{}
	mockUser := new(mocks.MockUserClient)
	mockUser.On("GetUserProfileByUserID", mock.Anything, "user-1").
		Return(&client.UserProfile{UserID: "user-1"}, nil)
	h := grpcHandler.NewFileHandler(spy, mockUser, nil, new(mocks.MockStorageClient), zap.NewNop())

	resp, err := h.UploadVehicleDocuments(callerCtx(), &filepb.UploadVehicleDocumentsRequest{
		VehicleID: "vehicle-1",
	})

	require.NoError(t, err)
	assert.False(t, resp.Success)
	assert.Equal(t, fileErrors.ErrorVehicleServiceUnavailable.Error(), resp.ErrorMessage)
	assert.Nil(t, spy.gotVehicleInput)
}

func TestUploadVehicleDocuments_UserServiceError_DoesNotCallVehicleService(t *testing.T) {
	spy := &spyFileService{}
	mockUser := new(mocks.MockUserClient)
	mockUser.On("GetUserProfileByUserID", mock.Anything, "user-1").
		Return((*client.UserProfile)(nil), errors.New("user-service down"))
	mockVehicle := new(mocks.MockVehicleClient)
	h := grpcHandler.NewFileHandler(spy, mockUser, mockVehicle, new(mocks.MockStorageClient), zap.NewNop())

	resp, err := h.UploadVehicleDocuments(callerCtx(), &filepb.UploadVehicleDocumentsRequest{
		VehicleID: "vehicle-1",
	})

	require.NoError(t, err)
	assert.False(t, resp.Success)
	mockVehicle.AssertNotCalled(t, "GetVehicleOwnerID", mock.Anything, mock.Anything)
}
