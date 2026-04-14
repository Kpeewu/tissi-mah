package integration

import (
	"context"
	"testing"
	"time"

	"github.com/Kpeewu/tissi-mah/services/support-service/fixtures"
	"github.com/Kpeewu/tissi-mah/services/support-service/internal/domain"
	"github.com/Kpeewu/tissi-mah/services/support-service/internal/repository/implementations"
	supportErrors "github.com/Kpeewu/tissi-mah/services/support-service/pkg/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestWriteRepo_Create(t *testing.T) {
	cleanTables(t)
	write := implementations.NewSupportUserWriteRepository(testPool, zap.NewNop())
	read := implementations.NewSupportUserReadRepository(testPool, zap.NewNop())
	ctx := context.Background()

	t.Run("succès + round-trip", func(t *testing.T) {
		u := &domain.SupportUser{
			UserID:       uuid.NewString(),
			Email:        "new@x.com",
			PasswordHash: fixtures.DefaultPasswordHash,
			FirstName:    "A",
			LastName:     "B",
			Role:         domain.RoleSupport,
			IsActive:     true,
		}
		require.NoError(t, write.Create(ctx, u))

		got, err := read.GetByEmail(ctx, "new@x.com")
		require.NoError(t, err)
		assert.Equal(t, u.UserID, got.UserID)
		assert.Equal(t, "A", got.FirstName)
		assert.WithinDuration(t, time.Now(), got.PasswordChangedAt, 10*time.Second)
	})

	t.Run("email déjà pris → ErrEmailAlreadyExists", func(t *testing.T) {
		u := &domain.SupportUser{
			UserID: uuid.NewString(), Email: "new@x.com",
			PasswordHash: "x", FirstName: "A", LastName: "B",
			Role: domain.RoleSupport, IsActive: true,
		}
		err := write.Create(ctx, u)
		assert.ErrorIs(t, err, supportErrors.ErrEmailAlreadyExists)
	})

	t.Run("rôle invalide → CHECK violation (erreur interne)", func(t *testing.T) {
		u := &domain.SupportUser{
			UserID: uuid.NewString(), Email: "badrole@x.com",
			PasswordHash: "x", FirstName: "A", LastName: "B",
			Role: "root", IsActive: true,
		}
		err := write.Create(ctx, u)
		assert.ErrorIs(t, err, supportErrors.ErrInternal)
	})
}

func TestWriteRepo_UpdatePassword(t *testing.T) {
	cleanTables(t)
	write := implementations.NewSupportUserWriteRepository(testPool, zap.NewNop())
	read := implementations.NewSupportUserReadRepository(testPool, zap.NewNop())
	ctx := context.Background()

	u := fixtures.NewTestSupportUser(fixtures.WithMustChangePassword(true))
	require.NoError(t, fixtures.InsertSupportUser(ctx, testPool, u))
	before, _ := read.GetByID(ctx, u.UserID)

	t.Run("succès + password_changed_at mis à jour", func(t *testing.T) {
		require.NoError(t, write.UpdatePassword(ctx, u.UserID, "newhash", false))
		after, err := read.GetByID(ctx, u.UserID)
		require.NoError(t, err)
		assert.Equal(t, "newhash", after.PasswordHash)
		assert.False(t, after.MustChangePassword)
		assert.True(t, after.PasswordChangedAt.After(before.PasswordChangedAt),
			"password_changed_at devrait avancer")
	})

	t.Run("user inexistant → ErrUserNotFound", func(t *testing.T) {
		err := write.UpdatePassword(ctx, uuid.NewString(), "h", false)
		assert.ErrorIs(t, err, supportErrors.ErrUserNotFound)
	})

	t.Run("soft-deleted → ErrUserNotFound", func(t *testing.T) {
		del := fixtures.NewTestSupportUser(fixtures.WithDeleted())
		require.NoError(t, fixtures.InsertSupportUser(ctx, testPool, del))
		err := write.UpdatePassword(ctx, del.UserID, "h", false)
		assert.ErrorIs(t, err, supportErrors.ErrUserNotFound)
	})
}

func TestWriteRepo_UpdateEmail(t *testing.T) {
	cleanTables(t)
	write := implementations.NewSupportUserWriteRepository(testPool, zap.NewNop())
	read := implementations.NewSupportUserReadRepository(testPool, zap.NewNop())
	ctx := context.Background()

	u := fixtures.NewTestSupportUser(fixtures.WithEmail("old@x.com"))
	require.NoError(t, fixtures.InsertSupportUser(ctx, testPool, u))

	t.Run("succès + email_changed_at", func(t *testing.T) {
		require.NoError(t, write.UpdateEmail(ctx, u.UserID, "new@x.com"))
		got, err := read.GetByID(ctx, u.UserID)
		require.NoError(t, err)
		assert.Equal(t, "new@x.com", got.Email)
		require.NotNil(t, got.EmailChangedAt)
		assert.WithinDuration(t, time.Now(), *got.EmailChangedAt, 10*time.Second)
	})

	t.Run("conflit unicité → ErrEmailAlreadyExists", func(t *testing.T) {
		other := fixtures.NewTestSupportUser(fixtures.WithEmail("taken@x.com"))
		require.NoError(t, fixtures.InsertSupportUser(ctx, testPool, other))
		err := write.UpdateEmail(ctx, u.UserID, "taken@x.com")
		assert.ErrorIs(t, err, supportErrors.ErrEmailAlreadyExists)
	})

	t.Run("user inexistant → ErrUserNotFound", func(t *testing.T) {
		err := write.UpdateEmail(ctx, uuid.NewString(), "any@x.com")
		assert.ErrorIs(t, err, supportErrors.ErrUserNotFound)
	})
}

func TestWriteRepo_Deactivate(t *testing.T) {
	cleanTables(t)
	write := implementations.NewSupportUserWriteRepository(testPool, zap.NewNop())
	read := implementations.NewSupportUserReadRepository(testPool, zap.NewNop())
	ctx := context.Background()

	u := fixtures.NewTestSupportUser()
	require.NoError(t, fixtures.InsertSupportUser(ctx, testPool, u))

	t.Run("succès : is_active=false", func(t *testing.T) {
		require.NoError(t, write.Deactivate(ctx, u.UserID))
		got, err := read.GetByID(ctx, u.UserID)
		require.NoError(t, err)
		assert.False(t, got.IsActive)
	})

	t.Run("déjà désactivé : idempotent (pas d'erreur)", func(t *testing.T) {
		require.NoError(t, write.Deactivate(ctx, u.UserID))
	})

	t.Run("user inexistant → ErrUserNotFound", func(t *testing.T) {
		err := write.Deactivate(ctx, uuid.NewString())
		assert.ErrorIs(t, err, supportErrors.ErrUserNotFound)
	})
}
