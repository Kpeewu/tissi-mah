package integration

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Kpeewu/tissi-mah/services/notification-service/fixtures"
	"github.com/Kpeewu/tissi-mah/services/notification-service/internal/domain"
)

func TestPreferenceRepository_GetByUserID_DefaultsWhenNoRow(t *testing.T) {
	ctx := context.Background()
	repo := newPreferenceRepo()
	cleanupTables(t, ctx, "user_notification_preferences")

	// Aucune entrée → valeurs par défaut (tout activé)
	prefs, err := repo.GetByUserID(ctx, "user-no-prefs")
	require.NoError(t, err)
	require.NotNil(t, prefs)
	assert.Equal(t, "user-no-prefs", prefs.UserID)
	assert.True(t, prefs.PushEnabled, "PushEnabled doit être true par défaut")
	assert.True(t, prefs.EmailEnabled, "EmailEnabled doit être true par défaut")
}

func TestPreferenceRepository_GetByUserID_ExistingRow(t *testing.T) {
	ctx := context.Background()
	repo := newPreferenceRepo()
	cleanupTables(t, ctx, "user_notification_preferences")

	pref := fixtures.NewTestPreference(
		fixtures.WithPrefUserID("user-has-prefs"),
		fixtures.WithPrefEmailDisabled(),
	)
	require.NoError(t, fixtures.InsertPreference(ctx, testPool, pref))

	result, err := repo.GetByUserID(ctx, "user-has-prefs")
	require.NoError(t, err)
	assert.Equal(t, "user-has-prefs", result.UserID)
	assert.True(t, result.PushEnabled)
	assert.False(t, result.EmailEnabled)
}

func TestPreferenceRepository_Upsert_Insert(t *testing.T) {
	ctx := context.Background()
	repo := newPreferenceRepo()
	cleanupTables(t, ctx, "user_notification_preferences")

	pref := &domain.UserNotificationPreference{
		UserID:       "user-upsert-new",
		PushEnabled:  true,
		EmailEnabled: false,
	}

	err := repo.Upsert(ctx, pref)
	require.NoError(t, err)

	// Vérifier l'insertion
	result, err := repo.GetByUserID(ctx, "user-upsert-new")
	require.NoError(t, err)
	assert.True(t, result.PushEnabled)
	assert.False(t, result.EmailEnabled)
}

func TestPreferenceRepository_Upsert_Update(t *testing.T) {
	ctx := context.Background()
	repo := newPreferenceRepo()
	cleanupTables(t, ctx, "user_notification_preferences")

	// Insert initial
	pref := &domain.UserNotificationPreference{
		UserID:       "user-upsert-existing",
		PushEnabled:  true,
		EmailEnabled: true,
	}
	require.NoError(t, repo.Upsert(ctx, pref))

	// Update via Upsert (ON CONFLICT)
	updated := &domain.UserNotificationPreference{
		UserID:       "user-upsert-existing",
		PushEnabled:  false,
		EmailEnabled: false,
	}
	require.NoError(t, repo.Upsert(ctx, updated))

	result, err := repo.GetByUserID(ctx, "user-upsert-existing")
	require.NoError(t, err)
	assert.False(t, result.PushEnabled, "PushEnabled doit être mis à jour à false")
	assert.False(t, result.EmailEnabled, "EmailEnabled doit être mis à jour à false")
}
