package integration

import (
	"context"
	"testing"
	"time"

	"github.com/Kpeewu/tissi-mah/services/auth-service/fixtures"
	authErrors "github.com/Kpeewu/tissi-mah/services/auth-service/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Create
// =============================================================================

func TestAuthRepositoryWrite_Create(t *testing.T) {
	ctx := context.Background()
	writeRepo := newTestAuthWriteRepository()
	readRepo := newTestAuthReadRepository()

	t.Run("should create auth record and return authID", func(t *testing.T) {
		cleanupAuthTable(t, ctx)

		auth := fixtures.NewTestAuth()

		authID, err := writeRepo.Create(ctx, auth)

		require.NoError(t, err)
		assert.NotEmpty(t, authID)
		assert.Equal(t, auth.AuthID, authID)

		// Vérifier en base
		result, err := readRepo.GetByAuthID(ctx, authID)
		require.NoError(t, err)
		assert.Equal(t, auth.FirebaseID, result.FirebaseID)
		assert.Equal(t, auth.Email, result.Email)
		assert.Equal(t, auth.PhoneNumber, result.PhoneNumber)
		assert.True(t, result.IsActive)
		assert.False(t, result.IsSuspended)
		assert.Nil(t, result.DeletedAt)
	})

	t.Run("should create auth record with email only (no phone)", func(t *testing.T) {
		cleanupAuthTable(t, ctx)

		auth := fixtures.NewTestAuth(fixtures.WithNoPhoneNumber())

		authID, err := writeRepo.Create(ctx, auth)

		require.NoError(t, err)
		assert.NotEmpty(t, authID)

		result, err := readRepo.GetByAuthID(ctx, authID)
		require.NoError(t, err)
		assert.NotNil(t, result.Email)
		assert.Nil(t, result.PhoneNumber)
	})

	t.Run("should create auth record with phone only (no email)", func(t *testing.T) {
		cleanupAuthTable(t, ctx)

		auth := fixtures.NewTestAuth(fixtures.WithNoEmail())

		authID, err := writeRepo.Create(ctx, auth)

		require.NoError(t, err)

		result, err := readRepo.GetByAuthID(ctx, authID)
		require.NoError(t, err)
		assert.Nil(t, result.Email)
		assert.NotNil(t, result.PhoneNumber)
	})

	t.Run("should fail on duplicate firebase_id (unique constraint)", func(t *testing.T) {
		cleanupAuthTable(t, ctx)

		auth1 := fixtures.NewTestAuth(fixtures.WithFirebaseID("dup-firebase-id"))
		require.NoError(t, fixtures.InsertAuth(ctx, testPool, auth1))

		auth2 := fixtures.NewTestAuth(
			fixtures.WithFirebaseID("dup-firebase-id"),
			fixtures.WithEmail("other@example.com"),
		)

		_, err := writeRepo.Create(ctx, auth2)

		assert.Error(t, err)
		assert.ErrorIs(t, err, authErrors.ErrorInternalServer)
	})

	t.Run("should fail on duplicate email (unique constraint)", func(t *testing.T) {
		cleanupAuthTable(t, ctx)

		auth1 := fixtures.NewTestAuth(fixtures.WithEmail("dup@example.com"))
		require.NoError(t, fixtures.InsertAuth(ctx, testPool, auth1))

		auth2 := fixtures.NewTestAuth(fixtures.WithEmail("dup@example.com"))

		_, err := writeRepo.Create(ctx, auth2)

		assert.Error(t, err)
	})
}

// =============================================================================
// Update
// =============================================================================

func TestAuthRepositoryWrite_Update(t *testing.T) {
	ctx := context.Background()
	writeRepo := newTestAuthWriteRepository()
	readRepo := newTestAuthReadRepository()

	t.Run("should update email and phone number", func(t *testing.T) {
		cleanupAuthTable(t, ctx)

		auth := fixtures.NewTestAuth()
		require.NoError(t, fixtures.InsertAuth(ctx, testPool, auth))

		newEmail := "updated@example.com"
		newPhone := "+22899999999"
		auth.Email = &newEmail
		auth.PhoneNumber = &newPhone

		updated, err := writeRepo.Update(ctx, auth)

		require.NoError(t, err)
		assert.NotNil(t, updated)
		assert.Equal(t, newEmail, *updated.Email)
		assert.Equal(t, newPhone, *updated.PhoneNumber)

		// Vérifier en base
		result, err := readRepo.GetByAuthID(ctx, auth.AuthID)
		require.NoError(t, err)
		assert.Equal(t, newEmail, *result.Email)
		assert.Equal(t, newPhone, *result.PhoneNumber)
	})

	t.Run("should suspend account", func(t *testing.T) {
		cleanupAuthTable(t, ctx)

		auth := fixtures.NewTestAuth()
		require.NoError(t, fixtures.InsertAuth(ctx, testPool, auth))

		suspensionEnd := time.Now().UTC().Add(24 * time.Hour)
		auth.Suspend(suspensionEnd)

		updated, err := writeRepo.Update(ctx, auth)

		require.NoError(t, err)
		assert.True(t, updated.IsSuspended)
		assert.NotNil(t, updated.SuspensionEndDate)
	})

	t.Run("should deactivate account", func(t *testing.T) {
		cleanupAuthTable(t, ctx)

		auth := fixtures.NewTestAuth()
		require.NoError(t, fixtures.InsertAuth(ctx, testPool, auth))

		auth.Deactivate()

		updated, err := writeRepo.Update(ctx, auth)

		require.NoError(t, err)
		assert.False(t, updated.IsActive)
	})

	t.Run("should return ErrorUserNotFound when auth_id does not exist", func(t *testing.T) {
		cleanupAuthTable(t, ctx)

		auth := fixtures.NewTestAuth()
		// Ne pas insérer — l'ID n'existe pas en base

		_, err := writeRepo.Update(ctx, auth)

		assert.Error(t, err)
		assert.ErrorIs(t, err, authErrors.ErrorUserNotFound)
	})
}

// =============================================================================
// Delete
// =============================================================================

func TestAuthRepositoryWrite_Delete(t *testing.T) {
	ctx := context.Background()
	writeRepo := newTestAuthWriteRepository()
	readRepo := newTestAuthReadRepository()

	t.Run("should anonymize and soft-delete account", func(t *testing.T) {
		cleanupAuthTable(t, ctx)

		auth := fixtures.NewTestAuth()
		require.NoError(t, fixtures.InsertAuth(ctx, testPool, auth))

		err := writeRepo.Delete(ctx, auth)

		require.NoError(t, err)

		// L'enregistrement ne doit plus être trouvable (deleted_at IS NOT NULL filtre les lectures)
		result, err := readRepo.GetByAuthID(ctx, auth.AuthID)
		assert.Error(t, err)
		assert.ErrorIs(t, err, authErrors.ErrorUserNotFound)
		assert.Nil(t, result)
	})

	t.Run("should anonymize PII fields on delete", func(t *testing.T) {
		cleanupAuthTable(t, ctx)

		auth := fixtures.NewTestAuth()
		require.NoError(t, fixtures.InsertAuth(ctx, testPool, auth))

		err := writeRepo.Delete(ctx, auth)

		require.NoError(t, err)

		// Vérifier directement en base que les données sont anonymisées
		var firebaseID string
		var email *string
		var phoneNumber *string
		var isActive bool
		var deletedAt *time.Time

		row := testPool.QueryRow(ctx,
			"SELECT firebase_id, email, phone_number, is_active, deleted_at FROM auth WHERE auth_id = $1",
			auth.AuthID,
		)
		err = row.Scan(&firebaseID, &email, &phoneNumber, &isActive, &deletedAt)
		require.NoError(t, err)

		assert.Equal(t, "deleted_"+auth.AuthID, firebaseID)
		assert.Nil(t, phoneNumber)
		assert.False(t, isActive)
		assert.NotNil(t, deletedAt)
	})

	t.Run("should delete account found by firebase_id", func(t *testing.T) {
		cleanupAuthTable(t, ctx)

		originalFirebaseID := "to-delete-firebase-id"
		auth := fixtures.NewTestAuth(fixtures.WithFirebaseID(originalFirebaseID))
		require.NoError(t, fixtures.InsertAuth(ctx, testPool, auth))

		err := writeRepo.Delete(ctx, auth)
		require.NoError(t, err)

		// Plus trouvable par firebase_id original non plus
		result, err := readRepo.GetByFirebaseID(ctx, originalFirebaseID)
		assert.Error(t, err)
		assert.ErrorIs(t, err, authErrors.ErrorUserNotFound)
		assert.Nil(t, result)
	})
}
