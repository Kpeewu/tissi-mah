package integration

import (
	"context"
	"testing"

	"github.com/Kpeewu/tissi-mah/services/support-service/fixtures"
	"github.com/Kpeewu/tissi-mah/services/support-service/internal/repository/implementations"
	supportErrors "github.com/Kpeewu/tissi-mah/services/support-service/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestReadRepo_GetByID(t *testing.T) {
	cleanTables(t)
	repo := implementations.NewSupportUserReadRepository(testPool, zap.NewNop())
	ctx := context.Background()

	t.Run("existant", func(t *testing.T) {
		u := fixtures.NewTestSupportUser()
		require.NoError(t, fixtures.InsertSupportUser(ctx, testPool, u))
		got, err := repo.GetByID(ctx, u.UserID)
		require.NoError(t, err)
		assert.Equal(t, u.Email, got.Email)
		assert.Equal(t, u.Role, got.Role)
	})

	t.Run("inexistant → ErrUserNotFound", func(t *testing.T) {
		_, err := repo.GetByID(ctx, "00000000-0000-0000-0000-000000000999")
		assert.ErrorIs(t, err, supportErrors.ErrUserNotFound)
	})

	t.Run("soft-deleted → filtré", func(t *testing.T) {
		u := fixtures.NewTestSupportUser(fixtures.WithDeleted())
		require.NoError(t, fixtures.InsertSupportUser(ctx, testPool, u))
		_, err := repo.GetByID(ctx, u.UserID)
		assert.ErrorIs(t, err, supportErrors.ErrUserNotFound)
	})
}

func TestReadRepo_GetByEmail(t *testing.T) {
	cleanTables(t)
	repo := implementations.NewSupportUserReadRepository(testPool, zap.NewNop())
	ctx := context.Background()

	t.Run("existant", func(t *testing.T) {
		u := fixtures.NewTestSupportUser(fixtures.WithEmail("ge@x.com"))
		require.NoError(t, fixtures.InsertSupportUser(ctx, testPool, u))
		got, err := repo.GetByEmail(ctx, "ge@x.com")
		require.NoError(t, err)
		assert.Equal(t, u.UserID, got.UserID)
	})

	t.Run("inconnu → ErrUserNotFound", func(t *testing.T) {
		_, err := repo.GetByEmail(ctx, "nope@x.com")
		assert.ErrorIs(t, err, supportErrors.ErrUserNotFound)
	})

	t.Run("soft-deleted → filtré", func(t *testing.T) {
		u := fixtures.NewTestSupportUser(fixtures.WithEmail("del@x.com"), fixtures.WithDeleted())
		require.NoError(t, fixtures.InsertSupportUser(ctx, testPool, u))
		_, err := repo.GetByEmail(ctx, "del@x.com")
		assert.ErrorIs(t, err, supportErrors.ErrUserNotFound)
	})
}

func TestReadRepo_ExistsByEmail(t *testing.T) {
	cleanTables(t)
	repo := implementations.NewSupportUserReadRepository(testPool, zap.NewNop())
	ctx := context.Background()

	u := fixtures.NewTestSupportUser(fixtures.WithEmail("exists@x.com"))
	require.NoError(t, fixtures.InsertSupportUser(ctx, testPool, u))

	ex, err := repo.ExistsByEmail(ctx, "exists@x.com")
	require.NoError(t, err)
	assert.True(t, ex)

	ex, err = repo.ExistsByEmail(ctx, "nope@x.com")
	require.NoError(t, err)
	assert.False(t, ex)

	// Soft-deleted ne doit pas compter.
	del := fixtures.NewTestSupportUser(fixtures.WithEmail("gone@x.com"), fixtures.WithDeleted())
	require.NoError(t, fixtures.InsertSupportUser(ctx, testPool, del))
	ex, err = repo.ExistsByEmail(ctx, "gone@x.com")
	require.NoError(t, err)
	assert.False(t, ex)
}

func TestReadRepo_List(t *testing.T) {
	cleanTables(t)
	repo := implementations.NewSupportUserReadRepository(testPool, zap.NewNop())
	ctx := context.Background()

	// Insérer 3 actifs + 1 soft-delete.
	for i := 0; i < 3; i++ {
		require.NoError(t, fixtures.InsertSupportUser(ctx, testPool, fixtures.NewTestSupportUser()))
	}
	require.NoError(t, fixtures.InsertSupportUser(ctx, testPool, fixtures.NewTestSupportUser(fixtures.WithDeleted())))

	t.Run("pagination défaut + exclusion soft-delete", func(t *testing.T) {
		users, total, err := repo.List(ctx, 50, 0)
		require.NoError(t, err)
		assert.Equal(t, 3, total)
		assert.Len(t, users, 3)
	})

	t.Run("limit 2 offset 0", func(t *testing.T) {
		users, total, err := repo.List(ctx, 2, 0)
		require.NoError(t, err)
		assert.Equal(t, 3, total)
		assert.Len(t, users, 2)
	})

	t.Run("offset > total → vide mais total correct", func(t *testing.T) {
		users, total, err := repo.List(ctx, 10, 100)
		require.NoError(t, err)
		assert.Equal(t, 3, total)
		assert.Empty(t, users)
	})

	t.Run("limit négatif → clamp à défaut", func(t *testing.T) {
		_, _, err := repo.List(ctx, -1, 0)
		require.NoError(t, err)
	})
}
