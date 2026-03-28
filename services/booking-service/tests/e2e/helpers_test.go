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

// cleanupBookingTables vide toutes les tables (TRUNCATE CASCADE)
func cleanupBookingTables(t *testing.T, ctx context.Context) {
	t.Helper()

	_, err := testPool.Exec(ctx, "TRUNCATE TABLE bookings, bookings_segments, bookings_status_history CASCADE")
	if err != nil {
		t.Fatalf("Failed to cleanup booking tables: %v", err)
	}
}
