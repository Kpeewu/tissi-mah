package e2e

import (
	"context"
	"testing"
)

// cleanupPaymentTables vide toutes les tables (TRUNCATE CASCADE)
func cleanupPaymentTables(t *testing.T, ctx context.Context) {
	t.Helper()

	_, err := testPool.Exec(ctx, "TRUNCATE TABLE payments, refunds, payouts, webhook_events, payout_batches CASCADE")
	if err != nil {
		t.Fatalf("Failed to cleanup payment tables: %v", err)
	}
}
