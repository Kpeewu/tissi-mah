package e2e

import (
	"context"
	"testing"

	"github.com/Kpeewu/tissi-mah/services/user-service/internal/client"
	userpb "github.com/Kpeewu/tissi-mah/services/user-service/proto/gen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ============================================================================
// Health
// ============================================================================

func TestE2E_Health(t *testing.T) {
	resp, err := grpcClient.Health(context.Background(), &userpb.HealthRequest{})
	require.NoError(t, err)
	assert.Equal(t, "healthy", resp.Status)
	assert.NotEmpty(t, resp.Version)
	assert.Greater(t, resp.Timestamp, int64(0))
}

// ============================================================================
// CreateUser (inter-service)
// ============================================================================

func TestE2E_CreateUser(t *testing.T) {
	t.Run("succès", func(t *testing.T) {
		cleanCollection(t)

		resp, err := grpcClient.CreateUser(context.Background(), &userpb.CreateUserRequest{
			AuthID:          "auth-001",
			FirebaseID:      "firebase-001",
			Name:            "Doe",
			FirstName:       "John",
			ProfilePhotoURL: "https://example.com/photo.jpg",
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, "auth-001", resp.AuthID)
		assert.Equal(t, "Doe", resp.Name)
		assert.Equal(t, "John", resp.FirstName)
		assert.True(t, resp.HasProfileImage)
		assert.True(t, resp.IsPassenger)
		assert.False(t, resp.IsDriver)
		assert.NotEmpty(t, resp.UserID)
	})

	t.Run("erreur - profil déjà existant", func(t *testing.T) {
		cleanCollection(t)

		req := &userpb.CreateUserRequest{
			AuthID:     "auth-dup",
			FirebaseID: "firebase-dup",
			Name:       "Test",
			FirstName:  "User",
		}

		_, err := grpcClient.CreateUser(context.Background(), req)
		require.NoError(t, err)

		_, err = grpcClient.CreateUser(context.Background(), req)
		require.Error(t, err)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.AlreadyExists, st.Code())
	})
}

// ============================================================================
// GetUserByAuthID (inter-service)
// ============================================================================

func TestE2E_GetUserByAuthID(t *testing.T) {
	t.Run("succès", func(t *testing.T) {
		cleanCollection(t)

		_, err := grpcClient.CreateUser(context.Background(), &userpb.CreateUserRequest{
			AuthID:     "auth-002",
			FirebaseID: "firebase-002",
			Name:       "Dupont",
			FirstName:  "Marie",
		})
		require.NoError(t, err)

		resp, err := grpcClient.GetUserByAuthID(context.Background(), &userpb.GetUserByAuthIDRequest{
			AuthID: "auth-002",
		})

		require.NoError(t, err)
		assert.Equal(t, "auth-002", resp.AuthID)
		assert.Equal(t, "Dupont", resp.Name)
	})

	t.Run("erreur - utilisateur introuvable", func(t *testing.T) {
		cleanCollection(t)

		_, err := grpcClient.GetUserByAuthID(context.Background(), &userpb.GetUserByAuthIDRequest{
			AuthID: "auth-inexistant",
		})

		require.Error(t, err)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.NotFound, st.Code())
	})
}

// ============================================================================
// GetUserByUserID (inter-service)
// ============================================================================

func TestE2E_GetUserByUserID(t *testing.T) {
	t.Run("succès", func(t *testing.T) {
		cleanCollection(t)

		created, err := grpcClient.CreateUser(context.Background(), &userpb.CreateUserRequest{
			AuthID:     "auth-003",
			FirebaseID: "firebase-003",
			Name:       "Traoré",
			FirstName:  "Oumar",
		})
		require.NoError(t, err)

		mockAuthClient.On("GetAuthInfo", mock.Anything, created.AuthID).
			Return(&client.AuthInfo{
				AuthID: created.AuthID, Email: "oumar@example.com", PhoneNumber: "+22500000000",
				IsActive: true,
			}, nil).Once()

		resp, err := grpcClient.GetUserByUserID(context.Background(), &userpb.GetUserByUserIDRequest{
			UserID: created.UserID,
		})

		require.NoError(t, err)
		assert.Equal(t, created.UserID, resp.UserID)
		assert.Equal(t, "Traoré", resp.Name)
		assert.Equal(t, "oumar@example.com", resp.Email)
	})

	t.Run("erreur - utilisateur introuvable", func(t *testing.T) {
		cleanCollection(t)

		_, err := grpcClient.GetUserByUserID(context.Background(), &userpb.GetUserByUserIDRequest{
			UserID: "user-inexistant",
		})

		require.Error(t, err)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.NotFound, st.Code())
	})
}

// ============================================================================
// GetMyProfile (client-facing, requiert x-firebase-uid)
// ============================================================================

func TestE2E_GetMyProfile(t *testing.T) {
	t.Run("succès", func(t *testing.T) {
		cleanCollection(t)

		created, err := grpcClient.CreateUser(context.Background(), &userpb.CreateUserRequest{
			AuthID:     "auth-004",
			FirebaseID: "firebase-004",
			Name:       "Koné",
			FirstName:  "Fatoumata",
		})
		require.NoError(t, err)

		mockAuthClient.On("GetAuthInfo", mock.Anything, created.AuthID).
			Return(&client.AuthInfo{
				AuthID:      created.AuthID,
				Email:       "fatoumata@example.com",
				PhoneNumber: "+22590000000",
				IsActive:    true,
				IsSuspended: false,
			}, nil).Once()

		resp, err := grpcClient.GetMyProfile(ctxWithUID("firebase-004"), &userpb.GetMyProfileRequest{})

		require.NoError(t, err)
		require.NotNil(t, resp.User)
		assert.Equal(t, created.UserID, resp.User.UserID)
		assert.Equal(t, "auth-004", resp.User.AuthID)
		assert.Equal(t, "Koné", resp.User.Name)
		assert.Equal(t, "fatoumata@example.com", resp.User.Email)
		assert.Equal(t, "+22590000000", resp.User.PhoneNumber)
		assert.True(t, resp.User.IsActive)
		assert.True(t, resp.User.IsPassenger)
		mockAuthClient.AssertExpectations(t)
	})

	t.Run("erreur - Firebase UID absent", func(t *testing.T) {
		_, err := grpcClient.GetMyProfile(context.Background(), &userpb.GetMyProfileRequest{})

		require.Error(t, err)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.Unauthenticated, st.Code())
	})

	t.Run("erreur - utilisateur introuvable pour ce Firebase UID", func(t *testing.T) {
		cleanCollection(t)

		_, err := grpcClient.GetMyProfile(ctxWithUID("firebase-inconnu"), &userpb.GetMyProfileRequest{})

		require.Error(t, err)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.NotFound, st.Code())
	})
}

// ============================================================================
// CreateDriverAccount (client-facing)
// ============================================================================

func TestE2E_CreateDriverAccount(t *testing.T) {
	t.Run("succès - activer conducteur", func(t *testing.T) {
		cleanCollection(t)

		created, err := grpcClient.CreateUser(context.Background(), &userpb.CreateUserRequest{
			AuthID:     "auth-005",
			FirebaseID: "firebase-005",
			Name:       "Diallo",
			FirstName:  "Ibrahima",
		})
		require.NoError(t, err)

		resp, err := grpcClient.CreateDriverAccount(ctxWithUID("firebase-005"), &userpb.CreateDriverAccountRequest{
			UserID:              created.UserID,
			CreateDriverAccount: true,
		})

		require.NoError(t, err)
		assert.True(t, resp.Success)
	})

	t.Run("erreur - utilisateur introuvable", func(t *testing.T) {
		cleanCollection(t)

		_, err := grpcClient.CreateDriverAccount(ctxWithUID("firebase-005"), &userpb.CreateDriverAccountRequest{
			UserID:              "user-inexistant",
			CreateDriverAccount: true,
		})

		require.Error(t, err)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.NotFound, st.Code())
	})

	t.Run("erreur - Firebase UID absent", func(t *testing.T) {
		_, err := grpcClient.CreateDriverAccount(context.Background(), &userpb.CreateDriverAccountRequest{
			UserID:              "user-any",
			CreateDriverAccount: true,
		})

		require.Error(t, err)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.Unauthenticated, st.Code())
	})
}

// ============================================================================
// AddTripPreferences (client-facing)
// ============================================================================

func TestE2E_AddTripPreferences(t *testing.T) {
	t.Run("succès", func(t *testing.T) {
		cleanCollection(t)

		created, err := grpcClient.CreateUser(context.Background(), &userpb.CreateUserRequest{
			AuthID:     "auth-006",
			FirebaseID: "firebase-006",
			Name:       "Sow",
			FirstName:  "Aminata",
		})
		require.NoError(t, err)

		resp, err := grpcClient.AddTripPreferences(ctxWithUID("firebase-006"), &userpb.AddTripPreferencesRequest{
			UserID: created.UserID,
			Preferences: []*userpb.TripPreference{
				{Preference: "music", IsAllowed: true},
				{Preference: "smoking", IsAllowed: false},
			},
		})

		require.NoError(t, err)
		assert.True(t, resp.Success)
	})

	t.Run("erreur - Firebase UID absent", func(t *testing.T) {
		_, err := grpcClient.AddTripPreferences(context.Background(), &userpb.AddTripPreferencesRequest{
			UserID: "user-any",
		})

		require.Error(t, err)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.Unauthenticated, st.Code())
	})
}

// ============================================================================
// UpdateProfile (client-facing)
// ============================================================================

func TestE2E_UpdateProfile(t *testing.T) {
	strPtr := func(s string) *string { return &s }

	t.Run("succès - mise à jour partielle", func(t *testing.T) {
		cleanCollection(t)

		created, err := grpcClient.CreateUser(context.Background(), &userpb.CreateUserRequest{
			AuthID:     "auth-007",
			FirebaseID: "firebase-007",
			Name:       "Camara",
			FirstName:  "Seydou",
		})
		require.NoError(t, err)

		mockAuthClient.On("GetAuthInfo", mock.Anything, created.AuthID).
			Return(&client.AuthInfo{
				AuthID:      created.AuthID,
				Email:       "seydou@example.com",
				PhoneNumber: "+22491111111",
				IsActive:    true,
			}, nil).Once()

		resp, err := grpcClient.UpdateProfile(ctxWithUID("firebase-007"), &userpb.UpdateProfileRequest{
			UserID:    created.UserID,
			FirstName: strPtr("Seydouba"),
		})

		require.NoError(t, err)
		require.NotNil(t, resp.User)
		assert.Equal(t, "Seydouba", resp.User.FirstName)
		assert.Equal(t, "Camara", resp.User.Name)
		assert.Equal(t, "seydou@example.com", resp.User.Email)
		mockAuthClient.AssertExpectations(t)
	})

	t.Run("succès - mise à jour complète", func(t *testing.T) {
		cleanCollection(t)

		created, err := grpcClient.CreateUser(context.Background(), &userpb.CreateUserRequest{
			AuthID:     "auth-008",
			FirebaseID: "firebase-008",
			Name:       "Ba",
			FirstName:  "Mamadou",
		})
		require.NoError(t, err)

		mockAuthClient.On("GetAuthInfo", mock.Anything, created.AuthID).
			Return(&client.AuthInfo{
				AuthID:      created.AuthID,
				Email:       "mamadou.updated@example.com",
				PhoneNumber: "+22492222222",
				IsActive:    true,
			}, nil).Once()

		resp, err := grpcClient.UpdateProfile(ctxWithUID("firebase-008"), &userpb.UpdateProfileRequest{
			UserID:    created.UserID,
			FirstName: strPtr("Mamadou Lamine"),
			LastName:  strPtr("Bah"),
			BirthDate: strPtr("1995-03-20"),
		})

		require.NoError(t, err)
		require.NotNil(t, resp.User)
		assert.Equal(t, "Mamadou Lamine", resp.User.FirstName)
		assert.Equal(t, "Bah", resp.User.Name)
		assert.Equal(t, "1995-03-20", resp.User.DateOfBirth)
		mockAuthClient.AssertExpectations(t)
	})

	t.Run("erreur - utilisateur introuvable", func(t *testing.T) {
		cleanCollection(t)

		_, err := grpcClient.UpdateProfile(ctxWithUID("firebase-any"), &userpb.UpdateProfileRequest{
			UserID:    "user-inexistant",
			FirstName: strPtr("Test"),
		})

		require.Error(t, err)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.NotFound, st.Code())
	})

	t.Run("erreur - Firebase UID absent", func(t *testing.T) {
		_, err := grpcClient.UpdateProfile(context.Background(), &userpb.UpdateProfileRequest{
			UserID: "user-any",
		})

		require.Error(t, err)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.Unauthenticated, st.Code())
	})
}

// ============================================================================
// Scénario complet
// ============================================================================

func TestE2E_ScenarioComplet(t *testing.T) {
	// Simule le cycle de vie d'un utilisateur : création → activation conducteur →
	// ajout préférences → mise à jour profil → récupération profil complet
	cleanCollection(t)

	// 1. Création du profil
	created, err := grpcClient.CreateUser(context.Background(), &userpb.CreateUserRequest{
		AuthID:          "auth-scenario",
		FirebaseID:      "firebase-scenario",
		Name:            "Touré",
		FirstName:       "Abou",
		ProfilePhotoURL: "https://example.com/abou.jpg",
	})
	require.NoError(t, err)
	assert.True(t, created.IsPassenger)
	assert.False(t, created.IsDriver)
	userID := created.UserID

	// 2. Activation du compte conducteur
	driverResp, err := grpcClient.CreateDriverAccount(ctxWithUID("firebase-scenario"), &userpb.CreateDriverAccountRequest{
		UserID:              userID,
		CreateDriverAccount: true,
	})
	require.NoError(t, err)
	assert.True(t, driverResp.Success)

	// 3. Ajout de préférences de trajet
	prefsResp, err := grpcClient.AddTripPreferences(ctxWithUID("firebase-scenario"), &userpb.AddTripPreferencesRequest{
		UserID: userID,
		Preferences: []*userpb.TripPreference{
			{Preference: "music", IsAllowed: true},
			{Preference: "smoking", IsAllowed: false},
			{Preference: "pets", IsAllowed: true},
		},
	})
	require.NoError(t, err)
	assert.True(t, prefsResp.Success)

	// 4. Mise à jour du profil
	mockAuthClient.On("GetAuthInfo", mock.Anything, "auth-scenario").
		Return(&client.AuthInfo{
			AuthID:      "auth-scenario",
			Email:       "abou.toure@example.com",
			PhoneNumber: "+22593333333",
			IsActive:    true,
			IsSuspended: false,
		}, nil)

	updateResp, err := grpcClient.UpdateProfile(ctxWithUID("firebase-scenario"), &userpb.UpdateProfileRequest{
		UserID:    userID,
		FirstName: func() *string { s := "Aboubacar"; return &s }(),
	})
	require.NoError(t, err)
	assert.Equal(t, "Aboubacar", updateResp.User.FirstName)
	assert.Equal(t, "abou.toure@example.com", updateResp.User.Email)
	assert.True(t, updateResp.User.IsDriver)

	// 5. Récupération du profil complet
	profileResp, err := grpcClient.GetMyProfile(ctxWithUID("firebase-scenario"), &userpb.GetMyProfileRequest{})
	require.NoError(t, err)
	require.NotNil(t, profileResp.User)
	assert.Equal(t, userID, profileResp.User.UserID)
	assert.Equal(t, "Aboubacar", profileResp.User.FirstName)
	assert.Equal(t, "abou.toure@example.com", profileResp.User.Email)
	assert.True(t, profileResp.User.IsDriver)
	assert.True(t, profileResp.User.IsPassenger)
	assert.Len(t, profileResp.User.TripPreferences, 3)

	mockAuthClient.AssertExpectations(t)
}


