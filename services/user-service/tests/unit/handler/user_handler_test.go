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

	"github.com/Kpeewu/tissi-mah/services/user-service/internal/domain"
	grpcHandler "github.com/Kpeewu/tissi-mah/services/user-service/internal/grpc"
	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/user-service/internal/service/interfaces"
	userErrors "github.com/Kpeewu/tissi-mah/services/user-service/pkg/errors"
	userpb "github.com/Kpeewu/tissi-mah/services/user-service/proto/gen"
	"github.com/Kpeewu/tissi-mah/services/user-service/tests/mocks"
)

// --- Fixtures ---

// newMockAndHandler crée un mock service et un handler pour chaque test
func newMockAndHandler() (*mocks.MockUserService, *grpcHandler.UserHandler) {
	mockService := new(mocks.MockUserService)
	handler := grpcHandler.NewUserHandler(mockService)
	return mockService, handler
}

// newDomainUser retourne un domain.User de fixture pour les tests inter-service
func newDomainUser() *domain.User {
	return &domain.User{
		UserID:                     "user-789",
		AuthID:                     "auth-123",
		FirebaseID:                 "firebase-456",
		Name:                       "Doe",
		FirstName:                  "John",
		Gender:                     "male",
		DateOfBirth:                "1990-01-15",
		Bio:                        "Conducteur passionné",
		HasProfileImage:            true,
		ProfileImageURL:            "https://example.com/photo.jpg",
		IsDriver:                   false,
		IsPassenger:                true,
		IsDriverProfileVerified:    false,
		IsPassengerProfileVerified: true,
		TripPreferences: []domain.TripPreference{
			{Preference: "music", IsAllowed: true},
			{Preference: "smoking", IsAllowed: false},
		},
		IDCardExpirationDate:       "2030-12-31",
		DriveLicenceExpirationDate: "2028-06-15",
		CreatedAt:                  time.Now().UTC(),
		UpdatedAt:                  time.Now().UTC(),
	}
}

// newFullProfile retourne un FullProfile de fixture pour les tests client-facing
func newFullProfile() *serviceInterfaces.FullProfile {
	return &serviceInterfaces.FullProfile{
		AuthID:                     "auth-123",
		ProfileID:                  "profile-456",
		Name:                       "Doe",
		FirstName:                  "John",
		Gender:                     "male",
		DateOfBirth:                "1990-01-15",
		Bio:                        "Conducteur passionné",
		Email:                      "john@example.com",
		PhoneNumber:                "+22890000000",
		ProfileImageURL:            "https://example.com/photo.jpg",
		HasProfileImage:            true,
		IsDriver:                   false,
		IsPassenger:                true,
		IsDriverProfileVerified:    false,
		IsPassengerProfileVerified: true,
		IsActive:                   true,
		IsSuspended:                false,
		SuspensionEndDate:          "",
		TripPreferences: []domain.TripPreference{
			{Preference: "music", IsAllowed: true},
		},
		IDCardExpirationDate:       "2030-12-31",
		DriveLicenceExpirationDate: "2028-06-15",
		UserFiles: []domain.UserFile{
			{FileID: "file-1", FileURL: "https://example.com/id.jpg", FileType: "id_card"},
		},
	}
}

// stringPtr retourne un pointeur vers la chaîne passée en argument
func stringPtr(s string) *string {
	return &s
}

// assertGRPCCode vérifie le code gRPC d'une erreur
func assertGRPCCode(t *testing.T, err error, expectedCode codes.Code) {
	t.Helper()
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok, "l'erreur devrait être un statut gRPC")
	assert.Equal(t, expectedCode, st.Code())
}

// ============================================================================
// Tests CreateUser
// ============================================================================

func TestCreateUser_Success(t *testing.T) {
	// Vérifie que CreateUser retourne un UserProfileResponse correct en cas de succès
	mockService, handler := newMockAndHandler()
	ctx := context.Background()
	user := newDomainUser()

	mockService.On("CreateUser", ctx, "auth-123", "firebase-456", "Doe", "John", "https://example.com/photo.jpg").
		Return(user, nil)

	req := &userpb.CreateUserRequest{
		AuthID:          "auth-123",
		FirebaseID:      "firebase-456",
		Name:            "Doe",
		FirstName:       "John",
		ProfilePhotoURL: "https://example.com/photo.jpg",
	}

	resp, err := handler.CreateUser(ctx, req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, user.UserID, resp.UserID)
	assert.Equal(t, user.AuthID, resp.AuthID)
	assert.Equal(t, user.Name, resp.Name)
	assert.Equal(t, user.FirstName, resp.FirstName)
	assert.Equal(t, user.Gender, resp.Gender)
	assert.Equal(t, user.DateOfBirth, resp.DateOfBirth)
	assert.Equal(t, user.Bio, resp.Bio)
	assert.Equal(t, user.HasProfileImage, resp.HasProfileImage)
	assert.Equal(t, user.ProfileImageURL, resp.ProfileImageURL)
	assert.Equal(t, user.IsDriver, resp.IsDriver)
	assert.Equal(t, user.IsPassenger, resp.IsPassenger)
	assert.Equal(t, user.IsDriverProfileVerified, resp.IsDriverProfileVerified)
	assert.Equal(t, user.IsPassengerProfileVerified, resp.IsPassengerProfileVerified)
	assert.Equal(t, user.IDCardExpirationDate, resp.IDCardExpirationDate)
	assert.Equal(t, user.DriveLicenceExpirationDate, resp.DriveLicenceExpirationDate)
	require.Len(t, resp.TripPreferences, 2)
	assert.Equal(t, "music", resp.TripPreferences[0].Preference)
	assert.True(t, resp.TripPreferences[0].IsAllowed)
	assert.Equal(t, "smoking", resp.TripPreferences[1].Preference)
	assert.False(t, resp.TripPreferences[1].IsAllowed)
	mockService.AssertExpectations(t)
}

func TestCreateUser_ProfileAlreadyExists(t *testing.T) {
	// Vérifie que ErrorProfileAlreadyExists est traduit en codes.AlreadyExists
	mockService, handler := newMockAndHandler()
	ctx := context.Background()

	mockService.On("CreateUser", ctx, "auth-123", "firebase-456", "Doe", "John", "").
		Return(nil, userErrors.ErrorProfileAlreadyExists)

	req := &userpb.CreateUserRequest{
		AuthID:     "auth-123",
		FirebaseID: "firebase-456",
		Name:       "Doe",
		FirstName:  "John",
	}

	resp, err := handler.CreateUser(ctx, req)

	assert.Nil(t, resp)
	assertGRPCCode(t, err, codes.AlreadyExists)
	mockService.AssertExpectations(t)
}

func TestCreateUser_InternalError(t *testing.T) {
	// Vérifie qu'une erreur interne est traduite en codes.Internal
	mockService, handler := newMockAndHandler()
	ctx := context.Background()

	mockService.On("CreateUser", ctx, "auth-123", "firebase-456", "Doe", "John", "").
		Return(nil, userErrors.ErrorInternalServer)

	req := &userpb.CreateUserRequest{
		AuthID:     "auth-123",
		FirebaseID: "firebase-456",
		Name:       "Doe",
		FirstName:  "John",
	}

	resp, err := handler.CreateUser(ctx, req)

	assert.Nil(t, resp)
	assertGRPCCode(t, err, codes.Internal)
	mockService.AssertExpectations(t)
}

// ============================================================================
// Tests GetUserByAuthID
// ============================================================================

func TestGetUserByAuthID_Success(t *testing.T) {
	// Vérifie que GetUserByAuthID retourne un profil correct
	mockService, handler := newMockAndHandler()
	ctx := context.Background()
	user := newDomainUser()

	mockService.On("GetUserByAuthID", ctx, "auth-123").Return(user, nil)

	req := &userpb.GetUserByAuthIDRequest{AuthID: "auth-123"}
	resp, err := handler.GetUserByAuthID(ctx, req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, user.UserID, resp.UserID)
	assert.Equal(t, user.AuthID, resp.AuthID)
	assert.Equal(t, user.Name, resp.Name)
	assert.Equal(t, user.FirstName, resp.FirstName)
	assert.Equal(t, user.IsPassenger, resp.IsPassenger)
	mockService.AssertExpectations(t)
}

func TestGetUserByAuthID_NotFound(t *testing.T) {
	// Vérifie que ErrorUserNotFound est traduit en codes.NotFound
	mockService, handler := newMockAndHandler()
	ctx := context.Background()

	mockService.On("GetUserByAuthID", ctx, "auth-unknown").Return(nil, userErrors.ErrorUserNotFound)

	req := &userpb.GetUserByAuthIDRequest{AuthID: "auth-unknown"}
	resp, err := handler.GetUserByAuthID(ctx, req)

	assert.Nil(t, resp)
	assertGRPCCode(t, err, codes.NotFound)
	mockService.AssertExpectations(t)
}

func TestGetUserByAuthID_InternalError(t *testing.T) {
	// Vérifie qu'une erreur de récupération de données est traduite en codes.Internal
	mockService, handler := newMockAndHandler()
	ctx := context.Background()

	mockService.On("GetUserByAuthID", ctx, "auth-123").Return(nil, userErrors.ErrorDataRetrievalFailed)

	req := &userpb.GetUserByAuthIDRequest{AuthID: "auth-123"}
	resp, err := handler.GetUserByAuthID(ctx, req)

	assert.Nil(t, resp)
	assertGRPCCode(t, err, codes.Internal)
	mockService.AssertExpectations(t)
}

// ============================================================================
// Tests GetMyProfile
// ============================================================================

func TestGetMyProfile_Success(t *testing.T) {
	// Vérifie que GetMyProfile retourne un FullUserProfile complet
	mockService, handler := newMockAndHandler()
	ctx := context.Background()
	profile := newFullProfile()

	mockService.On("GetMyProfile", ctx).Return(profile, nil)

	req := &userpb.GetMyProfileRequest{}
	resp, err := handler.GetMyProfile(ctx, req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, resp.User)

	u := resp.User
	assert.Equal(t, profile.AuthID, u.AuthID)
	assert.Equal(t, profile.ProfileID, u.ProfileID)
	assert.Equal(t, profile.Name, u.Name)
	assert.Equal(t, profile.FirstName, u.FirstName)
	assert.Equal(t, profile.Gender, u.Gender)
	assert.Equal(t, profile.DateOfBirth, u.DateOfBirth)
	assert.Equal(t, profile.Bio, u.Bio)
	assert.Equal(t, profile.Email, u.Email)
	assert.Equal(t, profile.PhoneNumber, u.PhoneNumber)
	assert.Equal(t, profile.ProfileImageURL, u.ProfileImageURL)
	assert.Equal(t, profile.HasProfileImage, u.HasProfileImage)
	assert.Equal(t, profile.IsDriver, u.IsDriver)
	assert.Equal(t, profile.IsPassenger, u.IsPassenger)
	assert.Equal(t, profile.IsDriverProfileVerified, u.IsDriverProfileVerified)
	assert.Equal(t, profile.IsPassengerProfileVerified, u.IsPassengerProfileVerified)
	assert.Equal(t, profile.IsActive, u.IsActive)
	assert.Equal(t, profile.IsSuspended, u.IsSuspended)
	assert.Equal(t, profile.SuspensionEndDate, u.SuspensionEndDate)
	assert.Equal(t, profile.IDCardExpirationDate, u.IDCardExpirationDate)
	assert.Equal(t, profile.DriveLicenceExpirationDate, u.DriveLicenceExpirationDate)

	// Vérification des préférences de trajet
	require.Len(t, u.TripPreferences, 1)
	assert.Equal(t, "music", u.TripPreferences[0].Preference)
	assert.True(t, u.TripPreferences[0].IsAllowed)

	// Vérification des fichiers utilisateur
	require.Len(t, u.UserFiles, 1)
	assert.Equal(t, "file-1", u.UserFiles[0].FileID)
	assert.Equal(t, "https://example.com/id.jpg", u.UserFiles[0].FileURL)
	assert.Equal(t, "id_card", u.UserFiles[0].FileType)

	mockService.AssertExpectations(t)
}

func TestGetMyProfile_NotFound(t *testing.T) {
	// Vérifie que ErrorUserNotFound est traduit en codes.NotFound
	mockService, handler := newMockAndHandler()
	ctx := context.Background()

	mockService.On("GetMyProfile", ctx).Return(nil, userErrors.ErrorUserNotFound)

	req := &userpb.GetMyProfileRequest{}
	resp, err := handler.GetMyProfile(ctx, req)

	assert.Nil(t, resp)
	assertGRPCCode(t, err, codes.NotFound)
	mockService.AssertExpectations(t)
}

func TestGetMyProfile_AuthServiceUnavailable(t *testing.T) {
	// Vérifie que ErrorAuthServiceUnavailable est traduit en codes.Unavailable
	mockService, handler := newMockAndHandler()
	ctx := context.Background()

	mockService.On("GetMyProfile", ctx).Return(nil, userErrors.ErrorAuthServiceUnavailable)

	req := &userpb.GetMyProfileRequest{}
	resp, err := handler.GetMyProfile(ctx, req)

	assert.Nil(t, resp)
	assertGRPCCode(t, err, codes.Unavailable)
	mockService.AssertExpectations(t)
}

func TestGetMyProfile_InternalError(t *testing.T) {
	// Vérifie qu'une erreur interne est traduite en codes.Internal
	mockService, handler := newMockAndHandler()
	ctx := context.Background()

	mockService.On("GetMyProfile", ctx).Return(nil, userErrors.ErrorInternalServer)

	req := &userpb.GetMyProfileRequest{}
	resp, err := handler.GetMyProfile(ctx, req)

	assert.Nil(t, resp)
	assertGRPCCode(t, err, codes.Internal)
	mockService.AssertExpectations(t)
}

// ============================================================================
// Tests CreateDriverAccount
// ============================================================================

func TestCreateDriverAccount_Success(t *testing.T) {
	// Vérifie que CreateDriverAccount retourne Success=true en cas de succès
	mockService, handler := newMockAndHandler()
	ctx := context.Background()

	mockService.On("CreateDriverAccount", ctx, "profile-123", true).Return(nil)

	req := &userpb.CreateDriverAccountRequest{
		ProfileID:           "profile-123",
		CreateDriverAccount: true,
	}

	resp, err := handler.CreateDriverAccount(ctx, req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.True(t, resp.Success)
	mockService.AssertExpectations(t)
}

func TestCreateDriverAccount_InvalidProfileID(t *testing.T) {
	// Vérifie que ErrorInvalidProfileID est traduit en codes.InvalidArgument
	mockService, handler := newMockAndHandler()
	ctx := context.Background()

	mockService.On("CreateDriverAccount", ctx, "invalid-id", true).Return(userErrors.ErrorInvalidProfileID)

	req := &userpb.CreateDriverAccountRequest{
		ProfileID:           "invalid-id",
		CreateDriverAccount: true,
	}

	resp, err := handler.CreateDriverAccount(ctx, req)

	assert.Nil(t, resp)
	assertGRPCCode(t, err, codes.InvalidArgument)
	mockService.AssertExpectations(t)
}

func TestCreateDriverAccount_NotFound(t *testing.T) {
	// Vérifie que ErrorUserNotFound est traduit en codes.NotFound
	mockService, handler := newMockAndHandler()
	ctx := context.Background()

	mockService.On("CreateDriverAccount", ctx, "profile-unknown", true).Return(userErrors.ErrorUserNotFound)

	req := &userpb.CreateDriverAccountRequest{
		ProfileID:           "profile-unknown",
		CreateDriverAccount: true,
	}

	resp, err := handler.CreateDriverAccount(ctx, req)

	assert.Nil(t, resp)
	assertGRPCCode(t, err, codes.NotFound)
	mockService.AssertExpectations(t)
}

func TestCreateDriverAccount_InternalError(t *testing.T) {
	// Vérifie qu'une erreur interne est traduite en codes.Internal
	mockService, handler := newMockAndHandler()
	ctx := context.Background()

	mockService.On("CreateDriverAccount", ctx, "profile-123", false).Return(userErrors.ErrorInternalServer)

	req := &userpb.CreateDriverAccountRequest{
		ProfileID:           "profile-123",
		CreateDriverAccount: false,
	}

	resp, err := handler.CreateDriverAccount(ctx, req)

	assert.Nil(t, resp)
	assertGRPCCode(t, err, codes.Internal)
	mockService.AssertExpectations(t)
}

// ============================================================================
// Tests AddTripPreferences
// ============================================================================

func TestAddTripPreferences_Success(t *testing.T) {
	// Vérifie que AddTripPreferences retourne Success=true et convertit correctement les préférences
	mockService, handler := newMockAndHandler()
	ctx := context.Background()

	expectedPrefs := []domain.TripPreference{
		{Preference: "music", IsAllowed: true},
		{Preference: "smoking", IsAllowed: false},
		{Preference: "pets", IsAllowed: true},
	}

	mockService.On("AddTripPreferences", ctx, "profile-123", expectedPrefs).Return(nil)

	req := &userpb.AddTripPreferencesRequest{
		ProfileID: "profile-123",
		Preferences: []*userpb.TripPreference{
			{Preference: "music", IsAllowed: true},
			{Preference: "smoking", IsAllowed: false},
			{Preference: "pets", IsAllowed: true},
		},
	}

	resp, err := handler.AddTripPreferences(ctx, req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.True(t, resp.Success)
	mockService.AssertExpectations(t)
}

func TestAddTripPreferences_InvalidProfileID(t *testing.T) {
	// Vérifie que ErrorInvalidProfileID est traduit en codes.InvalidArgument
	mockService, handler := newMockAndHandler()
	ctx := context.Background()

	mockService.On("AddTripPreferences", ctx, "bad-id", mock.Anything).Return(userErrors.ErrorInvalidProfileID)

	req := &userpb.AddTripPreferencesRequest{
		ProfileID: "bad-id",
		Preferences: []*userpb.TripPreference{
			{Preference: "music", IsAllowed: true},
		},
	}

	resp, err := handler.AddTripPreferences(ctx, req)

	assert.Nil(t, resp)
	assertGRPCCode(t, err, codes.InvalidArgument)
	mockService.AssertExpectations(t)
}

func TestAddTripPreferences_InternalError(t *testing.T) {
	// Vérifie qu'une erreur interne est traduite en codes.Internal
	mockService, handler := newMockAndHandler()
	ctx := context.Background()

	mockService.On("AddTripPreferences", ctx, "profile-123", mock.Anything).Return(userErrors.ErrorInternalServer)

	req := &userpb.AddTripPreferencesRequest{
		ProfileID: "profile-123",
		Preferences: []*userpb.TripPreference{
			{Preference: "music", IsAllowed: true},
		},
	}

	resp, err := handler.AddTripPreferences(ctx, req)

	assert.Nil(t, resp)
	assertGRPCCode(t, err, codes.Internal)
	mockService.AssertExpectations(t)
}

func TestAddTripPreferences_EmptyList(t *testing.T) {
	// Vérifie que l'ajout d'une liste vide de préférences fonctionne
	mockService, handler := newMockAndHandler()
	ctx := context.Background()

	mockService.On("AddTripPreferences", ctx, "profile-123", []domain.TripPreference{}).Return(nil)

	req := &userpb.AddTripPreferencesRequest{
		ProfileID:   "profile-123",
		Preferences: []*userpb.TripPreference{},
	}

	resp, err := handler.AddTripPreferences(ctx, req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.True(t, resp.Success)
	mockService.AssertExpectations(t)
}

// ============================================================================
// Tests UpdateProfile
// ============================================================================

func TestUpdateProfile_SuccessAllFields(t *testing.T) {
	// Vérifie que UpdateProfile convertit tous les champs optionnels et retourne le profil complet
	mockService, handler := newMockAndHandler()
	ctx := context.Background()
	profile := newFullProfile()

	mockService.On("UpdateProfile", ctx, mock.MatchedBy(func(req serviceInterfaces.UpdateProfileRequest) bool {
		return req.ProfileID == "profile-456" &&
			req.FirstName != nil && *req.FirstName == "Jean" &&
			req.LastName != nil && *req.LastName == "Dupont" &&
			req.BirthDate != nil && *req.BirthDate == "1992-05-20" &&
			req.Email != nil && *req.Email == "jean@example.com" &&
			req.PhoneNumber != nil && *req.PhoneNumber == "+22891111111" &&
			req.ProfilePictureURL != nil && *req.ProfilePictureURL == "https://example.com/new.jpg"
	})).Return(profile, nil)

	req := &userpb.UpdateProfileRequest{
		ProfileID:         "profile-456",
		FirstName:         stringPtr("Jean"),
		LastName:          stringPtr("Dupont"),
		BirthDate:         stringPtr("1992-05-20"),
		Email:             stringPtr("jean@example.com"),
		PhoneNumber:       stringPtr("+22891111111"),
		ProfilePictureURL: stringPtr("https://example.com/new.jpg"),
	}

	resp, err := handler.UpdateProfile(ctx, req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, resp.User)
	assert.Equal(t, profile.AuthID, resp.User.AuthID)
	assert.Equal(t, profile.ProfileID, resp.User.ProfileID)
	assert.Equal(t, profile.Name, resp.User.Name)
	assert.Equal(t, profile.Email, resp.User.Email)
	assert.Equal(t, profile.PhoneNumber, resp.User.PhoneNumber)
	assert.Equal(t, profile.IsActive, resp.User.IsActive)
	mockService.AssertExpectations(t)
}

func TestUpdateProfile_SuccessPartialFields(t *testing.T) {
	// Vérifie que les champs non fournis restent nil dans la requête service
	mockService, handler := newMockAndHandler()
	ctx := context.Background()
	profile := newFullProfile()

	mockService.On("UpdateProfile", ctx, mock.MatchedBy(func(req serviceInterfaces.UpdateProfileRequest) bool {
		return req.ProfileID == "profile-456" &&
			req.FirstName != nil && *req.FirstName == "Marie" &&
			req.LastName == nil &&
			req.BirthDate == nil &&
			req.Email == nil &&
			req.PhoneNumber == nil &&
			req.ProfilePictureURL == nil
	})).Return(profile, nil)

	req := &userpb.UpdateProfileRequest{
		ProfileID: "profile-456",
		FirstName: stringPtr("Marie"),
	}

	resp, err := handler.UpdateProfile(ctx, req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, resp.User)
	assert.Equal(t, profile.ProfileID, resp.User.ProfileID)
	mockService.AssertExpectations(t)
}

func TestUpdateProfile_InvalidProfileID(t *testing.T) {
	// Vérifie que ErrorInvalidProfileID est traduit en codes.InvalidArgument
	mockService, handler := newMockAndHandler()
	ctx := context.Background()

	mockService.On("UpdateProfile", ctx, mock.MatchedBy(func(req serviceInterfaces.UpdateProfileRequest) bool {
		return req.ProfileID == "bad-id"
	})).Return(nil, userErrors.ErrorInvalidProfileID)

	req := &userpb.UpdateProfileRequest{
		ProfileID: "bad-id",
		FirstName: stringPtr("Test"),
	}

	resp, err := handler.UpdateProfile(ctx, req)

	assert.Nil(t, resp)
	assertGRPCCode(t, err, codes.InvalidArgument)
	mockService.AssertExpectations(t)
}

func TestUpdateProfile_InternalError(t *testing.T) {
	// Vérifie qu'une erreur interne est traduite en codes.Internal
	mockService, handler := newMockAndHandler()
	ctx := context.Background()

	mockService.On("UpdateProfile", ctx, mock.MatchedBy(func(req serviceInterfaces.UpdateProfileRequest) bool {
		return req.ProfileID == "profile-456"
	})).Return(nil, userErrors.ErrorInternalServer)

	req := &userpb.UpdateProfileRequest{
		ProfileID: "profile-456",
		Email:     stringPtr("test@example.com"),
	}

	resp, err := handler.UpdateProfile(ctx, req)

	assert.Nil(t, resp)
	assertGRPCCode(t, err, codes.Internal)
	mockService.AssertExpectations(t)
}

// ============================================================================
// Tests Health
// ============================================================================

func TestHealth_Success(t *testing.T) {
	// Vérifie que Health retourne toujours un statut sain
	_, handler := newMockAndHandler()
	ctx := context.Background()

	before := time.Now().Unix()
	resp, err := handler.Health(ctx, &userpb.HealthRequest{})
	after := time.Now().Unix()

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "healthy", resp.Status)
	assert.Equal(t, "1.0.0", resp.Version)
	assert.GreaterOrEqual(t, resp.Timestamp, before)
	assert.LessOrEqual(t, resp.Timestamp, after)
}

// ============================================================================
// Tests toGRPCError (indirectement via les méthodes du handler)
// ============================================================================

func TestToGRPCError_ErrorUserNotFound(t *testing.T) {
	// Vérifie le mapping ErrorUserNotFound → codes.NotFound
	mockService, handler := newMockAndHandler()
	ctx := context.Background()

	mockService.On("GetUserByAuthID", ctx, "auth-x").Return(nil, userErrors.ErrorUserNotFound)

	_, err := handler.GetUserByAuthID(ctx, &userpb.GetUserByAuthIDRequest{AuthID: "auth-x"})
	assertGRPCCode(t, err, codes.NotFound)
	mockService.AssertExpectations(t)
}

func TestToGRPCError_ErrorProfileAlreadyExists(t *testing.T) {
	// Vérifie le mapping ErrorProfileAlreadyExists → codes.AlreadyExists
	mockService, handler := newMockAndHandler()
	ctx := context.Background()

	mockService.On("CreateUser", ctx, "a", "f", "n", "fn", "").
		Return(nil, userErrors.ErrorProfileAlreadyExists)

	_, err := handler.CreateUser(ctx, &userpb.CreateUserRequest{
		AuthID: "a", FirebaseID: "f", Name: "n", FirstName: "fn",
	})
	assertGRPCCode(t, err, codes.AlreadyExists)
	mockService.AssertExpectations(t)
}

func TestToGRPCError_ErrorInvalidProfileID(t *testing.T) {
	// Vérifie le mapping ErrorInvalidProfileID → codes.InvalidArgument
	mockService, handler := newMockAndHandler()
	ctx := context.Background()

	mockService.On("CreateDriverAccount", ctx, "bad", true).Return(userErrors.ErrorInvalidProfileID)

	_, err := handler.CreateDriverAccount(ctx, &userpb.CreateDriverAccountRequest{
		ProfileID: "bad", CreateDriverAccount: true,
	})
	assertGRPCCode(t, err, codes.InvalidArgument)
	mockService.AssertExpectations(t)
}

func TestToGRPCError_ErrorAuthServiceUnavailable(t *testing.T) {
	// Vérifie le mapping ErrorAuthServiceUnavailable → codes.Unavailable
	mockService, handler := newMockAndHandler()
	ctx := context.Background()

	mockService.On("GetMyProfile", ctx).Return(nil, userErrors.ErrorAuthServiceUnavailable)

	_, err := handler.GetMyProfile(ctx, &userpb.GetMyProfileRequest{})
	assertGRPCCode(t, err, codes.Unavailable)
	mockService.AssertExpectations(t)
}

func TestToGRPCError_ErrorDataRetrievalFailed(t *testing.T) {
	// Vérifie le mapping ErrorDataRetrievalFailed → codes.Internal
	mockService, handler := newMockAndHandler()
	ctx := context.Background()

	mockService.On("GetMyProfile", ctx).Return(nil, userErrors.ErrorDataRetrievalFailed)

	_, err := handler.GetMyProfile(ctx, &userpb.GetMyProfileRequest{})
	assertGRPCCode(t, err, codes.Internal)
	mockService.AssertExpectations(t)
}

func TestToGRPCError_ErrorInternalServer(t *testing.T) {
	// Vérifie le mapping ErrorInternalServer → codes.Internal
	mockService, handler := newMockAndHandler()
	ctx := context.Background()

	mockService.On("GetMyProfile", ctx).Return(nil, userErrors.ErrorInternalServer)

	_, err := handler.GetMyProfile(ctx, &userpb.GetMyProfileRequest{})
	assertGRPCCode(t, err, codes.Internal)
	mockService.AssertExpectations(t)
}

func TestToGRPCError_UnknownError(t *testing.T) {
	// Vérifie qu'une erreur inconnue est traduite en codes.Internal avec message générique
	mockService, handler := newMockAndHandler()
	ctx := context.Background()

	unknownErr := errors.New("quelque chose d'inattendu")
	mockService.On("GetMyProfile", ctx).Return(nil, unknownErr)

	_, err := handler.GetMyProfile(ctx, &userpb.GetMyProfileRequest{})

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code())
	assert.Equal(t, "internal server error", st.Message())
	mockService.AssertExpectations(t)
}
