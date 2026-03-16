package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/Kpeewu/tissi-mah/services/user-service/fixtures"
	"github.com/Kpeewu/tissi-mah/services/user-service/internal/client"
	"github.com/Kpeewu/tissi-mah/services/user-service/internal/domain"
	"github.com/Kpeewu/tissi-mah/services/user-service/internal/middleware"
	"github.com/Kpeewu/tissi-mah/services/user-service/internal/service"
	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/user-service/internal/service/interfaces"
	userErrors "github.com/Kpeewu/tissi-mah/services/user-service/pkg/errors"
	"github.com/Kpeewu/tissi-mah/services/user-service/tests/mocks"
)

// ---------- helpers ----------

// newService crée un service avec les mocks injectés et retourne les quatre mocks pour le setup.
func newService() (*mocks.MockUserRepositoryRead, *mocks.MockUserRepositoryWrite, *mocks.MockAuthClient, *mocks.MockFileClient, serviceInterfaces.UserService) {
	mockReadRepo := new(mocks.MockUserRepositoryRead)
	mockWriteRepo := new(mocks.MockUserRepositoryWrite)
	mockAuthClient := new(mocks.MockAuthClient)
	mockFileClient := new(mocks.MockFileClient)
	// Par défaut : file-service retourne "" (dégradation gracieuse)
	mockFileClient.On("GetDocumentExpiry", mock.Anything, mock.Anything, mock.Anything).Return("").Maybe()
	svc := service.NewUserService(mockReadRepo, mockWriteRepo, mockAuthClient, mockFileClient, zap.NewNop())
	return mockReadRepo, mockWriteRepo, mockAuthClient, mockFileClient, svc
}

// ctxWithFirebaseID retourne un contexte contenant le Firebase UID.
func ctxWithFirebaseID(firebaseID string) context.Context {
	return context.WithValue(context.Background(), middleware.FirebaseIDKey, firebaseID)
}

// defaultAuthInfo retourne un AuthInfo de test par défaut.
func defaultAuthInfo(authID string) *client.AuthInfo {
	return &client.AuthInfo{
		AuthID:            authID,
		Email:             "john@example.com",
		PhoneNumber:       "+221770000000",
		IsActive:          true,
		IsSuspended:       false,
		SuspensionEndDate: "",
	}
}

// stringPtr retourne un pointeur vers la chaîne donnée.
func stringPtr(s string) *string {
	return &s
}

// ========== CreateUser ==========

func TestCreateUser(t *testing.T) {
	t.Run("succes - cree un utilisateur avec photo de profil", func(t *testing.T) {
		mockReadRepo, mockWriteRepo, _, _, svc := newService()

		mockWriteRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.User")).
			Return("inserted-id", nil)

		user, err := svc.CreateUser(context.Background(), "auth-123", "firebase-123", "Doe", "John", "https://img.example.com/photo.jpg")

		require.NoError(t, err)
		require.NotNil(t, user)

		// Vérification des champs
		assert.Equal(t, "auth-123", user.AuthID)
		assert.Equal(t, "firebase-123", user.FirebaseID)
		assert.Equal(t, "Doe", user.Name)
		assert.Equal(t, "John", user.FirstName)
		assert.Equal(t, "https://img.example.com/photo.jpg", user.ProfileImageURL)
		assert.True(t, user.HasProfileImage, "HasProfileImage doit etre true quand profilePhotoURL est fourni")
		assert.True(t, user.IsPassenger, "IsPassenger doit etre true par defaut")
		assert.False(t, user.IsDriver, "IsDriver doit etre false par defaut")
		assert.NotEmpty(t, user.UserID, "UserID doit etre genere automatiquement")
		assert.False(t, user.CreatedAt.IsZero(), "CreatedAt doit etre defini")
		assert.False(t, user.UpdatedAt.IsZero(), "UpdatedAt doit etre defini")

		mockWriteRepo.AssertExpectations(t)
		mockReadRepo.AssertNotCalled(t, "GetByUserID")
	})

	t.Run("succes - cree un utilisateur sans photo de profil", func(t *testing.T) {
		_, mockWriteRepo, _, _, svc := newService()

		mockWriteRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.User")).
			Return("inserted-id", nil)

		user, err := svc.CreateUser(context.Background(), "auth-456", "firebase-456", "Diop", "Fatou", "")

		require.NoError(t, err)
		require.NotNil(t, user)

		assert.Equal(t, "", user.ProfileImageURL)
		assert.False(t, user.HasProfileImage, "HasProfileImage doit etre false quand profilePhotoURL est vide")
		assert.True(t, user.IsPassenger)

		mockWriteRepo.AssertExpectations(t)
	})

	t.Run("erreur - writeRepo.Create echoue", func(t *testing.T) {
		_, mockWriteRepo, _, _, svc := newService()

		repoErr := errors.New("mongo write error")
		mockWriteRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.User")).
			Return("", repoErr)

		user, err := svc.CreateUser(context.Background(), "auth-789", "firebase-789", "Ndiaye", "Ousmane", "")

		require.Error(t, err)
		assert.Nil(t, user)
		assert.Equal(t, repoErr, err)

		mockWriteRepo.AssertExpectations(t)
	})
}

// ========== GetUserByAuthID ==========

func TestGetUserByAuthID(t *testing.T) {
	t.Run("succes - retourne l utilisateur par AuthID", func(t *testing.T) {
		mockReadRepo, _, _, _, svc := newService()

		expectedUser := fixtures.NewTestUser(
			fixtures.WithAuthID("auth-123"),
			fixtures.WithName("Doe"),
			fixtures.WithFirstName("John"),
		)

		mockReadRepo.On("GetByAuthID", mock.Anything, "auth-123").
			Return(expectedUser, nil)

		user, err := svc.GetUserByAuthID(context.Background(), "auth-123")

		require.NoError(t, err)
		require.NotNil(t, user)
		assert.Equal(t, expectedUser, user)

		mockReadRepo.AssertExpectations(t)
	})

	t.Run("erreur - authID vide retourne ErrorUserNotFound", func(t *testing.T) {
		mockReadRepo, _, _, _, svc := newService()

		user, err := svc.GetUserByAuthID(context.Background(), "")

		require.Error(t, err)
		assert.Nil(t, user)
		assert.ErrorIs(t, err, userErrors.ErrorUserNotFound)

		mockReadRepo.AssertNotCalled(t, "GetByAuthID")
	})

	t.Run("erreur - repo retourne une erreur", func(t *testing.T) {
		mockReadRepo, _, _, _, svc := newService()

		repoErr := userErrors.ErrorDataRetrievalFailed
		mockReadRepo.On("GetByAuthID", mock.Anything, "auth-unknown").
			Return(nil, repoErr)

		user, err := svc.GetUserByAuthID(context.Background(), "auth-unknown")

		require.Error(t, err)
		assert.Nil(t, user)
		assert.ErrorIs(t, err, repoErr)

		mockReadRepo.AssertExpectations(t)
	})
}

// ========== GetMyProfile ==========

func TestGetMyProfile(t *testing.T) {
	t.Run("succes - retourne le profil complet enrichi avec auth info", func(t *testing.T) {
		mockReadRepo, _, mockAuthClient, _, svc := newService()

		testUser := fixtures.NewTestUser(
			fixtures.WithUserID("user-001"),
			fixtures.WithAuthID("auth-001"),
			fixtures.WithFirebaseID("firebase-001"),
			fixtures.WithName("Doe"),
			fixtures.WithFirstName("John"),
			fixtures.WithPassenger(),
			fixtures.WithProfileImage("https://img.example.com/photo.jpg"),
		)

		authInfo := defaultAuthInfo("auth-001")

		mockReadRepo.On("GetByFirebaseID", mock.Anything, "firebase-001").
			Return(testUser, nil)
		mockAuthClient.On("GetAuthInfo", mock.Anything, "auth-001").
			Return(authInfo, nil)

		ctx := ctxWithFirebaseID("firebase-001")
		profile, err := svc.GetMyProfile(ctx)

		require.NoError(t, err)
		require.NotNil(t, profile)

		// Verification des champs provenant du User
		assert.Equal(t, "auth-001", profile.AuthID)
		assert.Equal(t, "user-001", profile.UserID)
		assert.Equal(t, "Doe", profile.Name)
		assert.Equal(t, "John", profile.FirstName)
		assert.True(t, profile.HasProfileImage)
		assert.Equal(t, "https://img.example.com/photo.jpg", profile.ProfileImageURL)
		assert.True(t, profile.IsPassenger)
		// Verification des champs provenant de AuthInfo
		assert.Equal(t, "john@example.com", profile.Email)
		assert.Equal(t, "+221770000000", profile.PhoneNumber)
		assert.True(t, profile.IsActive)
		assert.False(t, profile.IsSuspended)

		mockReadRepo.AssertExpectations(t)
		mockAuthClient.AssertExpectations(t)
	})

	t.Run("erreur - firebaseID absent du contexte retourne ErrorInternalServer", func(t *testing.T) {
		mockReadRepo, _, mockAuthClient, _, svc := newService()

		// Contexte sans FirebaseIDKey
		profile, err := svc.GetMyProfile(context.Background())

		require.Error(t, err)
		assert.Nil(t, profile)
		assert.ErrorIs(t, err, userErrors.ErrorInternalServer)

		mockReadRepo.AssertNotCalled(t, "GetByFirebaseID")
		mockAuthClient.AssertNotCalled(t, "GetAuthInfo")
	})

	t.Run("erreur - firebaseID vide dans le contexte retourne ErrorInternalServer", func(t *testing.T) {
		mockReadRepo, _, mockAuthClient, _, svc := newService()

		ctx := ctxWithFirebaseID("")
		profile, err := svc.GetMyProfile(ctx)

		require.Error(t, err)
		assert.Nil(t, profile)
		assert.ErrorIs(t, err, userErrors.ErrorInternalServer)

		mockReadRepo.AssertNotCalled(t, "GetByFirebaseID")
		mockAuthClient.AssertNotCalled(t, "GetAuthInfo")
	})

	t.Run("erreur - utilisateur non trouve dans le repo", func(t *testing.T) {
		mockReadRepo, _, mockAuthClient, _, svc := newService()

		repoErr := userErrors.ErrorUserNotFound
		mockReadRepo.On("GetByFirebaseID", mock.Anything, "firebase-unknown").
			Return(nil, repoErr)

		ctx := ctxWithFirebaseID("firebase-unknown")
		profile, err := svc.GetMyProfile(ctx)

		require.Error(t, err)
		assert.Nil(t, profile)
		assert.ErrorIs(t, err, repoErr)

		mockReadRepo.AssertExpectations(t)
		mockAuthClient.AssertNotCalled(t, "GetAuthInfo")
	})

	t.Run("erreur - authClient.GetAuthInfo echoue retourne ErrorAuthServiceUnavailable", func(t *testing.T) {
		mockReadRepo, _, mockAuthClient, _, svc := newService()

		testUser := fixtures.NewTestUser(
			fixtures.WithAuthID("auth-002"),
			fixtures.WithFirebaseID("firebase-002"),
		)

		mockReadRepo.On("GetByFirebaseID", mock.Anything, "firebase-002").
			Return(testUser, nil)
		mockAuthClient.On("GetAuthInfo", mock.Anything, "auth-002").
			Return(nil, errors.New("grpc connection refused"))

		ctx := ctxWithFirebaseID("firebase-002")
		profile, err := svc.GetMyProfile(ctx)

		require.Error(t, err)
		assert.Nil(t, profile)
		assert.ErrorIs(t, err, userErrors.ErrorAuthServiceUnavailable)

		mockReadRepo.AssertExpectations(t)
		mockAuthClient.AssertExpectations(t)
	})
}

// ========== CreateDriverAccount ==========

func TestCreateDriverAccount(t *testing.T) {
	t.Run("succes - active le compte conducteur", func(t *testing.T) {
		mockReadRepo, mockWriteRepo, _, _, svc := newService()

		testUser := fixtures.NewTestUser(
			fixtures.WithUserID("user-driver-01"),
			fixtures.WithPassenger(),
		)

		mockReadRepo.On("GetByUserID", mock.Anything, "user-driver-01").
			Return(testUser, nil)
		mockWriteRepo.On("Update", mock.Anything, mock.Anything).
			Return(testUser, nil)

		err := svc.CreateDriverAccount(context.Background(), "user-driver-01", true)

		require.NoError(t, err)

		// Verification que EnableDriverAccount() a ete appele sur le user
		assert.True(t, testUser.IsDriver, "IsDriver doit etre true apres activation")

		mockReadRepo.AssertExpectations(t)
		mockWriteRepo.AssertExpectations(t)
	})

	t.Run("succes - desactive le compte conducteur", func(t *testing.T) {
		mockReadRepo, mockWriteRepo, _, _, svc := newService()

		testUser := fixtures.NewTestUser(
			fixtures.WithUserID("user-driver-02"),
			fixtures.WithDriver(),
			fixtures.WithPassenger(),
		)

		mockReadRepo.On("GetByUserID", mock.Anything, "user-driver-02").
			Return(testUser, nil)
		mockWriteRepo.On("Update", mock.Anything, mock.Anything).
			Return(testUser, nil)

		err := svc.CreateDriverAccount(context.Background(), "user-driver-02", false)

		require.NoError(t, err)

		// Verification que DisableDriverAccount() a ete appele sur le user
		assert.False(t, testUser.IsDriver, "IsDriver doit etre false apres desactivation")

		mockReadRepo.AssertExpectations(t)
		mockWriteRepo.AssertExpectations(t)
	})

	t.Run("erreur - profileID vide retourne ErrorInvalidUserID", func(t *testing.T) {
		mockReadRepo, mockWriteRepo, _, _, svc := newService()

		err := svc.CreateDriverAccount(context.Background(), "", true)

		require.Error(t, err)
		assert.ErrorIs(t, err, userErrors.ErrorInvalidUserID)

		mockReadRepo.AssertNotCalled(t, "GetByUserID")
		mockWriteRepo.AssertNotCalled(t, "Update")
	})

	t.Run("erreur - utilisateur non trouve", func(t *testing.T) {
		mockReadRepo, mockWriteRepo, _, _, svc := newService()

		repoErr := userErrors.ErrorUserNotFound
		mockReadRepo.On("GetByUserID", mock.Anything, "user-unknown").
			Return(nil, repoErr)

		err := svc.CreateDriverAccount(context.Background(), "user-unknown", true)

		require.Error(t, err)
		assert.ErrorIs(t, err, repoErr)

		mockReadRepo.AssertExpectations(t)
		mockWriteRepo.AssertNotCalled(t, "Update")
	})

	t.Run("erreur - writeRepo.Update echoue", func(t *testing.T) {
		mockReadRepo, mockWriteRepo, _, _, svc := newService()

		testUser := fixtures.NewTestUser(
			fixtures.WithUserID("user-driver-03"),
		)

		updateErr := errors.New("mongo update error")

		mockReadRepo.On("GetByUserID", mock.Anything, "user-driver-03").
			Return(testUser, nil)
		mockWriteRepo.On("Update", mock.Anything, mock.Anything).
			Return(nil, updateErr)

		err := svc.CreateDriverAccount(context.Background(), "user-driver-03", true)

		require.Error(t, err)
		assert.Equal(t, updateErr, err)

		mockReadRepo.AssertExpectations(t)
		mockWriteRepo.AssertExpectations(t)
	})
}

// ========== AddTripPreferences ==========

func TestAddTripPreferences(t *testing.T) {
	t.Run("succes - ajoute les preferences de trajet", func(t *testing.T) {
		mockReadRepo, mockWriteRepo, _, _, svc := newService()

		testUser := fixtures.NewTestUser(
			fixtures.WithUserID("user-prefs-01"),
		)

		preferences := []domain.TripPreference{
			{Preference: "music", IsAllowed: true},
			{Preference: "smoking", IsAllowed: false},
			{Preference: "pets", IsAllowed: true},
		}

		mockReadRepo.On("GetByUserID", mock.Anything, "user-prefs-01").
			Return(testUser, nil)
		mockWriteRepo.On("Update", mock.Anything, mock.Anything).
			Return(testUser, nil)

		err := svc.AddTripPreferences(context.Background(), "user-prefs-01", preferences)

		require.NoError(t, err)

		// Verification que SetTripPreferences() a ete appele
		require.Len(t, testUser.TripPreferences, 3)
		assert.Equal(t, "music", testUser.TripPreferences[0].Preference)
		assert.True(t, testUser.TripPreferences[0].IsAllowed)
		assert.Equal(t, "smoking", testUser.TripPreferences[1].Preference)
		assert.False(t, testUser.TripPreferences[1].IsAllowed)

		mockReadRepo.AssertExpectations(t)
		mockWriteRepo.AssertExpectations(t)
	})

	t.Run("erreur - profileID vide retourne ErrorInvalidUserID", func(t *testing.T) {
		mockReadRepo, mockWriteRepo, _, _, svc := newService()

		preferences := []domain.TripPreference{
			{Preference: "music", IsAllowed: true},
		}

		err := svc.AddTripPreferences(context.Background(), "", preferences)

		require.Error(t, err)
		assert.ErrorIs(t, err, userErrors.ErrorInvalidUserID)

		mockReadRepo.AssertNotCalled(t, "GetByUserID")
		mockWriteRepo.AssertNotCalled(t, "Update")
	})

	t.Run("erreur - utilisateur non trouve", func(t *testing.T) {
		mockReadRepo, mockWriteRepo, _, _, svc := newService()

		repoErr := userErrors.ErrorUserNotFound
		mockReadRepo.On("GetByUserID", mock.Anything, "user-unknown").
			Return(nil, repoErr)

		preferences := []domain.TripPreference{
			{Preference: "music", IsAllowed: true},
		}

		err := svc.AddTripPreferences(context.Background(), "user-unknown", preferences)

		require.Error(t, err)
		assert.ErrorIs(t, err, repoErr)

		mockReadRepo.AssertExpectations(t)
		mockWriteRepo.AssertNotCalled(t, "Update")
	})

	t.Run("erreur - writeRepo.Update echoue", func(t *testing.T) {
		mockReadRepo, mockWriteRepo, _, _, svc := newService()

		testUser := fixtures.NewTestUser(
			fixtures.WithUserID("user-prefs-02"),
		)

		updateErr := errors.New("mongo update error")

		mockReadRepo.On("GetByUserID", mock.Anything, "user-prefs-02").
			Return(testUser, nil)
		mockWriteRepo.On("Update", mock.Anything, mock.Anything).
			Return(nil, updateErr)

		preferences := []domain.TripPreference{
			{Preference: "music", IsAllowed: true},
		}

		err := svc.AddTripPreferences(context.Background(), "user-prefs-02", preferences)

		require.Error(t, err)
		assert.Equal(t, updateErr, err)

		mockReadRepo.AssertExpectations(t)
		mockWriteRepo.AssertExpectations(t)
	})
}

// ========== UpdateProfile ==========

func TestUpdateProfile(t *testing.T) {
	t.Run("succes - mise a jour de tous les champs", func(t *testing.T) {
		mockReadRepo, mockWriteRepo, mockAuthClient, _, svc := newService()

		testUser := fixtures.NewTestUser(
			fixtures.WithUserID("user-update-01"),
			fixtures.WithAuthID("auth-update-01"),
			fixtures.WithName("Doe"),
			fixtures.WithFirstName("John"),
			fixtures.WithNoProfileImage(),
		)

		authInfo := defaultAuthInfo("auth-update-01")

		mockReadRepo.On("GetByUserID", mock.Anything, "user-update-01").
			Return(testUser, nil)

		// Le writeRepo.Update recoit le user modifie en place et le retourne
		mockWriteRepo.On("Update", mock.Anything, mock.Anything).
			Return(testUser, nil)

		mockAuthClient.On("GetAuthInfo", mock.Anything, "auth-update-01").
			Return(authInfo, nil)

		req := serviceInterfaces.UpdateProfileRequest{
			UserID:            "user-update-01",
			FirstName:         stringPtr("Amadou"),
			LastName:          stringPtr("Diallo"),
			BirthDate:         stringPtr("1990-01-15"),
			ProfilePictureURL: stringPtr("https://img.example.com/new-photo.jpg"),
		}

		profile, err := svc.UpdateProfile(context.Background(), req)

		require.NoError(t, err)
		require.NotNil(t, profile)

		// Verification que les champs du user ont ete mis a jour
		assert.Equal(t, "Amadou", testUser.FirstName)
		assert.Equal(t, "Diallo", testUser.Name)
		assert.Equal(t, "1990-01-15", testUser.DateOfBirth)
		assert.Equal(t, "https://img.example.com/new-photo.jpg", testUser.ProfileImageURL)
		assert.True(t, testUser.HasProfileImage)

		// Verification du FullProfile retourne
		assert.Equal(t, "auth-update-01", profile.AuthID)
		assert.Equal(t, "user-update-01", profile.UserID)
		assert.Equal(t, "Amadou", profile.FirstName)
		assert.Equal(t, "Diallo", profile.Name)
		assert.Equal(t, "1990-01-15", profile.DateOfBirth)
		assert.Equal(t, "https://img.example.com/new-photo.jpg", profile.ProfileImageURL)
		assert.True(t, profile.HasProfileImage)

		// Champs auth enrichis
		assert.Equal(t, "john@example.com", profile.Email)
		assert.Equal(t, "+221770000000", profile.PhoneNumber)
		assert.True(t, profile.IsActive)

		mockReadRepo.AssertExpectations(t)
		mockWriteRepo.AssertExpectations(t)
		mockAuthClient.AssertExpectations(t)
	})

	t.Run("succes - mise a jour partielle (seulement FirstName)", func(t *testing.T) {
		mockReadRepo, mockWriteRepo, mockAuthClient, _, svc := newService()

		testUser := fixtures.NewTestUser(
			fixtures.WithUserID("user-update-02"),
			fixtures.WithAuthID("auth-update-02"),
			fixtures.WithName("Doe"),
			fixtures.WithFirstName("John"),
			fixtures.WithProfileImage("https://img.example.com/old-photo.jpg"),
		)

		// Sauvegarder les valeurs originales pour verification
		originalName := testUser.Name
		originalDOB := testUser.DateOfBirth
		originalProfileURL := testUser.ProfileImageURL

		authInfo := defaultAuthInfo("auth-update-02")

		mockReadRepo.On("GetByUserID", mock.Anything, "user-update-02").
			Return(testUser, nil)
		mockWriteRepo.On("Update", mock.Anything, mock.Anything).
			Return(testUser, nil)
		mockAuthClient.On("GetAuthInfo", mock.Anything, "auth-update-02").
			Return(authInfo, nil)

		req := serviceInterfaces.UpdateProfileRequest{
			UserID:    "user-update-02",
			FirstName: stringPtr("Mamadou"),
			// Pas de LastName, BirthDate, ProfilePictureURL
		}

		profile, err := svc.UpdateProfile(context.Background(), req)

		require.NoError(t, err)
		require.NotNil(t, profile)

		// Seul le FirstName a du changer
		assert.Equal(t, "Mamadou", testUser.FirstName)
		assert.Equal(t, "Mamadou", profile.FirstName)

		// Les autres champs doivent rester inchanges
		assert.Equal(t, originalName, testUser.Name)
		assert.Equal(t, originalDOB, testUser.DateOfBirth)
		assert.Equal(t, originalProfileURL, testUser.ProfileImageURL)

		mockReadRepo.AssertExpectations(t)
		mockWriteRepo.AssertExpectations(t)
		mockAuthClient.AssertExpectations(t)
	})

	t.Run("succes - ProfilePictureURL vide desactive HasProfileImage", func(t *testing.T) {
		mockReadRepo, mockWriteRepo, mockAuthClient, _, svc := newService()

		testUser := fixtures.NewTestUser(
			fixtures.WithUserID("user-update-03"),
			fixtures.WithAuthID("auth-update-03"),
			fixtures.WithProfileImage("https://img.example.com/photo.jpg"),
		)

		authInfo := defaultAuthInfo("auth-update-03")

		mockReadRepo.On("GetByUserID", mock.Anything, "user-update-03").
			Return(testUser, nil)
		mockWriteRepo.On("Update", mock.Anything, mock.Anything).
			Return(testUser, nil)
		mockAuthClient.On("GetAuthInfo", mock.Anything, "auth-update-03").
			Return(authInfo, nil)

		req := serviceInterfaces.UpdateProfileRequest{
			UserID:            "user-update-03",
			ProfilePictureURL: stringPtr(""),
		}

		profile, err := svc.UpdateProfile(context.Background(), req)

		require.NoError(t, err)
		require.NotNil(t, profile)

		assert.Equal(t, "", testUser.ProfileImageURL)
		assert.False(t, testUser.HasProfileImage, "HasProfileImage doit etre false quand ProfilePictureURL est vide")
		assert.False(t, profile.HasProfileImage)

		mockReadRepo.AssertExpectations(t)
		mockWriteRepo.AssertExpectations(t)
		mockAuthClient.AssertExpectations(t)
	})

	t.Run("erreur - profileID vide retourne ErrorInvalidUserID", func(t *testing.T) {
		mockReadRepo, mockWriteRepo, mockAuthClient, _, svc := newService()

		req := serviceInterfaces.UpdateProfileRequest{
			UserID:    "",
			FirstName: stringPtr("Amadou"),
		}

		profile, err := svc.UpdateProfile(context.Background(), req)

		require.Error(t, err)
		assert.Nil(t, profile)
		assert.ErrorIs(t, err, userErrors.ErrorInvalidUserID)

		mockReadRepo.AssertNotCalled(t, "GetByUserID")
		mockWriteRepo.AssertNotCalled(t, "Update")
		mockAuthClient.AssertNotCalled(t, "GetAuthInfo")
	})

	t.Run("erreur - utilisateur non trouve", func(t *testing.T) {
		mockReadRepo, mockWriteRepo, mockAuthClient, _, svc := newService()

		repoErr := userErrors.ErrorUserNotFound
		mockReadRepo.On("GetByUserID", mock.Anything, "user-unknown").
			Return(nil, repoErr)

		req := serviceInterfaces.UpdateProfileRequest{
			UserID:    "user-unknown",
			FirstName: stringPtr("Amadou"),
		}

		profile, err := svc.UpdateProfile(context.Background(), req)

		require.Error(t, err)
		assert.Nil(t, profile)
		assert.ErrorIs(t, err, repoErr)

		mockReadRepo.AssertExpectations(t)
		mockWriteRepo.AssertNotCalled(t, "Update")
		mockAuthClient.AssertNotCalled(t, "GetAuthInfo")
	})

	t.Run("erreur - writeRepo.Update echoue", func(t *testing.T) {
		mockReadRepo, mockWriteRepo, mockAuthClient, _, svc := newService()

		testUser := fixtures.NewTestUser(
			fixtures.WithUserID("user-update-04"),
			fixtures.WithAuthID("auth-update-04"),
		)

		updateErr := errors.New("mongo update error")

		mockReadRepo.On("GetByUserID", mock.Anything, "user-update-04").
			Return(testUser, nil)
		mockWriteRepo.On("Update", mock.Anything, mock.Anything).
			Return(nil, updateErr)

		req := serviceInterfaces.UpdateProfileRequest{
			UserID:    "user-update-04",
			FirstName: stringPtr("Amadou"),
		}

		profile, err := svc.UpdateProfile(context.Background(), req)

		require.Error(t, err)
		assert.Nil(t, profile)
		assert.Equal(t, updateErr, err)

		mockReadRepo.AssertExpectations(t)
		mockWriteRepo.AssertExpectations(t)
		mockAuthClient.AssertNotCalled(t, "GetAuthInfo")
	})

	t.Run("erreur - authClient.GetAuthInfo echoue retourne ErrorAuthServiceUnavailable", func(t *testing.T) {
		mockReadRepo, mockWriteRepo, mockAuthClient, _, svc := newService()

		testUser := fixtures.NewTestUser(
			fixtures.WithUserID("user-update-05"),
			fixtures.WithAuthID("auth-update-05"),
		)

		mockReadRepo.On("GetByUserID", mock.Anything, "user-update-05").
			Return(testUser, nil)
		mockWriteRepo.On("Update", mock.Anything, mock.Anything).
			Return(testUser, nil)
		mockAuthClient.On("GetAuthInfo", mock.Anything, "auth-update-05").
			Return(nil, errors.New("grpc connection refused"))

		req := serviceInterfaces.UpdateProfileRequest{
			UserID:    "user-update-05",
			FirstName: stringPtr("Amadou"),
		}

		profile, err := svc.UpdateProfile(context.Background(), req)

		require.Error(t, err)
		assert.Nil(t, profile)
		assert.ErrorIs(t, err, userErrors.ErrorAuthServiceUnavailable)

		mockReadRepo.AssertExpectations(t)
		mockWriteRepo.AssertExpectations(t)
		mockAuthClient.AssertExpectations(t)
	})
}

// ========== GetMyProfile avec dates d'expiration depuis file-service ==========

func TestGetMyProfile_WithDocumentExpiry(t *testing.T) {
	t.Run("succes - dates d expiration renseignees depuis file-service", func(t *testing.T) {
		mockReadRepo, _, mockAuthClient, mockFileClient, svc := newService()

		testUser := fixtures.NewTestUser(
			fixtures.WithUserID("user-expiry-01"),
			fixtures.WithAuthID("auth-expiry-01"),
			fixtures.WithFirebaseID("firebase-expiry-01"),
		)

		authInfo := defaultAuthInfo("auth-expiry-01")

		mockReadRepo.On("GetByFirebaseID", mock.Anything, "firebase-expiry-01").
			Return(testUser, nil)
		mockAuthClient.On("GetAuthInfo", mock.Anything, "auth-expiry-01").
			Return(authInfo, nil)

		// Remplacer le comportement par défaut pour les types spécifiques
		mockFileClient.ExpectedCalls = nil
		mockFileClient.On("GetDocumentExpiry", mock.Anything, "user-expiry-01", "idCardFront").
			Return("2030-12-31T00:00:00Z")
		mockFileClient.On("GetDocumentExpiry", mock.Anything, "user-expiry-01", "driverLicenceFront").
			Return("2028-06-15T00:00:00Z")

		ctx := ctxWithFirebaseID("firebase-expiry-01")
		profile, err := svc.GetMyProfile(ctx)

		require.NoError(t, err)
		require.NotNil(t, profile)

		assert.Equal(t, "2030-12-31T00:00:00Z", profile.IDCardExpirationDate)
		assert.Equal(t, "2028-06-15T00:00:00Z", profile.DriveLicenceExpirationDate)

		mockReadRepo.AssertExpectations(t)
		mockAuthClient.AssertExpectations(t)
		mockFileClient.AssertExpectations(t)
	})

	t.Run("succes - fallback sur idCardBack si idCardFront est vide", func(t *testing.T) {
		mockReadRepo, _, mockAuthClient, mockFileClient, svc := newService()

		testUser := fixtures.NewTestUser(
			fixtures.WithUserID("user-expiry-02"),
			fixtures.WithAuthID("auth-expiry-02"),
			fixtures.WithFirebaseID("firebase-expiry-02"),
		)

		authInfo := defaultAuthInfo("auth-expiry-02")

		mockReadRepo.On("GetByFirebaseID", mock.Anything, "firebase-expiry-02").
			Return(testUser, nil)
		mockAuthClient.On("GetAuthInfo", mock.Anything, "auth-expiry-02").
			Return(authInfo, nil)

		// Pas de document idCardFront → fallback sur idCardBack
		mockFileClient.ExpectedCalls = nil
		mockFileClient.On("GetDocumentExpiry", mock.Anything, "user-expiry-02", "idCardFront").Return("")
		mockFileClient.On("GetDocumentExpiry", mock.Anything, "user-expiry-02", "idCardBack").Return("2031-01-01T00:00:00Z")
		mockFileClient.On("GetDocumentExpiry", mock.Anything, "user-expiry-02", "driverLicenceFront").Return("")
		mockFileClient.On("GetDocumentExpiry", mock.Anything, "user-expiry-02", "driverLicenceBack").Return("")

		ctx := ctxWithFirebaseID("firebase-expiry-02")
		profile, err := svc.GetMyProfile(ctx)

		require.NoError(t, err)
		require.NotNil(t, profile)

		assert.Equal(t, "2031-01-01T00:00:00Z", profile.IDCardExpirationDate)
		assert.Equal(t, "", profile.DriveLicenceExpirationDate)

		mockReadRepo.AssertExpectations(t)
		mockAuthClient.AssertExpectations(t)
		mockFileClient.AssertExpectations(t)
	})
}

// ========== ChangeProfilePicture ==========

func TestChangeProfilePicture(t *testing.T) {
	t.Run("succes - photo uploadee et profil mis a jour", func(t *testing.T) {
		mockReadRepo, mockWriteRepo, mockAuthClient, mockFileClient, svc := newService()

		testUser := fixtures.NewTestUser(
			fixtures.WithUserID("user-photo-01"),
			fixtures.WithAuthID("auth-photo-01"),
			fixtures.WithNoProfileImage(),
		)
		authInfo := defaultAuthInfo("auth-photo-01")
		imageBytes := []byte{0xFF, 0xD8, 0xFF, 0xE0}
		imageURL := "https://s3.amazonaws.com/profile-photo-01.jpg"

		mockReadRepo.On("GetByUserID", mock.Anything, "user-photo-01").Return(testUser, nil)
		mockFileClient.On("UploadProfilePicture", mock.Anything, "user-photo-01", imageBytes).Return(imageURL, nil)
		mockWriteRepo.On("Update", mock.Anything, mock.Anything).Return(testUser, nil)
		mockAuthClient.On("GetAuthInfo", mock.Anything, "auth-photo-01").Return(authInfo, nil)
		mockFileClient.On("GetDocumentExpiry", mock.Anything, mock.Anything, mock.Anything).Return("").Maybe()

		profile, err := svc.ChangeProfilePicture(context.Background(), "user-photo-01", imageBytes)

		require.NoError(t, err)
		require.NotNil(t, profile)

		// Vérification que le user a été mis à jour en place
		assert.Equal(t, imageURL, testUser.ProfileImageURL)
		assert.True(t, testUser.HasProfileImage)

		// Vérification du profil enrichi retourné
		assert.Equal(t, "user-photo-01", profile.UserID)
		assert.Equal(t, "auth-photo-01", profile.AuthID)
		assert.Equal(t, imageURL, profile.ProfileImageURL)
		assert.True(t, profile.HasProfileImage)
		assert.Equal(t, "john@example.com", profile.Email)

		mockReadRepo.AssertExpectations(t)
		mockWriteRepo.AssertExpectations(t)
		mockAuthClient.AssertExpectations(t)
		mockFileClient.AssertExpectations(t)
	})

	t.Run("erreur - utilisateur non trouve", func(t *testing.T) {
		mockReadRepo, _, _, _, svc := newService()

		mockReadRepo.On("GetByUserID", mock.Anything, "user-inconnu").Return(nil, userErrors.ErrorUserNotFound)

		profile, err := svc.ChangeProfilePicture(context.Background(), "user-inconnu", []byte{0xFF})

		require.Error(t, err)
		assert.Nil(t, profile)
		assert.Equal(t, userErrors.ErrorUserNotFound, err)

		mockReadRepo.AssertExpectations(t)
	})

	t.Run("erreur - upload echoue retourne ErrorInternalServer", func(t *testing.T) {
		mockReadRepo, _, _, mockFileClient, svc := newService()

		testUser := fixtures.NewTestUser(
			fixtures.WithUserID("user-photo-02"),
			fixtures.WithAuthID("auth-photo-02"),
		)
		imageBytes := []byte{0xFF, 0xD8}

		mockReadRepo.On("GetByUserID", mock.Anything, "user-photo-02").Return(testUser, nil)
		mockFileClient.On("UploadProfilePicture", mock.Anything, "user-photo-02", imageBytes).Return("", errors.New("S3 unavailable"))

		profile, err := svc.ChangeProfilePicture(context.Background(), "user-photo-02", imageBytes)

		require.Error(t, err)
		assert.Nil(t, profile)
		assert.Equal(t, userErrors.ErrorInternalServer, err)

		mockReadRepo.AssertExpectations(t)
		mockFileClient.AssertExpectations(t)
	})
}
