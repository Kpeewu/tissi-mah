package integration

import (
	"context"
	"testing"

	"github.com/Kpeewu/tissi-mah/services/auth-service/fixtures"
	authErrors "github.com/Kpeewu/tissi-mah/services/auth-service/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthRepositoryRead_GetByFirebaseID(t *testing.T) {
	ctx := context.Background()
	repo := newTestAuthReadRepository()

	t.Run("should return user when firebase_id exists", func(t *testing.T) {
		// Arrange : Cleanup + insérer un utilisateur de test
		cleanupAuthTable(t, ctx)

		testAuth := fixtures.NewTestAuth()
		err := fixtures.InsertAuth(ctx, testPool, testAuth)
		require.NoError(t, err, "Failed to insert test user")

		// Act : Récupérer par FirebaseID
		result, err := repo.GetByFirebaseID(ctx, testAuth.FirebaseID)

		// Assert : Vérifier le résultat
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, testAuth.AuthID, result.AuthID)
		assert.Equal(t, testAuth.FirebaseID, result.FirebaseID)
		assert.Equal(t, testAuth.Email, result.Email)
		assert.Equal(t, testAuth.PhoneNumber, result.PhoneNumber)
		assert.Equal(t, testAuth.IsActive, result.IsActive)
		assert.Equal(t, testAuth.IsSuspended, result.IsSuspended)
	})

	t.Run("should return ErrorUserNotFound when firebase_id does not exist", func(t *testing.T) {
		// Arrange : Cleanup (table vide)
		cleanupAuthTable(t, ctx)

		// Act : Chercher un FirebaseID qui n'existe pas
		result, err := repo.GetByFirebaseID(ctx, "nonexistent_firebase_id")

		// Assert : Vérifier l'erreur
		assert.Error(t, err)
		assert.ErrorIs(t, err, authErrors.ErrorUserNotFound)
		assert.Nil(t, result)
	})
}

func TestAuthRepositoryRead_GetByAuthID(t *testing.T) {
	ctx := context.Background()
	repo := newTestAuthReadRepository()

	t.Run("should return user when auth_id exists", func(t *testing.T) {
		// Arrange
		cleanupAuthTable(t, ctx)
		testAuth := fixtures.NewTestAuth()
		err := fixtures.InsertAuth(ctx, testPool, testAuth)
		require.NoError(t, err, "Failed to insert test user")

		// Act
		result, err := repo.GetByAuthID(ctx, testAuth.AuthID)

		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, testAuth.AuthID, result.AuthID)
	})

	t.Run("should return ErrorUserNotFound when auth_id does not exist", func(t *testing.T) {
		// Arrange
		cleanupAuthTable(t, ctx)

		// Act
		result, err := repo.GetByAuthID(ctx, "nonexistent_auth_id")

		// Assert
		assert.Error(t, err)
		assert.ErrorIs(t, err, authErrors.ErrorUserNotFound)
		assert.Nil(t, result)
	})
}

func TestAuthRepositoryRead_GetByEmail(t *testing.T) {
	ctx := context.Background()
	repo := newTestAuthReadRepository()

	t.Run("should return user when email exists", func(t *testing.T) {
		// Arrange
		cleanupAuthTable(t, ctx)
		testAuth := fixtures.NewTestAuth()
		err := fixtures.InsertAuth(ctx, testPool, testAuth)
		require.NoError(t, err, "Failed to insert test user")

		// Act
		result, err := repo.GetByEmail(ctx, *testAuth.Email)

		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, testAuth.Email, result.Email)
	})

	t.Run("should return ErrorUserNotFound when email does not exist", func(t *testing.T) {
		// Arrange
		cleanupAuthTable(t, ctx)

		// Act
		result, err := repo.GetByEmail(ctx, "nonexistent_email")

		// Assert
		assert.Error(t, err)
		assert.ErrorIs(t, err, authErrors.ErrorUserNotFound)
		assert.Nil(t, result)
	})
}

func TestAuthRepositoryRead_GetByPhoneNumber(t *testing.T) {
	ctx := context.Background()
	repo := newTestAuthReadRepository()

	t.Run("should return user when phonenumber exists", func(t *testing.T) {
		// Arrange
		cleanupAuthTable(t, ctx)
		testAuth := fixtures.NewTestAuth()
		err := fixtures.InsertAuth(ctx, testPool, testAuth)
		require.NoError(t, err, "Failed to insert test user")

		// Act
		result, err := repo.GetByPhoneNumber(ctx, *testAuth.PhoneNumber)

		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, testAuth.PhoneNumber, result.PhoneNumber)
	})

	t.Run("should return ErrorUserNotFound when phonenumber not exists", func(t *testing.T) {
		// Arrange
		cleanupAuthTable(t, ctx)

		// Act
		result, err := repo.GetByEmail(ctx, "nonexistent_phonenumber")

		// Assert
		assert.Error(t, err)
		assert.ErrorIs(t, err, authErrors.ErrorUserNotFound)
		assert.Nil(t, result)
	})
}

func TestAuthRepositoryRead_EmailExists(t *testing.T) {
	ctx := context.Background()
	repo := newTestAuthReadRepository()

	t.Run("should return true when email exists", func(t *testing.T) {
		// Arrange
		cleanupAuthTable(t, ctx)
		testAuth := fixtures.NewTestAuth()
		err := fixtures.InsertAuth(ctx, testPool, testAuth)
		require.NoError(t, err)

		// Act
		exists, err := repo.EmailExists(ctx, *testAuth.Email)

		// Assert
		assert.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("should return false when email does not exist", func(t *testing.T) {
		// Arrange
		cleanupAuthTable(t, ctx)

		// Act
		exists, err := repo.EmailExists(ctx, "nonexistent@example.com")

		assert.NoError(t, err)
		assert.False(t, exists)
	})
}

func TestAuthRepositoryRead_PhoneNumberExists(t *testing.T) {
	ctx := context.Background()
	repo := newTestAuthReadRepository()

	t.Run("should return true when phone number exists", func(t *testing.T) {
		// Arrange
		cleanupAuthTable(t, ctx)
		testAuth := fixtures.NewTestAuth()
		err := fixtures.InsertAuth(ctx, testPool, testAuth)
		require.NoError(t, err)

		// Act
		exists, err := repo.PhoneNumberExists(ctx, *testAuth.PhoneNumber)

		// Assert
		assert.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("should return false when phone number does not exist", func(t *testing.T) {
		// Arrange
		cleanupAuthTable(t, ctx)

		// Act
		exists, err := repo.PhoneNumberExists(ctx, "+22890123456")

		assert.NoError(t, err)
		assert.False(t, exists)
	})
}
