package integration

import (
	"context"
	"testing"

	"github.com/Kpeewu/tissi-mah/services/booking-service/internal/repository/implementations"
	repoInterfaces "github.com/Kpeewu/tissi-mah/services/booking-service/internal/repository/interfaces"
	"go.uber.org/zap"
)

// cleanupBookingTables vide toutes les tables (TRUNCATE CASCADE)
func cleanupBookingTables(t *testing.T, ctx context.Context) {
	t.Helper()

	_, err := testPool.Exec(ctx, "TRUNCATE TABLE bookings, bookings_segments, bookings_status_history CASCADE")
	if err != nil {
		t.Fatalf("Failed to cleanup booking tables: %v", err)
	}
}

func newTestBookingReadRepository() repoInterfaces.BookingRepositoryRead {
	return implementations.NewBookingReadRepository(testPool, zap.NewNop())
}

func newTestBookingWriteRepository() repoInterfaces.BookingRepositoryWrite {
	return implementations.NewBookingWriteRepository(testPool, zap.NewNop())
}
