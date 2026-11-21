package handler_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.uber.org/zap"

	"github.com/Kpeewu/tissi-mah/services/auth-service/internal/domain"
	grpcHandler "github.com/Kpeewu/tissi-mah/services/auth-service/internal/grpc"
	"github.com/Kpeewu/tissi-mah/services/auth-service/internal/middleware"
	authErrors "github.com/Kpeewu/tissi-mah/services/auth-service/pkg/errors"
	authpb "github.com/Kpeewu/tissi-mah/services/auth-service/proto/gen"
	"github.com/Kpeewu/tissi-mah/services/auth-service/tests/mocks"
)

// --------------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------------

// newHandler crée un handler avec un mock service frais.
func newHandler() (*grpcHandler.AuthHandler, *mocks.MockAuthService) {
	mockService := new(mocks.MockAuthService)
	handler := grpcHandler.NewAuthHandler(mockService, zap.NewNop())
	return handler, mockService
}

// ptrString retourne un pointeur vers la chaîne fournie.
func ptrString(s string) *string { return &s }

// ptrTime retourne un pointeur vers l'instant fourni.
func ptrTime(t time.Time) *time.Time { return &t }

// assertGRPCCode vérifie que l'erreur est un statut gRPC avec le code attendu.
func assertGRPCCode(t *testing.T, err error, expected codes.Code) {
	t.Helper()
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok, "l'erreur doit etre un statut gRPC")
	assert.Equal(t, expected, st.Code())
}

// --------------------------------------------------------------------------
// CreateAccount
// --------------------------------------------------------------------------

func TestCreateAccount_Success(t *testing.T) {
	// Arrange : creation reussie.
	handler, mockService := newHandler()
	ctx := context.Background()

	email := "new@example.com"
	phone := "+221780000000"
	photo := "https://img.example.com/new.jpg"

	preview := &domain.UserPreview{
		AuthID:          "auth-new",
		UserID:          "user-new",
		Name:            "Toure",
		FirstName:       "Moussa",
		Email:           &email,
		PhoneNumber:     &phone,
		ProfilePhotoURL: &photo,
	}

	mockService.On("RegisterUser", mock.Anything, "Toure", "Moussa", email, phone, photo, "").
		Return(preview, nil)

	// Act
	resp, err := handler.CreateAccount(ctx, &authpb.CreateAccountRequest{
		Name:            "Toure",
		FirstName:       "Moussa",
		Email:           email,
		PhoneNumber:     phone,
		ProfileImageURL: photo,
	})

	// Assert
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, resp.User)
	assert.Equal(t, "auth-new", resp.User.AuthID)
	assert.Equal(t, "user-new", resp.User.UserID)
	assert.Equal(t, "Toure", resp.User.Name)
	assert.Equal(t, "Moussa", resp.User.FirstName)
	assert.Equal(t, email, resp.User.Email)
	assert.Equal(t, phone, resp.User.PhoneNumber)
	assert.Equal(t, photo, resp.User.ProfileImageURL)

	mockService.AssertExpectations(t)
}

func TestCreateAccount_EmailNotAvailable(t *testing.T) {
	// Arrange : email deja pris.
	handler, mockService := newHandler()
	ctx := context.Background()

	mockService.On("RegisterUser", mock.Anything, "Toure", "Moussa", "taken@example.com", "+221780000000", "", "").
		Return(nil, authErrors.ErrorEmailNotAvailable)

	// Act
	resp, err := handler.CreateAccount(ctx, &authpb.CreateAccountRequest{
		Name:        "Toure",
		FirstName:   "Moussa",
		Email:       "taken@example.com",
		PhoneNumber: "+221780000000",
	})

	// Assert
	assert.Nil(t, resp)
	assertGRPCCode(t, err, codes.AlreadyExists)

	mockService.AssertExpectations(t)
}

func TestCreateAccount_PhoneNotAvailable(t *testing.T) {
	// Arrange : numero de telephone deja pris.
	handler, mockService := newHandler()
	ctx := context.Background()

	mockService.On("RegisterUser", mock.Anything, "Diop", "Awa", "awa@example.com", "+221770000001", "", "").
		Return(nil, authErrors.ErrorPhoneNumberNotAvailable)

	// Act
	resp, err := handler.CreateAccount(ctx, &authpb.CreateAccountRequest{
		Name:        "Diop",
		FirstName:   "Awa",
		Email:       "awa@example.com",
		PhoneNumber: "+221770000001",
	})

	// Assert
	assert.Nil(t, resp)
	assertGRPCCode(t, err, codes.AlreadyExists)

	mockService.AssertExpectations(t)
}

func TestCreateAccount_InternalError(t *testing.T) {
	// Arrange : erreur interne.
	handler, mockService := newHandler()
	ctx := context.Background()

	mockService.On("RegisterUser", mock.Anything, "X", "Y", "x@y.com", "+000", "", "").
		Return(nil, authErrors.ErrorInternalServer)

	// Act
	resp, err := handler.CreateAccount(ctx, &authpb.CreateAccountRequest{
		Name:        "X",
		FirstName:   "Y",
		Email:       "x@y.com",
		PhoneNumber: "+000",
	})

	// Assert
	assert.Nil(t, resp)
	assertGRPCCode(t, err, codes.Internal)

	mockService.AssertExpectations(t)
}

// --------------------------------------------------------------------------
// CheckEmail
// --------------------------------------------------------------------------

func TestCheckEmail_Available(t *testing.T) {
	// Arrange : email disponible.
	handler, mockService := newHandler()
	ctx := context.Background()

	mockService.On("CheckEmail", mock.Anything, "free@example.com").Return(true, nil)

	// Act
	resp, err := handler.CheckEmail(ctx, &authpb.CheckEmailRequest{Email: "free@example.com"})

	// Assert
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.True(t, resp.IsAvailable)

	mockService.AssertExpectations(t)
}

func TestCheckEmail_NotAvailable(t *testing.T) {
	// Arrange : email deja pris, le service retourne une erreur.
	handler, mockService := newHandler()
	ctx := context.Background()

	mockService.On("CheckEmail", mock.Anything, "taken@example.com").
		Return(false, authErrors.ErrorEmailNotAvailable)

	// Act
	resp, err := handler.CheckEmail(ctx, &authpb.CheckEmailRequest{Email: "taken@example.com"})

	// Assert
	assert.Nil(t, resp)
	assertGRPCCode(t, err, codes.AlreadyExists)

	mockService.AssertExpectations(t)
}

func TestCheckEmail_InternalError(t *testing.T) {
	// Arrange : erreur interne.
	handler, mockService := newHandler()
	ctx := context.Background()

	mockService.On("CheckEmail", mock.Anything, "fail@example.com").
		Return(false, authErrors.ErrorDataRetrievalFailed)

	// Act
	resp, err := handler.CheckEmail(ctx, &authpb.CheckEmailRequest{Email: "fail@example.com"})

	// Assert
	assert.Nil(t, resp)
	assertGRPCCode(t, err, codes.Internal)

	mockService.AssertExpectations(t)
}

// --------------------------------------------------------------------------
// CheckPhoneNumber
// --------------------------------------------------------------------------

func TestCheckPhoneNumber_Available(t *testing.T) {
	// Arrange : numero disponible.
	handler, mockService := newHandler()
	ctx := context.Background()

	mockService.On("CheckPhoneNumber", mock.Anything, "+221770000000").Return(true, nil)

	// Act
	resp, err := handler.CheckPhoneNumber(ctx, &authpb.CheckPhoneNumberRequest{PhoneNumber: "+221770000000"})

	// Assert
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.True(t, resp.IsAvailable)

	mockService.AssertExpectations(t)
}

func TestCheckPhoneNumber_NotAvailable(t *testing.T) {
	// Arrange : numero deja pris.
	handler, mockService := newHandler()
	ctx := context.Background()

	mockService.On("CheckPhoneNumber", mock.Anything, "+221770000001").
		Return(false, authErrors.ErrorPhoneNumberNotAvailable)

	// Act
	resp, err := handler.CheckPhoneNumber(ctx, &authpb.CheckPhoneNumberRequest{PhoneNumber: "+221770000001"})

	// Assert
	assert.Nil(t, resp)
	assertGRPCCode(t, err, codes.AlreadyExists)

	mockService.AssertExpectations(t)
}

func TestCheckPhoneNumber_InternalError(t *testing.T) {
	// Arrange : erreur interne.
	handler, mockService := newHandler()
	ctx := context.Background()

	mockService.On("CheckPhoneNumber", mock.Anything, "+000").
		Return(false, authErrors.ErrorInternalServer)

	// Act
	resp, err := handler.CheckPhoneNumber(ctx, &authpb.CheckPhoneNumberRequest{PhoneNumber: "+000"})

	// Assert
	assert.Nil(t, resp)
	assertGRPCCode(t, err, codes.Internal)

	mockService.AssertExpectations(t)
}

// --------------------------------------------------------------------------
// DeleteAccount
// --------------------------------------------------------------------------

func TestDeleteAccount_Success(t *testing.T) {
	// Arrange : suppression reussie avec Firebase UID dans le contexte.
	handler, mockService := newHandler()
	ctx := context.WithValue(context.Background(), middleware.FirebaseIDKey, "firebase-uid-123")

	mockService.On("DeleteUserAccount", mock.Anything, "firebase-uid-123").Return(nil)

	// Act
	resp, err := handler.DeleteAccount(ctx, &authpb.DeleteAccountRequest{})

	// Assert
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.True(t, resp.Success)

	mockService.AssertExpectations(t)
}

func TestDeleteAccount_MissingFirebaseID(t *testing.T) {
	// Arrange : aucun Firebase UID dans le contexte.
	handler, _ := newHandler()
	ctx := context.Background()

	// Act
	resp, err := handler.DeleteAccount(ctx, &authpb.DeleteAccountRequest{})

	// Assert
	assert.Nil(t, resp)
	assertGRPCCode(t, err, codes.Unauthenticated)
}

func TestDeleteAccount_EmptyFirebaseID(t *testing.T) {
	// Arrange : Firebase UID present mais vide.
	handler, _ := newHandler()
	ctx := context.WithValue(context.Background(), middleware.FirebaseIDKey, "")

	// Act
	resp, err := handler.DeleteAccount(ctx, &authpb.DeleteAccountRequest{})

	// Assert
	assert.Nil(t, resp)
	assertGRPCCode(t, err, codes.Unauthenticated)
}

func TestDeleteAccount_UserNotFound(t *testing.T) {
	// Arrange : l'utilisateur n'existe pas.
	handler, mockService := newHandler()
	ctx := context.WithValue(context.Background(), middleware.FirebaseIDKey, "firebase-uid-ghost")

	mockService.On("DeleteUserAccount", mock.Anything, "firebase-uid-ghost").
		Return(authErrors.ErrorUserNotFound)

	// Act
	resp, err := handler.DeleteAccount(ctx, &authpb.DeleteAccountRequest{})

	// Assert
	assert.Nil(t, resp)
	assertGRPCCode(t, err, codes.NotFound)

	mockService.AssertExpectations(t)
}

func TestDeleteAccount_CantDelete(t *testing.T) {
	// Arrange : suppression impossible (precondition non remplie).
	handler, mockService := newHandler()
	ctx := context.WithValue(context.Background(), middleware.FirebaseIDKey, "firebase-uid-locked")

	mockService.On("DeleteUserAccount", mock.Anything, "firebase-uid-locked").
		Return(authErrors.ErrorCantDeleteAccount)

	// Act
	resp, err := handler.DeleteAccount(ctx, &authpb.DeleteAccountRequest{})

	// Assert
	assert.Nil(t, resp)
	assertGRPCCode(t, err, codes.FailedPrecondition)

	mockService.AssertExpectations(t)
}

func TestDeleteAccount_InternalError(t *testing.T) {
	// Arrange : erreur interne.
	handler, mockService := newHandler()
	ctx := context.WithValue(context.Background(), middleware.FirebaseIDKey, "firebase-uid-err")

	mockService.On("DeleteUserAccount", mock.Anything, "firebase-uid-err").
		Return(authErrors.ErrorInternalServer)

	// Act
	resp, err := handler.DeleteAccount(ctx, &authpb.DeleteAccountRequest{})

	// Assert
	assert.Nil(t, resp)
	assertGRPCCode(t, err, codes.Internal)

	mockService.AssertExpectations(t)
}

// --------------------------------------------------------------------------
// Health
// --------------------------------------------------------------------------

func TestHealth_ReturnsHealthy(t *testing.T) {
	// Arrange : Health est statique, pas besoin de mock.
	handler, _ := newHandler()
	ctx := context.Background()
	before := time.Now().Unix()

	// Act
	resp, err := handler.Health(ctx, &authpb.HealthRequest{})

	// Assert
	after := time.Now().Unix()
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "healthy", resp.Status)
	assert.Equal(t, "1.0.0", resp.Version)
	assert.GreaterOrEqual(t, resp.Timestamp, before, "le timestamp doit etre >= avant l'appel")
	assert.LessOrEqual(t, resp.Timestamp, after, "le timestamp doit etre <= apres l'appel")
}

// --------------------------------------------------------------------------
// GetAuthInfo
// --------------------------------------------------------------------------

func TestGetAuthInfo_SuccessAllFields(t *testing.T) {
	// Arrange : toutes les donnees sont presentes (email, phone, suspension).
	handler, mockService := newHandler()
	ctx := context.Background()

	email := "auth@example.com"
	phone := "+221770000099"
	suspEnd := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)

	auth := &domain.Auth{
		AuthID:            "auth-info-1",
		Email:             &email,
		PhoneNumber:       &phone,
		IsActive:          true,
		IsSuspended:       true,
		SuspensionEndDate: &suspEnd,
	}

	mockService.On("GetAuthInfo", mock.Anything, "auth-info-1").Return(auth, nil)

	// Act
	resp, err := handler.GetAuthInfo(ctx, &authpb.GetAuthInfoRequest{AuthID: "auth-info-1"})

	// Assert
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "auth-info-1", resp.AuthID)
	assert.Equal(t, email, resp.Email)
	assert.Equal(t, phone, resp.PhoneNumber)
	assert.True(t, resp.IsActive)
	assert.True(t, resp.IsSuspended)
	assert.Equal(t, suspEnd.Format(time.RFC3339), resp.SuspensionEndDate)

	mockService.AssertExpectations(t)
}

func TestGetAuthInfo_SuccessNilOptionalFields(t *testing.T) {
	// Arrange : aucun email, telephone ni date de suspension.
	handler, mockService := newHandler()
	ctx := context.Background()

	auth := &domain.Auth{
		AuthID:      "auth-info-2",
		IsActive:    true,
		IsSuspended: false,
	}

	mockService.On("GetAuthInfo", mock.Anything, "auth-info-2").Return(auth, nil)

	// Act
	resp, err := handler.GetAuthInfo(ctx, &authpb.GetAuthInfoRequest{AuthID: "auth-info-2"})

	// Assert
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "auth-info-2", resp.AuthID)
	assert.Equal(t, "", resp.Email, "email nil doit donner une chaine vide")
	assert.Equal(t, "", resp.PhoneNumber, "phone nil doit donner une chaine vide")
	assert.True(t, resp.IsActive)
	assert.False(t, resp.IsSuspended)
	assert.Equal(t, "", resp.SuspensionEndDate, "pas de date de suspension")

	mockService.AssertExpectations(t)
}

func TestGetAuthInfo_UserNotFound(t *testing.T) {
	// Arrange : auth introuvable.
	handler, mockService := newHandler()
	ctx := context.Background()

	mockService.On("GetAuthInfo", mock.Anything, "auth-ghost").
		Return(nil, authErrors.ErrorUserNotFound)

	// Act
	resp, err := handler.GetAuthInfo(ctx, &authpb.GetAuthInfoRequest{AuthID: "auth-ghost"})

	// Assert
	assert.Nil(t, resp)
	assertGRPCCode(t, err, codes.NotFound)

	mockService.AssertExpectations(t)
}

func TestGetAuthInfo_InternalError(t *testing.T) {
	// Arrange : erreur interne.
	handler, mockService := newHandler()
	ctx := context.Background()

	mockService.On("GetAuthInfo", mock.Anything, "auth-fail").
		Return(nil, authErrors.ErrorDataRetrievalFailed)

	// Act
	resp, err := handler.GetAuthInfo(ctx, &authpb.GetAuthInfoRequest{AuthID: "auth-fail"})

	// Assert
	assert.Nil(t, resp)
	assertGRPCCode(t, err, codes.Internal)

	mockService.AssertExpectations(t)
}

// --------------------------------------------------------------------------
// toGRPCError (teste indirectement via les handlers)
// --------------------------------------------------------------------------

func TestToGRPCError_Mapping(t *testing.T) {
	// Verifie que chaque erreur domaine est traduite vers le bon code gRPC.
	// On utilise des methodes differentes pour couvrir tous les chemins de toGRPCError.

	tests := []struct {
		name         string
		setupMock    func(ms *mocks.MockAuthService)
		call         func(h *grpcHandler.AuthHandler) error
		expectedCode codes.Code
	}{
		{
			name: "ErrorUserNotFound → NotFound",
			setupMock: func(ms *mocks.MockAuthService) {
				ms.On("GetAuthInfo", mock.Anything, "id").Return(nil, authErrors.ErrorUserNotFound)
			},
			call: func(h *grpcHandler.AuthHandler) error {
				_, err := h.GetAuthInfo(context.Background(), &authpb.GetAuthInfoRequest{AuthID: "id"})
				return err
			},
			expectedCode: codes.NotFound,
		},
		{
			name: "ErrorEmailNotAvailable → AlreadyExists",
			setupMock: func(ms *mocks.MockAuthService) {
				ms.On("CheckEmail", mock.Anything, "e").Return(false, authErrors.ErrorEmailNotAvailable)
			},
			call: func(h *grpcHandler.AuthHandler) error {
				_, err := h.CheckEmail(context.Background(), &authpb.CheckEmailRequest{Email: "e"})
				return err
			},
			expectedCode: codes.AlreadyExists,
		},
		{
			name: "ErrorPhoneNumberNotAvailable → AlreadyExists",
			setupMock: func(ms *mocks.MockAuthService) {
				ms.On("CheckPhoneNumber", mock.Anything, "p").Return(false, authErrors.ErrorPhoneNumberNotAvailable)
			},
			call: func(h *grpcHandler.AuthHandler) error {
				_, err := h.CheckPhoneNumber(context.Background(), &authpb.CheckPhoneNumberRequest{PhoneNumber: "p"})
				return err
			},
			expectedCode: codes.AlreadyExists,
		},
		{
			name: "ErrorCantDeleteAccount → FailedPrecondition",
			setupMock: func(ms *mocks.MockAuthService) {
				ms.On("DeleteUserAccount", mock.Anything, "uid").Return(authErrors.ErrorCantDeleteAccount)
			},
			call: func(h *grpcHandler.AuthHandler) error {
				ctx := context.WithValue(context.Background(), middleware.FirebaseIDKey, "uid")
				_, err := h.DeleteAccount(ctx, &authpb.DeleteAccountRequest{})
				return err
			},
			expectedCode: codes.FailedPrecondition,
		},
		{
			name: "ErrorDataRetrievalFailed → Internal",
			setupMock: func(ms *mocks.MockAuthService) {
				ms.On("GetAuthInfo", mock.Anything, "id").Return(nil, authErrors.ErrorDataRetrievalFailed)
			},
			call: func(h *grpcHandler.AuthHandler) error {
				_, err := h.GetAuthInfo(context.Background(), &authpb.GetAuthInfoRequest{AuthID: "id"})
				return err
			},
			expectedCode: codes.Internal,
		},
		{
			name: "ErrorInternalServer → Internal",
			setupMock: func(ms *mocks.MockAuthService) {
				ms.On("GetAuthInfo", mock.Anything, "id").Return(nil, authErrors.ErrorInternalServer)
			},
			call: func(h *grpcHandler.AuthHandler) error {
				_, err := h.GetAuthInfo(context.Background(), &authpb.GetAuthInfoRequest{AuthID: "id"})
				return err
			},
			expectedCode: codes.Internal,
		},
		{
			name: "Erreur inconnue → Internal",
			setupMock: func(ms *mocks.MockAuthService) {
				ms.On("GetAuthInfo", mock.Anything, "id").Return(nil, errors.New("erreur inattendue"))
			},
			call: func(h *grpcHandler.AuthHandler) error {
				_, err := h.GetAuthInfo(context.Background(), &authpb.GetAuthInfoRequest{AuthID: "id"})
				return err
			},
			expectedCode: codes.Internal,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockService := new(mocks.MockAuthService)
			handler := grpcHandler.NewAuthHandler(mockService, zap.NewNop())

			tc.setupMock(mockService)

			err := tc.call(handler)

			assertGRPCCode(t, err, tc.expectedCode)
			mockService.AssertExpectations(t)
		})
	}
}

// TestToGRPCError_UnknownErrorMessage verifie que le message d'erreur generique
// est utilise pour les erreurs non-mappees (pas de fuite d'information).
func TestToGRPCError_UnknownErrorMessage(t *testing.T) {
	handler, mockService := newHandler()
	ctx := context.Background()

	mockService.On("GetAuthInfo", mock.Anything, "id").Return(nil, errors.New("secret database details"))

	// Act
	_, err := handler.GetAuthInfo(ctx, &authpb.GetAuthInfoRequest{AuthID: "id"})

	// Assert
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code())
	assert.Equal(t, "internal server error", st.Message(),
		"les erreurs inconnues ne doivent pas exposer de details internes")

	mockService.AssertExpectations(t)
}

// TestToGRPCError_KnownErrorPreservesMessage verifie que les erreurs connues
// conservent leur message d'erreur original dans le statut gRPC.
func TestToGRPCError_KnownErrorPreservesMessage(t *testing.T) {
	handler, mockService := newHandler()
	ctx := context.Background()

	mockService.On("CheckEmail", mock.Anything, "taken@test.com").
		Return(false, authErrors.ErrorEmailNotAvailable)

	// Act
	_, err := handler.CheckEmail(ctx, &authpb.CheckEmailRequest{Email: "taken@test.com"})

	// Assert
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.AlreadyExists, st.Code())
	assert.Equal(t, authErrors.ErrorEmailNotAvailable.Error(), st.Message(),
		"les erreurs connues doivent conserver leur message")

	mockService.AssertExpectations(t)
}
