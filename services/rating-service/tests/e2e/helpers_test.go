package e2e

import (
	"context"
	"testing"
)

// cleanupRatingsTable vide la table ratings (TRUNCATE)
func cleanupRatingsTable(t *testing.T, ctx context.Context) {
	t.Helper()

	_, err := testPool.Exec(ctx, "TRUNCATE TABLE ratings CASCADE")
	if err != nil {
		t.Fatalf("Failed to cleanup ratings table: %v", err)
	}
}
