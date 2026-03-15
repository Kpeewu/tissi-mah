package integration

import (
	"context"
	"testing"
	"time"

	"github.com/Kpeewu/tissi-mah/services/user-service/fixtures"
	userErrors "github.com/Kpeewu/tissi-mah/services/user-service/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserRepositoryWrite_Create(t *testing.T) {
	repo := newWriteRepo()

	t.Run("should create user and return userID", func(t *testing.T) {
		cleanCollection(t)
		user := fixtures.NewTestUser()

		userID, err := repo.Create(context.Background(), user)

		require.NoError(t, err)
		assert.Equal(t, user.UserID, userID)
	})

	t.Run("should return ErrorProfileAlreadyExists on duplicate user_id", func(t *testing.T) {
		cleanCollection(t)
		user := fixtures.NewTestUser()
		insertUser(t, user)

		_, err := repo.Create(context.Background(), user)

		assert.ErrorIs(t, err, userErrors.ErrorProfileAlreadyExists)
	})

	t.Run("should return ErrorProfileAlreadyExists on duplicate auth_id", func(t *testing.T) {
		cleanCollection(t)
		user := fixtures.NewTestUser()
		insertUser(t, user)

		duplicate := fixtures.NewTestUser(fixtures.WithAuthID(user.AuthID))
		_, err := repo.Create(context.Background(), duplicate)

		assert.ErrorIs(t, err, userErrors.ErrorProfileAlreadyExists)
	})
}

func TestUserRepositoryWrite_Update(t *testing.T) {
	repo := newWriteRepo()

	t.Run("should update user and return updated document", func(t *testing.T) {
		cleanCollection(t)
		user := fixtures.NewTestUser()
		insertUser(t, user)

		user.Name = "Updated"
		user.Bio = "Bio mise à jour"

		updated, err := repo.Update(context.Background(), user)

		require.NoError(t, err)
		assert.Equal(t, "Updated", updated.Name)
		assert.Equal(t, "Bio mise à jour", updated.Bio)
		assert.Equal(t, user.UserID, updated.UserID)
	})

	t.Run("should return ErrorUserNotFound when user does not exist", func(t *testing.T) {
		cleanCollection(t)
		user := fixtures.NewTestUser()

		result, err := repo.Update(context.Background(), user)

		assert.Nil(t, result)
		assert.ErrorIs(t, err, userErrors.ErrorUserNotFound)
	})

	t.Run("should return ErrorUserNotFound for soft-deleted user", func(t *testing.T) {
		cleanCollection(t)
		user := fixtures.NewTestUser(fixtures.WithDeleted())
		insertUser(t, user)

		user.Name = "ShouldNotUpdate"
		result, err := repo.Update(context.Background(), user)

		assert.Nil(t, result)
		assert.ErrorIs(t, err, userErrors.ErrorUserNotFound)
	})

	t.Run("should update updated_at timestamp", func(t *testing.T) {
		cleanCollection(t)
		user := fixtures.NewTestUser()
		insertUser(t, user)

		before := time.Now().UTC().Add(-1 * time.Second)
		user.Name = "TimestampTest"
		updated, err := repo.Update(context.Background(), user)

		require.NoError(t, err)
		assert.True(t, updated.UpdatedAt.After(before))
	})
}

func TestUserRepositoryWrite_Delete(t *testing.T) {
	repo := newWriteRepo()

	t.Run("should soft-delete user", func(t *testing.T) {
		cleanCollection(t)
		user := fixtures.NewTestUser()
		insertUser(t, user)

		err := repo.Delete(context.Background(), user.UserID)

		require.NoError(t, err)

		// Vérification via le read repo : l'utilisateur n'est plus visible
		readRepo := newReadRepo()
		result, readErr := readRepo.GetByUserID(context.Background(), user.UserID)
		assert.Nil(t, result)
		assert.ErrorIs(t, readErr, userErrors.ErrorUserNotFound)
	})

	t.Run("should return ErrorUserNotFound when user does not exist", func(t *testing.T) {
		cleanCollection(t)

		err := repo.Delete(context.Background(), "nonexistent-id")

		assert.ErrorIs(t, err, userErrors.ErrorUserNotFound)
	})

	t.Run("should return ErrorUserNotFound for already-deleted user", func(t *testing.T) {
		cleanCollection(t)
		user := fixtures.NewTestUser(fixtures.WithDeleted())
		insertUser(t, user)

		err := repo.Delete(context.Background(), user.UserID)

		assert.ErrorIs(t, err, userErrors.ErrorUserNotFound)
	})
}
