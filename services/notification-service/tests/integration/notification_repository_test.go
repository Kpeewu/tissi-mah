package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Kpeewu/tissi-mah/services/notification-service/fixtures"
)

func TestNotificationRepository_Create(t *testing.T) {
	ctx := context.Background()
	repo := newNotificationRepo()
	cleanupTables(t, ctx, "notifications")

	n := fixtures.NewTestNotification(
		fixtures.WithNotifEventID("evt-create-001"),
		fixtures.WithNotifChannel("push"),
		fixtures.WithNotifStatus("pending"),
	)

	err := repo.Create(ctx, n)
	require.NoError(t, err)
	assert.NotEmpty(t, n.NotificationID, "notification_id doit être rempli par RETURNING")
}

func TestNotificationRepository_ExistsByEventIDAndChannel(t *testing.T) {
	ctx := context.Background()
	repo := newNotificationRepo()
	cleanupTables(t, ctx, "notifications")

	n := fixtures.NewTestNotification(
		fixtures.WithNotifEventID("evt-dedup-001"),
		fixtures.WithNotifChannel("push"),
	)
	require.NoError(t, repo.Create(ctx, n))

	t.Run("should return true when exists", func(t *testing.T) {
		exists, err := repo.ExistsByEventIDAndChannel(ctx, "evt-dedup-001", "push")
		require.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("should return false when not exists", func(t *testing.T) {
		exists, err := repo.ExistsByEventIDAndChannel(ctx, "evt-dedup-001", "email")
		require.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("should return false for unknown event_id", func(t *testing.T) {
		exists, err := repo.ExistsByEventIDAndChannel(ctx, "evt-unknown", "push")
		require.NoError(t, err)
		assert.False(t, exists)
	})
}

func TestNotificationRepository_UpdateStatus(t *testing.T) {
	ctx := context.Background()
	repo := newNotificationRepo()
	cleanupTables(t, ctx, "notifications")

	n := fixtures.NewTestNotification(
		fixtures.WithNotifEventID("evt-status-001"),
		fixtures.WithNotifChannel("push"),
		fixtures.WithNotifStatus("processing"),
	)
	require.NoError(t, repo.Create(ctx, n))

	err := repo.UpdateStatus(ctx, n.NotificationID, "sent", "", "msg-provider-001")
	require.NoError(t, err)

	// Vérifier en base
	var status string
	err = testPool.QueryRow(ctx,
		"SELECT status FROM notifications WHERE notification_id = $1", n.NotificationID,
	).Scan(&status)
	require.NoError(t, err)
	assert.Equal(t, "sent", status)
}

func TestNotificationRepository_GetPendingForRetry(t *testing.T) {
	ctx := context.Background()
	repo := newNotificationRepo()
	cleanupTables(t, ctx, "notifications")

	// Insérer une notification retryable (failed, attempt < max)
	retryable := fixtures.NewTestNotification(
		fixtures.WithNotifEventID("evt-retry-001"),
		fixtures.WithNotifStatus("failed"),
		fixtures.WithNotifAttemptCount(1),
		fixtures.WithNotifMaxAttempts(3),
	)
	require.NoError(t, repo.Create(ctx, retryable))

	// Insérer une notification non-retryable (attempt >= max)
	exhausted := fixtures.NewTestNotification(
		fixtures.WithNotifEventID("evt-exhaust-001"),
		fixtures.WithNotifStatus("failed"),
		fixtures.WithNotifAttemptCount(3),
		fixtures.WithNotifMaxAttempts(3),
	)
	require.NoError(t, repo.Create(ctx, exhausted))

	// Insérer une notification envoyée (ne doit pas apparaître)
	sent := fixtures.NewTestNotification(
		fixtures.WithNotifEventID("evt-sent-001"),
		fixtures.WithNotifStatus("sent"),
		fixtures.WithNotifAttemptCount(1),
		fixtures.WithNotifMaxAttempts(3),
	)
	require.NoError(t, repo.Create(ctx, sent))

	pending, err := repo.GetPendingForRetry(ctx, 50)
	require.NoError(t, err)

	// Seule la notification retryable doit apparaître
	assert.Len(t, pending, 1)
	assert.Equal(t, retryable.EventID, pending[0].EventID)
}

func TestNotificationRepository_PurgeOldSent(t *testing.T) {
	ctx := context.Background()
	repo := newNotificationRepo()
	cleanupTables(t, ctx, "notifications")

	// Insérer une vieille notification envoyée (15 jours)
	oldSent := fixtures.NewTestNotification(
		fixtures.WithNotifEventID("evt-old-sent"),
		fixtures.WithNotifStatus("sent"),
	)
	require.NoError(t, repo.Create(ctx, oldSent))
	// Reculer la date de création dans le passé
	_, err := testPool.Exec(ctx,
		"UPDATE notifications SET created_at = NOW() - INTERVAL '15 days' WHERE notification_id = $1",
		oldSent.NotificationID,
	)
	require.NoError(t, err)

	// Insérer une notification récente (ne doit pas être supprimée)
	recentSent := fixtures.NewTestNotification(
		fixtures.WithNotifEventID("evt-recent-sent"),
		fixtures.WithNotifStatus("sent"),
	)
	require.NoError(t, repo.Create(ctx, recentSent))

	deleted, err := repo.PurgeOldSent(ctx, 14)
	require.NoError(t, err)
	assert.Equal(t, int64(1), deleted)

	// La récente doit toujours exister
	var count int
	err = testPool.QueryRow(ctx,
		"SELECT COUNT(*) FROM notifications WHERE notification_id = $1", recentSent.NotificationID,
	).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestNotificationRepository_UpdateRetry(t *testing.T) {
	ctx := context.Background()
	repo := newNotificationRepo()
	cleanupTables(t, ctx, "notifications")

	n := fixtures.NewTestNotification(
		fixtures.WithNotifEventID("evt-retry-update"),
		fixtures.WithNotifStatus("failed"),
		fixtures.WithNotifAttemptCount(1),
		fixtures.WithNotifMaxAttempts(3),
	)
	require.NoError(t, repo.Create(ctx, n))

	n.Status = "failed"
	n.AttemptCount = 2
	n.FailureReason = "FCM_ERROR"
	next := time.Now().Add(2 * time.Minute)
	n.NextAttemptAt = &next

	err := repo.UpdateRetry(ctx, n)
	require.NoError(t, err)

	var dbStatus string
	var dbAttempt int16
	err = testPool.QueryRow(ctx,
		"SELECT status, attempt_count FROM notifications WHERE notification_id = $1",
		n.NotificationID,
	).Scan(&dbStatus, &dbAttempt)
	require.NoError(t, err)
	assert.Equal(t, "failed", dbStatus)
	assert.Equal(t, int16(2), dbAttempt)
}

// TestNotificationRepository_UniqueDedup vérifie la contrainte UNIQUE (event_id, channel).
func TestNotificationRepository_UniqueDedup(t *testing.T) {
	ctx := context.Background()
	repo := newNotificationRepo()
	cleanupTables(t, ctx, "notifications")

	n := fixtures.NewTestNotification(
		fixtures.WithNotifEventID("evt-unique-001"),
		fixtures.WithNotifChannel("push"),
	)
	require.NoError(t, repo.Create(ctx, n))

	// Tenter un second insert avec le même event_id + channel
	dup := fixtures.NewTestNotification(
		fixtures.WithNotifEventID("evt-unique-001"),
		fixtures.WithNotifChannel("push"),
	)
	err := repo.Create(ctx, dup)
	assert.Error(t, err, "la contrainte UNIQUE (event_id, channel) doit être violée")
}

