package e2e

import (
	"context"
	"testing"

	"google.golang.org/grpc/metadata"
)

// e2eUnknownFirebaseUID est un Firebase UID que le résolveur de l'intercepteur ne connaît pas.
const e2eUnknownFirebaseUID = "e2e-unknown"

// ctxWithUID retourne un contexte avec le Firebase UID dans les metadata gRPC
func ctxWithUID(uid string) context.Context {
	md := metadata.Pairs("x-firebase-uid", uid)
	return metadata.NewOutgoingContext(context.Background(), md)
}

// cleanupRatingsTable vide la table ratings (TRUNCATE)
func cleanupRatingsTable(t *testing.T, ctx context.Context) {
	t.Helper()

	_, err := testPool.Exec(ctx, "TRUNCATE TABLE ratings CASCADE")
	if err != nil {
		t.Fatalf("Failed to cleanup ratings table: %v", err)
	}
}
