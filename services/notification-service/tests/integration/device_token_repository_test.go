package integration

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Kpeewu/tissi-mah/services/notification-service/fixtures"
	"github.com/Kpeewu/tissi-mah/services/notification-service/internal/domain"
)

func TestDeviceTokenRepository_Upsert_Insert(t *testing.T) {
	ctx := context.Background()
	repo := newDeviceTokenRepo()
	cleanupTables(t, ctx, "user_device_tokens")

	token := &domain.UserDeviceToken{
		UserID:     "user-dev-001",
		FCMToken:   "fcm-new-token-abc",
		Platform:   "android",
		DeviceName: "Pixel 8",
	}

	tokenID, err := repo.Upsert(ctx, token)
	require.NoError(t, err)
	assert.NotEmpty(t, tokenID)
}

func TestDeviceTokenRepository_Upsert_Conflict_Reactivates(t *testing.T) {
	ctx := context.Background()
	repo := newDeviceTokenRepo()
	cleanupTables(t, ctx, "user_device_tokens")

	// Insérer un token inactif
	dt := fixtures.NewTestDeviceToken(
		fixtures.WithDeviceUserID("user-dev-002"),
		fixtures.WithDeviceFCMToken("fcm-reactivate-001"),
		fixtures.WithDeviceInactive(),
	)
	require.NoError(t, fixtures.InsertDeviceToken(ctx, testPool, dt))

	// Upsert avec le même fcm_token → doit réactiver
	token := &domain.UserDeviceToken{
		UserID:   "user-dev-002",
		FCMToken: "fcm-reactivate-001",
		Platform: "android",
	}
	_, err := repo.Upsert(ctx, token)
	require.NoError(t, err)

	// Vérifier que is_active = true maintenant
	var isActive bool
	err = testPool.QueryRow(ctx,
		"SELECT is_active FROM user_device_tokens WHERE fcm_token = $1", "fcm-reactivate-001",
	).Scan(&isActive)
	require.NoError(t, err)
	assert.True(t, isActive, "le token doit être réactivé après upsert")
}

func TestDeviceTokenRepository_GetActiveByUserID(t *testing.T) {
	ctx := context.Background()
	repo := newDeviceTokenRepo()
	cleanupTables(t, ctx, "user_device_tokens")

	userID := "user-tokens-test"

	// 2 tokens actifs
	for i := 0; i < 2; i++ {
		dt := fixtures.NewTestDeviceToken(
			fixtures.WithDeviceUserID(userID),
		)
		require.NoError(t, fixtures.InsertDeviceToken(ctx, testPool, dt))
	}
	// 1 token inactif
	dtInactive := fixtures.NewTestDeviceToken(
		fixtures.WithDeviceUserID(userID),
		fixtures.WithDeviceInactive(),
	)
	require.NoError(t, fixtures.InsertDeviceToken(ctx, testPool, dtInactive))

	tokens, err := repo.GetActiveByUserID(ctx, userID)
	require.NoError(t, err)
	assert.Len(t, tokens, 2, "seuls les 2 tokens actifs doivent être retournés")
	for _, tok := range tokens {
		assert.True(t, tok.IsActive)
	}
}

func TestDeviceTokenRepository_GetActiveByUserID_Empty(t *testing.T) {
	ctx := context.Background()
	repo := newDeviceTokenRepo()
	cleanupTables(t, ctx, "user_device_tokens")

	tokens, err := repo.GetActiveByUserID(ctx, "user-no-tokens")
	require.NoError(t, err)
	assert.Empty(t, tokens)
}

func TestDeviceTokenRepository_InvalidateByFCMToken(t *testing.T) {
	ctx := context.Background()
	repo := newDeviceTokenRepo()
	cleanupTables(t, ctx, "user_device_tokens")

	dt := fixtures.NewTestDeviceToken(
		fixtures.WithDeviceUserID("user-invalidate"),
		fixtures.WithDeviceFCMToken("fcm-to-invalidate"),
	)
	require.NoError(t, fixtures.InsertDeviceToken(ctx, testPool, dt))

	err := repo.InvalidateByFCMToken(ctx, "fcm-to-invalidate")
	require.NoError(t, err)

	var isActive bool
	err = testPool.QueryRow(ctx,
		"SELECT is_active FROM user_device_tokens WHERE fcm_token = $1", "fcm-to-invalidate",
	).Scan(&isActive)
	require.NoError(t, err)
	assert.False(t, isActive)
}
