package service_test

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/zap"

	"github.com/Kpeewu/tissi-mah/services/auth-service/fixtures"
	"github.com/Kpeewu/tissi-mah/services/auth-service/internal/domain"
	"github.com/Kpeewu/tissi-mah/services/auth-service/internal/middleware"
	"github.com/Kpeewu/tissi-mah/services/auth-service/internal/service"
	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/auth-service/internal/service/interfaces"
	authErrors "github.com/Kpeewu/tissi-mah/services/auth-service/pkg/errors"
	"github.com/Kpeewu/tissi-mah/services/auth-service/tests/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// --- Helpers ---

// ctxWithFirebaseID retourne un contexte contenant le FirebaseIDKey
func ctxWithFirebaseID(firebaseID string) context.Context {
	return context.WithValue(context.Background(), middleware.FirebaseIDKey, firebaseID)
}

// newTestService crée un triplet (readRepo, writeRepo, userClient) mockés
// et le service instancié à partir de ces mocks.
func newTestService() (*mocks.MockAuthRepositoryRead, *mocks.MockAuthRepositoryWrite, *mocks.MockUserClient, serviceInterfaces.AuthService) {
	mockReadRepo := new(mocks.MockAuthRepositoryRead)
	mockWriteRepo := new(mocks.MockAuthRepositoryWrite)
	mockUserClient := new(mocks.MockUserClient)
	svc := service.NewAuthService(mockReadRepo, mockWriteRepo, mockUserClient, nil, zap.NewNop())
	return mockReadRepo, mockWriteRepo, mockUserClient, svc
}

// =============================================================================
// GetUserByFirebaseID
// =============================================================================

func TestGetUserByFirebaseID(t *testing.T) {
	t.Run("succès - retourne l'auth depuis le repository", func(t *testing.T) {
		mockReadRepo, _, _, svc := newTestService()
		ctx := context.Background()

		auth := fixtures.NewTestAuth(fixtures.WithFirebaseID("firebase-123"))
		mockReadRepo.On("GetByFirebaseID", mock.Anything, "firebase-123").
			Return(auth, nil)

		result, err := svc.GetUserByFirebaseID(ctx, "firebase-123")

		require.NoError(t, err)
		assert.Equal(t, auth, result)
		assert.Equal(t, "firebase-123", result.FirebaseID)
		mockReadRepo.AssertExpectations(t)
	})

	t.Run("erreur - firebaseID vide retourne ErrorUserNotFound", func(t *testing.T) {
		_, _, _, svc := newTestService()
		ctx := context.Background()

		result, err := svc.GetUserByFirebaseID(ctx, "")

		assert.Nil(t, result)
		assert.ErrorIs(t, err, authErrors.ErrorUserNotFound)
	})

	t.Run("erreur - le repository remonte une erreur", func(t *testing.T) {
		mockReadRepo, _, _, svc := newTestService()
		ctx := context.Background()
		repoErr := errors.New("database connection lost")

		mockReadRepo.On("GetByFirebaseID", mock.Anything, "firebase-xyz").
			Return(nil, repoErr)

		result, err := svc.GetUserByFirebaseID(ctx, "firebase-xyz")

		assert.Nil(t, result)
		assert.ErrorIs(t, err, repoErr)
		mockReadRepo.AssertExpectations(t)
	})
}

// =============================================================================
// RegisterUser
// =============================================================================

func TestRegisterUser(t *testing.T) {
	t.Run("succès - crée le compte auth et le profil utilisateur", func(t *testing.T) {
		mockReadRepo, mockWriteRepo, mockUserClient, svc := newTestService()
		ctx := ctxWithFirebaseID("firebase-register-ok")

		email := "new@example.com"
		phone := "+22890000000"

		// Email et téléphone disponibles
		mockReadRepo.On("EmailExists", mock.Anything, email).
			Return(false, nil)
		mockReadRepo.On("PhoneNumberExists", mock.Anything, phone).
			Return(false, nil)

		// Création en base réussie — retourne l'authID
		mockWriteRepo.On("Create", mock.Anything, mock.MatchedBy(func(a *domain.Auth) bool {
			return a.FirebaseID == "firebase-register-ok" &&
				a.GetEmail() == email &&
				a.GetPhoneNumber() == phone &&
				a.IsActive
		})).Return("generated-auth-id", nil)

		// Appel inter-service vers user-service
		photoURL := "https://photo.url/pic.jpg"
		userPreview := &domain.UserPreview{
			AuthID:          "generated-auth-id",
			UserID:          "user-001",
			Name:            "Doe",
			FirstName:       "John",
			ProfilePhotoURL: &photoURL,
		}
		mockUserClient.On("CreateUser", mock.Anything, "generated-auth-id", "firebase-register-ok", "Doe", "John", photoURL).
			Return(userPreview, nil)

		result, err := svc.RegisterUser(ctx, "Doe", "John", email, phone, photoURL)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "generated-auth-id", result.AuthID)
		assert.Equal(t, "Doe", result.Name)
		assert.Equal(t, "John", result.FirstName)
		// Enrichissement avec email/phone provenant de auth
		require.NotNil(t, result.Email)
		assert.Equal(t, email, *result.Email)
		require.NotNil(t, result.PhoneNumber)
		assert.Equal(t, phone, *result.PhoneNumber)

		mockReadRepo.AssertExpectations(t)
		mockWriteRepo.AssertExpectations(t)
		mockUserClient.AssertExpectations(t)
	})

	t.Run("succès - sans email ni téléphone (champs optionnels)", func(t *testing.T) {
		_, mockWriteRepo, mockUserClient, svc := newTestService()
		ctx := ctxWithFirebaseID("firebase-no-contact")

		// Pas de vérification email/phone car vides
		mockWriteRepo.On("Create", mock.Anything, mock.MatchedBy(func(a *domain.Auth) bool {
			return a.FirebaseID == "firebase-no-contact" &&
				a.Email == nil &&
				a.PhoneNumber == nil
		})).Return("auth-no-contact", nil)

		userPreview := &domain.UserPreview{
			AuthID:    "auth-no-contact",
			UserID:    "user-002",
			Name:      "Koffi",
			FirstName: "Ama",
		}
		mockUserClient.On("CreateUser", mock.Anything, "auth-no-contact", "firebase-no-contact", "Koffi", "Ama", "").
			Return(userPreview, nil)

		result, err := svc.RegisterUser(ctx, "Koffi", "Ama", "", "", "")

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Nil(t, result.Email)
		assert.Nil(t, result.PhoneNumber)
		mockWriteRepo.AssertExpectations(t)
		mockUserClient.AssertExpectations(t)
	})

	t.Run("erreur - firebaseID absent du contexte retourne ErrorInternalServer", func(t *testing.T) {
		_, _, _, svc := newTestService()
		// Contexte sans FirebaseIDKey
		ctx := context.Background()

		result, err := svc.RegisterUser(ctx, "Doe", "John", "a@b.com", "+22800000000", "")

		assert.Nil(t, result)
		assert.ErrorIs(t, err, authErrors.ErrorInternalServer)
	})

	t.Run("erreur - firebaseID vide dans le contexte retourne ErrorInternalServer", func(t *testing.T) {
		_, _, _, svc := newTestService()
		ctx := ctxWithFirebaseID("")

		result, err := svc.RegisterUser(ctx, "Doe", "John", "a@b.com", "+22800000000", "")

		assert.Nil(t, result)
		assert.ErrorIs(t, err, authErrors.ErrorInternalServer)
	})

	t.Run("erreur - email déjà pris retourne ErrorEmailNotAvailable", func(t *testing.T) {
		mockReadRepo, _, _, svc := newTestService()
		ctx := ctxWithFirebaseID("firebase-dup-email")

		mockReadRepo.On("EmailExists", mock.Anything, "taken@example.com").
			Return(true, authErrors.ErrorEmailNotAvailable)

		result, err := svc.RegisterUser(ctx, "Doe", "John", "taken@example.com", "+22890000001", "")

		assert.Nil(t, result)
		assert.ErrorIs(t, err, authErrors.ErrorEmailNotAvailable)
		mockReadRepo.AssertExpectations(t)
	})

	t.Run("erreur - téléphone déjà pris retourne ErrorPhoneNumberNotAvailable", func(t *testing.T) {
		mockReadRepo, _, _, svc := newTestService()
		ctx := ctxWithFirebaseID("firebase-dup-phone")

		// Email disponible
		mockReadRepo.On("EmailExists", mock.Anything, "ok@example.com").
			Return(false, nil)
		// Téléphone pris
		mockReadRepo.On("PhoneNumberExists", mock.Anything, "+22811111111").
			Return(true, authErrors.ErrorPhoneNumberNotAvailable)

		result, err := svc.RegisterUser(ctx, "Doe", "John", "ok@example.com", "+22811111111", "")

		assert.Nil(t, result)
		assert.ErrorIs(t, err, authErrors.ErrorPhoneNumberNotAvailable)
		mockReadRepo.AssertExpectations(t)
	})

	t.Run("erreur - writeRepo.Create échoue et propage l'erreur", func(t *testing.T) {
		mockReadRepo, mockWriteRepo, _, svc := newTestService()
		ctx := ctxWithFirebaseID("firebase-create-fail")
		createErr := errors.New("write failure")

		mockReadRepo.On("EmailExists", mock.Anything, "e@f.com").
			Return(false, nil)
		mockReadRepo.On("PhoneNumberExists", mock.Anything, "+22822222222").
			Return(false, nil)
		mockWriteRepo.On("Create", mock.Anything, mock.Anything).
			Return("", createErr)

		result, err := svc.RegisterUser(ctx, "Doe", "John", "e@f.com", "+22822222222", "")

		assert.Nil(t, result)
		assert.ErrorIs(t, err, createErr)
		mockWriteRepo.AssertExpectations(t)
	})

	t.Run("erreur - userClient.CreateUser échoue, rollback auth record", func(t *testing.T) {
		mockReadRepo, mockWriteRepo, mockUserClient, svc := newTestService()
		ctx := ctxWithFirebaseID("firebase-user-client-fail")

		mockReadRepo.On("EmailExists", mock.Anything, "g@h.com").
			Return(false, nil)
		mockReadRepo.On("PhoneNumberExists", mock.Anything, "+22833333333").
			Return(false, nil)
		mockWriteRepo.On("Create", mock.Anything, mock.Anything).
			Return("auth-for-fail", nil)
		mockUserClient.On("CreateUser", mock.Anything, "auth-for-fail", "firebase-user-client-fail", "Doe", "John", "").
			Return(nil, errors.New("user-service unavailable"))
		// Compensation : writeRepo.Delete doit être appelé pour rollback
		mockWriteRepo.On("Delete", mock.Anything, mock.MatchedBy(func(a *domain.Auth) bool {
			return a.FirebaseID == "firebase-user-client-fail"
		})).Return(nil)

		result, err := svc.RegisterUser(ctx, "Doe", "John", "g@h.com", "+22833333333", "")

		assert.Nil(t, result)
		assert.ErrorIs(t, err, authErrors.ErrorInternalServer)
		mockUserClient.AssertExpectations(t)
		mockWriteRepo.AssertExpectations(t)
	})

	t.Run("erreur - userClient.CreateUser échoue et le rollback échoue aussi", func(t *testing.T) {
		mockReadRepo, mockWriteRepo, mockUserClient, svc := newTestService()
		ctx := ctxWithFirebaseID("firebase-double-fail")

		mockReadRepo.On("EmailExists", mock.Anything, "x@y.com").
			Return(false, nil)
		mockReadRepo.On("PhoneNumberExists", mock.Anything, "+22844444444").
			Return(false, nil)
		mockWriteRepo.On("Create", mock.Anything, mock.Anything).
			Return("auth-double-fail", nil)
		mockUserClient.On("CreateUser", mock.Anything, "auth-double-fail", "firebase-double-fail", "Doe", "John", "").
			Return(nil, errors.New("user-service unavailable"))
		// Compensation échoue aussi
		mockWriteRepo.On("Delete", mock.Anything, mock.Anything).
			Return(errors.New("delete also failed"))

		result, err := svc.RegisterUser(ctx, "Doe", "John", "x@y.com", "+22844444444", "")

		// L'erreur retournée reste ErrorInternalServer même si le rollback échoue
		assert.Nil(t, result)
		assert.ErrorIs(t, err, authErrors.ErrorInternalServer)
		mockWriteRepo.AssertExpectations(t)
	})

	t.Run("erreur - email au format invalide retourne ErrEmailInvalidFormat", func(t *testing.T) {
		_, _, _, svc := newTestService()
		ctx := ctxWithFirebaseID("firebase-bad-email")

		result, err := svc.RegisterUser(ctx, "Doe", "John", "not-an-email", "+22890000000", "")

		assert.Nil(t, result)
		assert.ErrorIs(t, err, domain.ErrEmailInvalidFormat)
	})

	t.Run("erreur - téléphone au format invalide retourne ErrPhoneInvalidFormat", func(t *testing.T) {
		_, _, _, svc := newTestService()
		ctx := ctxWithFirebaseID("firebase-bad-phone")

		result, err := svc.RegisterUser(ctx, "Doe", "John", "ok@example.com", "12345", "")

		assert.Nil(t, result)
		assert.ErrorIs(t, err, domain.ErrPhoneInvalidFormat)
	})

	t.Run("succès - normalise l'email en minuscules", func(t *testing.T) {
		mockReadRepo, mockWriteRepo, mockUserClient, svc := newTestService()
		ctx := ctxWithFirebaseID("firebase-normalize")

		// L'email normalisé est en minuscules
		mockReadRepo.On("EmailExists", mock.Anything, "upper@example.com").
			Return(false, nil)
		mockWriteRepo.On("Create", mock.Anything, mock.MatchedBy(func(a *domain.Auth) bool {
			return a.GetEmail() == "upper@example.com"
		})).Return("auth-normalize", nil)

		userPreview := &domain.UserPreview{
			AuthID: "auth-normalize", UserID: "user-norm",
			Name: "Doe", FirstName: "John",
		}
		mockUserClient.On("CreateUser", mock.Anything, "auth-normalize", "firebase-normalize", "Doe", "John", "").
			Return(userPreview, nil)

		result, err := svc.RegisterUser(ctx, "Doe", "John", "UPPER@Example.COM", "", "")

		require.NoError(t, err)
		require.NotNil(t, result)
		// L'email stocké doit être normalisé
		require.NotNil(t, result.Email)
		assert.Equal(t, "upper@example.com", *result.Email)
		mockReadRepo.AssertExpectations(t)
	})
}

// =============================================================================
// CheckEmail
// =============================================================================

func TestCheckEmail(t *testing.T) {
	t.Run("disponible - retourne (true, nil)", func(t *testing.T) {
		mockReadRepo, _, _, svc := newTestService()
		ctx := context.Background()

		mockReadRepo.On("EmailExists", mock.Anything, "free@example.com").
			Return(false, nil)

		available, err := svc.CheckEmail(ctx, "free@example.com")

		require.NoError(t, err)
		assert.True(t, available)
		mockReadRepo.AssertExpectations(t)
	})

	t.Run("indisponible - retourne (false, ErrorEmailNotAvailable)", func(t *testing.T) {
		mockReadRepo, _, _, svc := newTestService()
		ctx := context.Background()

		mockReadRepo.On("EmailExists", mock.Anything, "taken@example.com").
			Return(true, authErrors.ErrorEmailNotAvailable)

		available, err := svc.CheckEmail(ctx, "taken@example.com")

		assert.False(t, available)
		assert.ErrorIs(t, err, authErrors.ErrorEmailNotAvailable)
		mockReadRepo.AssertExpectations(t)
	})

	t.Run("erreur - email vide retourne (false, ErrorInternalServer)", func(t *testing.T) {
		_, _, _, svc := newTestService()
		ctx := context.Background()

		available, err := svc.CheckEmail(ctx, "")

		assert.False(t, available)
		assert.ErrorIs(t, err, authErrors.ErrorInternalServer)
	})

	t.Run("erreur - email au format invalide retourne ErrEmailInvalidFormat", func(t *testing.T) {
		_, _, _, svc := newTestService()
		ctx := context.Background()

		available, err := svc.CheckEmail(ctx, "not-an-email")

		assert.False(t, available)
		assert.ErrorIs(t, err, domain.ErrEmailInvalidFormat)
	})
}

// =============================================================================
// CheckPhoneNumber
// =============================================================================

func TestCheckPhoneNumber(t *testing.T) {
	t.Run("disponible - retourne (true, nil)", func(t *testing.T) {
		mockReadRepo, _, _, svc := newTestService()
		ctx := context.Background()

		mockReadRepo.On("PhoneNumberExists", mock.Anything, "+22890000099").
			Return(false, nil)

		available, err := svc.CheckPhoneNumber(ctx, "+22890000099")

		require.NoError(t, err)
		assert.True(t, available)
		mockReadRepo.AssertExpectations(t)
	})

	t.Run("indisponible - retourne (false, ErrorPhoneNumberNotAvailable)", func(t *testing.T) {
		mockReadRepo, _, _, svc := newTestService()
		ctx := context.Background()

		mockReadRepo.On("PhoneNumberExists", mock.Anything, "+22890000088").
			Return(true, authErrors.ErrorPhoneNumberNotAvailable)

		available, err := svc.CheckPhoneNumber(ctx, "+22890000088")

		assert.False(t, available)
		assert.ErrorIs(t, err, authErrors.ErrorPhoneNumberNotAvailable)
		mockReadRepo.AssertExpectations(t)
	})

	t.Run("erreur - numéro vide retourne (false, ErrorInternalServer)", func(t *testing.T) {
		_, _, _, svc := newTestService()
		ctx := context.Background()

		available, err := svc.CheckPhoneNumber(ctx, "")

		assert.False(t, available)
		assert.ErrorIs(t, err, authErrors.ErrorInternalServer)
	})

	t.Run("erreur - téléphone au format invalide retourne ErrPhoneInvalidFormat", func(t *testing.T) {
		_, _, _, svc := newTestService()
		ctx := context.Background()

		available, err := svc.CheckPhoneNumber(ctx, "12345")

		assert.False(t, available)
		assert.ErrorIs(t, err, domain.ErrPhoneInvalidFormat)
	})
}

// =============================================================================
// GetAuthInfo
// =============================================================================

func TestGetAuthInfo(t *testing.T) {
	t.Run("succès - retourne l'auth par authID", func(t *testing.T) {
		mockReadRepo, _, _, svc := newTestService()
		ctx := context.Background()

		auth := fixtures.NewTestAuth(fixtures.WithAuthID("auth-info-001"))
		mockReadRepo.On("GetByAuthID", mock.Anything, "auth-info-001").
			Return(auth, nil)

		result, err := svc.GetAuthInfo(ctx, "auth-info-001")

		require.NoError(t, err)
		assert.Equal(t, auth, result)
		assert.Equal(t, "auth-info-001", result.AuthID)
		mockReadRepo.AssertExpectations(t)
	})

	t.Run("erreur - authID vide retourne ErrorUserNotFound", func(t *testing.T) {
		_, _, _, svc := newTestService()
		ctx := context.Background()

		result, err := svc.GetAuthInfo(ctx, "")

		assert.Nil(t, result)
		assert.ErrorIs(t, err, authErrors.ErrorUserNotFound)
	})

	t.Run("erreur - le repository remonte une erreur", func(t *testing.T) {
		mockReadRepo, _, _, svc := newTestService()
		ctx := context.Background()
		repoErr := errors.New("unexpected db error")

		mockReadRepo.On("GetByAuthID", mock.Anything, "auth-unknown").
			Return(nil, repoErr)

		result, err := svc.GetAuthInfo(ctx, "auth-unknown")

		assert.Nil(t, result)
		assert.ErrorIs(t, err, repoErr)
		mockReadRepo.AssertExpectations(t)
	})
}

// =============================================================================
// DeleteUserAccount
// =============================================================================

func TestDeleteUserAccount(t *testing.T) {
	t.Run("succès - supprime le compte auth", func(t *testing.T) {
		mockReadRepo, mockWriteRepo, mockUserClient, svc := newTestService()
		ctx := context.Background()

		auth := fixtures.NewTestAuth(fixtures.WithFirebaseID("firebase-delete-ok"))
		mockReadRepo.On("GetByFirebaseID", mock.Anything, "firebase-delete-ok").
			Return(auth, nil)
		mockUserClient.On("SoftDeleteUser", mock.Anything, auth.AuthID).Return(nil)
		mockWriteRepo.On("Delete", mock.Anything, auth).
			Return(nil)

		err := svc.DeleteUserAccount(ctx, "firebase-delete-ok")

		require.NoError(t, err)
		mockReadRepo.AssertExpectations(t)
		mockWriteRepo.AssertExpectations(t)
	})

	t.Run("erreur - firebaseID vide retourne ErrorUserNotFound", func(t *testing.T) {
		_, _, _, svc := newTestService()
		ctx := context.Background()

		err := svc.DeleteUserAccount(ctx, "")

		assert.ErrorIs(t, err, authErrors.ErrorUserNotFound)
	})

	t.Run("erreur - utilisateur introuvable retourne ErrorUserNotFound", func(t *testing.T) {
		mockReadRepo, _, _, svc := newTestService()
		ctx := context.Background()

		mockReadRepo.On("GetByFirebaseID", mock.Anything, "firebase-ghost").
			Return(nil, authErrors.ErrorUserNotFound)

		err := svc.DeleteUserAccount(ctx, "firebase-ghost")

		assert.ErrorIs(t, err, authErrors.ErrorUserNotFound)
		mockReadRepo.AssertExpectations(t)
	})

	t.Run("succès - writeRepo.Delete échoue puis réussit au retry", func(t *testing.T) {
		mockReadRepo, mockWriteRepo, mockUserClient, svc := newTestService()
		ctx := context.Background()

		auth := fixtures.NewTestAuth(fixtures.WithFirebaseID("firebase-delete-retry"))
		mockReadRepo.On("GetByFirebaseID", mock.Anything, "firebase-delete-retry").
			Return(auth, nil)
		mockUserClient.On("SoftDeleteUser", mock.Anything, auth.AuthID).Return(nil)
		// Premier appel échoue, deuxième réussit
		mockWriteRepo.On("Delete", mock.Anything, auth).
			Return(errors.New("transient error")).Once()
		mockWriteRepo.On("Delete", mock.Anything, auth).
			Return(nil).Once()

		err := svc.DeleteUserAccount(ctx, "firebase-delete-retry")

		require.NoError(t, err)
		mockReadRepo.AssertExpectations(t)
		mockWriteRepo.AssertExpectations(t)
	})

	t.Run("erreur - writeRepo.Delete échoue deux fois retourne ErrorCantDeleteAccount", func(t *testing.T) {
		mockReadRepo, mockWriteRepo, mockUserClient, svc := newTestService()
		ctx := context.Background()

		auth := fixtures.NewTestAuth(fixtures.WithFirebaseID("firebase-delete-fail"))
		mockReadRepo.On("GetByFirebaseID", mock.Anything, "firebase-delete-fail").
			Return(auth, nil)
		mockUserClient.On("SoftDeleteUser", mock.Anything, auth.AuthID).Return(nil)
		// Les deux tentatives échouent
		mockWriteRepo.On("Delete", mock.Anything, auth).
			Return(errors.New("persistent error"))

		err := svc.DeleteUserAccount(ctx, "firebase-delete-fail")

		assert.ErrorIs(t, err, authErrors.ErrorCantDeleteAccount)
		mockReadRepo.AssertExpectations(t)
		mockWriteRepo.AssertExpectations(t)
	})
}
