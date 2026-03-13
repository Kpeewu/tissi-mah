package integration

import (
	"context"
	"testing"

	"github.com/Kpeewu/tissi-mah/services/rating-service/internal/repository/implementations"
	"github.com/Kpeewu/tissi-mah/services/rating-service/internal/repository/interfaces"
	"go.uber.org/zap"
)

// cleanupRatingsTable vide la table ratings (TRUNCATE)
func cleanupRatingsTable(t *testing.T, ctx context.Context) {
	t.Helper()

	_, err := testPool.Exec(ctx, "TRUNCATE TABLE ratings CASCADE")
	if err != nil {
		t.Fatalf("Failed to cleanup ratings table: %v", err)
	}
}

// newTestRatingReadRepository crée une instance du read repository pour les tests
func newTestRatingReadRepository() interfaces.RatingRepositoryRead {
	return implementations.NewRatingReadRepository(testPool, zap.NewNop())
}

// newTestRatingWriteRepository crée une instance du write repository pour les tests
func newTestRatingWriteRepository() interfaces.RatingRepositoryWrite {
	return implementations.NewRatingWriteRepository(testPool, zap.NewNop())
}
