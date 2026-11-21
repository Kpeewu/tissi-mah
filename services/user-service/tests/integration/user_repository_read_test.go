package integration

import (
	"context"
	"testing"

	"github.com/Kpeewu/tissi-mah/services/user-service/fixtures"
	userErrors "github.com/Kpeewu/tissi-mah/services/user-service/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserRepositoryRead_GetByUserID(t *testing.T) {
	repo := newReadRepo()

	t.Run("should return user when found", func(t *testing.T) {
		cleanCollection(t)
		user := fixtures.NewTestUser()
		insertUser(t, user)

		result, err := repo.GetByUserID(context.Background(), user.UserID)

		require.NoError(t, err)
		assert.Equal(t, user.UserID, result.UserID)
		assert.Equal(t, user.AuthID, result.AuthID)
		assert.Equal(t, user.FirebaseID, result.FirebaseID)
		assert.Equal(t, user.Name, result.Name)
		assert.Equal(t, user.FirstName, result.FirstName)
	})

	t.Run("should return ErrorUserNotFound when not found", func(t *testing.T) {
		cleanCollection(t)

		result, err := repo.GetByUserID(context.Background(), "nonexistent-id")

		assert.Nil(t, result)
		assert.ErrorIs(t, err, userErrors.ErrorUserNotFound)
	})

	t.Run("should return ErrorUserNotFound for soft-deleted user", func(t *testing.T) {
		cleanCollection(t)
		user := fixtures.NewTestUser(fixtures.WithDeleted())
		insertUser(t, user)

		result, err := repo.GetByUserID(context.Background(), user.UserID)

		assert.Nil(t, result)
		assert.ErrorIs(t, err, userErrors.ErrorUserNotFound)
	})
}

func TestUserRepositoryRead_GetByAuthID(t *testing.T) {
	repo := newReadRepo()

	t.Run("should return user when found", func(t *testing.T) {
		cleanCollection(t)
		user := fixtures.NewTestUser()
		insertUser(t, user)

		result, err := repo.GetByAuthID(context.Background(), user.AuthID)

		require.NoError(t, err)
		assert.Equal(t, user.UserID, result.UserID)
		assert.Equal(t, user.AuthID, result.AuthID)
	})

	t.Run("should return ErrorUserNotFound when not found", func(t *testing.T) {
		cleanCollection(t)

		result, err := repo.GetByAuthID(context.Background(), "nonexistent-auth-id")

		assert.Nil(t, result)
		assert.ErrorIs(t, err, userErrors.ErrorUserNotFound)
	})

	t.Run("should return ErrorUserNotFound for soft-deleted user", func(t *testing.T) {
		cleanCollection(t)
		user := fixtures.NewTestUser(fixtures.WithDeleted())
		insertUser(t, user)

		result, err := repo.GetByAuthID(context.Background(), user.AuthID)

		assert.Nil(t, result)
		assert.ErrorIs(t, err, userErrors.ErrorUserNotFound)
	})
}

func TestUserRepositoryRead_GetByFirebaseID(t *testing.T) {
	repo := newReadRepo()

	t.Run("should return user when found", func(t *testing.T) {
		cleanCollection(t)
		user := fixtures.NewTestUser()
		insertUser(t, user)

		result, err := repo.GetByFirebaseID(context.Background(), user.FirebaseID)

		require.NoError(t, err)
		assert.Equal(t, user.UserID, result.UserID)
		assert.Equal(t, user.FirebaseID, result.FirebaseID)
	})

	t.Run("should return ErrorUserNotFound when not found", func(t *testing.T) {
		cleanCollection(t)

		result, err := repo.GetByFirebaseID(context.Background(), "nonexistent-firebase-id")

		assert.Nil(t, result)
		assert.ErrorIs(t, err, userErrors.ErrorUserNotFound)
	})

	t.Run("should return ErrorUserNotFound for soft-deleted user", func(t *testing.T) {
		cleanCollection(t)
		user := fixtures.NewTestUser(fixtures.WithDeleted())
		insertUser(t, user)

		result, err := repo.GetByFirebaseID(context.Background(), user.FirebaseID)

		assert.Nil(t, result)
		assert.ErrorIs(t, err, userErrors.ErrorUserNotFound)
	})
}
