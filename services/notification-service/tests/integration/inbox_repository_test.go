package integration

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Kpeewu/tissi-mah/services/notification-service/fixtures"
	notifErrors "github.com/Kpeewu/tissi-mah/services/notification-service/pkg/errors"
)

func TestInboxRepository_Create(t *testing.T) {
	ctx := context.Background()
	repo := newInboxRepo()
	cleanupTables(t, ctx, "notification_inbox")

	entry := fixtures.NewTestInboxEntry(
		fixtures.WithInboxUserID("user-create-001"),
		fixtures.WithInboxEventType("BOOKING_CONFIRMED"),
	)

	err := repo.Create(ctx, entry)
	require.NoError(t, err)
	assert.NotEmpty(t, entry.InboxID, "inbox_id doit être rempli par RETURNING")
}

func TestInboxRepository_GetByUserID_Pagination(t *testing.T) {
	ctx := context.Background()
	repo := newInboxRepo()
	cleanupTables(t, ctx, "notification_inbox")

	// Insérer 5 entrées pour user-paginate
	for i := 0; i < 5; i++ {
		entry := fixtures.NewTestInboxEntry(
			fixtures.WithInboxUserID("user-paginate"),
		)
		require.NoError(t, repo.Create(ctx, entry))
	}

	// Page 1, taille 3
	entries, total, err := repo.GetByUserID(ctx, "user-paginate", 1, 3)
	require.NoError(t, err)
	assert.Equal(t, 5, total)
	assert.Len(t, entries, 3)

	// Page 2, taille 3
	entries2, total2, err := repo.GetByUserID(ctx, "user-paginate", 2, 3)
	require.NoError(t, err)
	assert.Equal(t, 5, total2)
	assert.Len(t, entries2, 2)
}

func TestInboxRepository_GetByUserID_Empty(t *testing.T) {
	ctx := context.Background()
	repo := newInboxRepo()
	cleanupTables(t, ctx, "notification_inbox")

	entries, total, err := repo.GetByUserID(ctx, "user-nobody", 1, 20)
	require.NoError(t, err)
	assert.Equal(t, 0, total)
	assert.Empty(t, entries)
}

func TestInboxRepository_MarkAsRead(t *testing.T) {
	ctx := context.Background()
	repo := newInboxRepo()
	cleanupTables(t, ctx, "notification_inbox")

	entry := fixtures.NewTestInboxEntry(
		fixtures.WithInboxUserID("user-mark-read"),
	)
	require.NoError(t, repo.Create(ctx, entry))

	err := repo.MarkAsRead(ctx, entry.InboxID, "user-mark-read")
	require.NoError(t, err)

	// Vérifier en base
	var isRead bool
	err = testPool.QueryRow(ctx,
		"SELECT is_read FROM notification_inbox WHERE inbox_id = $1", entry.InboxID,
	).Scan(&isRead)
	require.NoError(t, err)
	assert.True(t, isRead)
}

func TestInboxRepository_MarkAsRead_NotFound(t *testing.T) {
	ctx := context.Background()
	repo := newInboxRepo()
	cleanupTables(t, ctx, "notification_inbox")

	err := repo.MarkAsRead(ctx, "00000000-0000-0000-0000-000000000000", "user-xyz")
	assert.ErrorIs(t, err, notifErrors.ErrorNotFound)
}

func TestInboxRepository_MarkAllAsRead(t *testing.T) {
	ctx := context.Background()
	repo := newInboxRepo()
	cleanupTables(t, ctx, "notification_inbox")

	// Insérer 3 non-lues + 1 déjà lue
	for i := 0; i < 3; i++ {
		entry := fixtures.NewTestInboxEntry(
			fixtures.WithInboxUserID("user-mark-all"),
		)
		require.NoError(t, repo.Create(ctx, entry))
	}
	readEntry := fixtures.NewTestInboxEntry(
		fixtures.WithInboxUserID("user-mark-all"),
		fixtures.WithInboxRead(),
	)
	require.NoError(t, fixtures.InsertInboxEntry(ctx, testPool, readEntry))

	count, err := repo.MarkAllAsRead(ctx, "user-mark-all")
	require.NoError(t, err)
	assert.Equal(t, 3, count, "seules les 3 non-lues doivent être mises à jour")
}

func TestInboxRepository_GetUnreadCount(t *testing.T) {
	ctx := context.Background()
	repo := newInboxRepo()
	cleanupTables(t, ctx, "notification_inbox")

	// 2 non-lues + 1 lue
	for i := 0; i < 2; i++ {
		entry := fixtures.NewTestInboxEntry(fixtures.WithInboxUserID("user-unread"))
		require.NoError(t, repo.Create(ctx, entry))
	}
	readEntry := fixtures.NewTestInboxEntry(
		fixtures.WithInboxUserID("user-unread"),
		fixtures.WithInboxRead(),
	)
	require.NoError(t, fixtures.InsertInboxEntry(ctx, testPool, readEntry))

	count, err := repo.GetUnreadCount(ctx, "user-unread")
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestInboxRepository_PurgeOldRead(t *testing.T) {
	ctx := context.Background()
	repo := newInboxRepo()
	cleanupTables(t, ctx, "notification_inbox")

	// Entrée lue ancienne (95 jours) — InsertInboxEntry respecte IsRead=true
	oldRead := fixtures.NewTestInboxEntry(
		fixtures.WithInboxUserID("user-purge"),
		fixtures.WithInboxRead(),
	)
	require.NoError(t, fixtures.InsertInboxEntry(ctx, testPool, oldRead))
	_, err := testPool.Exec(ctx,
		"UPDATE notification_inbox SET created_at = NOW() - INTERVAL '95 days' WHERE inbox_id = $1",
		oldRead.InboxID,
	)
	require.NoError(t, err)

	// Entrée non-lue (ne doit pas être supprimée même si ancienne)
	unread := fixtures.NewTestInboxEntry(
		fixtures.WithInboxUserID("user-purge"),
	)
	require.NoError(t, repo.Create(ctx, unread))
	_, err = testPool.Exec(ctx,
		"UPDATE notification_inbox SET created_at = NOW() - INTERVAL '95 days' WHERE inbox_id = $1",
		unread.InboxID,
	)
	require.NoError(t, err)

	deleted, err := repo.PurgeOldRead(ctx, 90)
	require.NoError(t, err)
	assert.Equal(t, int64(1), deleted, "seule la lue ancienne doit être supprimée")
}
