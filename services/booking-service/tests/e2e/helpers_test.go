package e2e

import (
	"context"
	"testing"
)

// cleanupBookingTables vide toutes les tables (TRUNCATE CASCADE)
func cleanupBookingTables(t *testing.T, ctx context.Context) {
	t.Helper()

	_, err := testPool.Exec(ctx, "TRUNCATE TABLE bookings, bookings_segments, bookings_status_history CASCADE")
	if err != nil {
		t.Fatalf("Failed to cleanup booking tables: %v", err)
	}
}
