package e2e

import (
	"context"
	"testing"

	"google.golang.org/grpc/metadata"
)

// unknownFirebaseUID est un Firebase UID sans profil côté user-service (mock).
const unknownFirebaseUID = "e2e-unknown-firebase-uid"

// Trajet et point de prise en charge des tests no-show, dont l'heure prévue est passée (mock trips-service).
const (
	noShowTripID     = "e2e-trip-noshow"
	noShowWaypointID = "e2e-wp-noshow"
)

// ctxWithUID retourne un contexte avec le Firebase UID dans les metadata gRPC
func ctxWithUID(uid string) context.Context {
	md := metadata.Pairs("x-firebase-uid", uid)
	return metadata.NewOutgoingContext(context.Background(), md)
}

// ctxWithSupportUID retourne un contexte avec l'UID d'un agent support (JWT support validé par l'api-gateway)
func ctxWithSupportUID(uid string) context.Context {
	md := metadata.Pairs("x-support-uid", uid, "x-support-role", "support")
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
