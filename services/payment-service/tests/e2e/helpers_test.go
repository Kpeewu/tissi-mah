package e2e

import (
	"context"
	"testing"

	"google.golang.org/grpc/metadata"
)

// ctxWithUID retourne un contexte avec le Firebase UID dans les metadata gRPC
func ctxWithUID(uid string) context.Context {
	md := metadata.Pairs("x-firebase-uid", uid)
	return metadata.NewOutgoingContext(context.Background(), md)
}

// cleanupPaymentTables vide toutes les tables (TRUNCATE CASCADE)
func cleanupPaymentTables(t *testing.T, ctx context.Context) {
	t.Helper()

	_, err := testPool.Exec(ctx, "TRUNCATE TABLE payments, refunds, payouts, webhook_events, payout_batches CASCADE")
	if err != nil {
		t.Fatalf("Failed to cleanup payment tables: %v", err)
	}
}
