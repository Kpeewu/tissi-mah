package integration

import (
	"context"
	"testing"

	"go.uber.org/zap"

	"github.com/Kpeewu/tissi-mah/services/notification-service/internal/repository/implementations"
	"github.com/Kpeewu/tissi-mah/services/notification-service/internal/repository/interfaces"
)

func cleanupTables(t *testing.T, ctx context.Context, tables ...string) {
	t.Helper()
	for _, table := range tables {
		_, err := testPool.Exec(ctx, "TRUNCATE TABLE "+table+" CASCADE")
		if err != nil {
			t.Fatalf("Failed to cleanup table %s: %v", table, err)
		}
	}
}

func newNotificationRepo() interfaces.NotificationRepository {
	return implementations.NewNotificationRepository(testPool, zap.NewNop())
}

func newInboxRepo() interfaces.InboxRepository {
	return implementations.NewInboxRepository(testPool, zap.NewNop())
}

func newPreferenceRepo() interfaces.PreferenceRepository {
	return implementations.NewPreferenceRepository(testPool, zap.NewNop())
}

func newDeviceTokenRepo() interfaces.DeviceTokenRepository {
	return implementations.NewDeviceTokenRepository(testPool, zap.NewNop())
}
