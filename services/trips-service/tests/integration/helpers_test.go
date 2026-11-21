package integration

import (
	"context"
	"testing"

	"github.com/Kpeewu/tissi-mah/services/trips-service/internal/repository/implementations"
	i "github.com/Kpeewu/tissi-mah/services/trips-service/internal/repository/interfaces"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// cleanupTripsTable vide la table trips (et ses dépendances via CASCADE).
func cleanupTripsTable(t *testing.T, ctx context.Context) {
	t.Helper()
	_, err := testPool.Exec(ctx, "TRUNCATE trips CASCADE")
	require.NoError(t, err, "cleanup trips table failed")
}

// newTestWriteRepository crée une instance du repository d'écriture pour les tests.
func newTestWriteRepository() i.TripRepositoryWrite {
	logger := zap.NewNop()
	return implementations.NewTripWriteRepository(testPool, logger)
}

// newTestReadRepository crée une instance du repository de lecture pour les tests.
func newTestReadRepository() i.TripRepositoryRead {
	logger := zap.NewNop()
	return implementations.NewTripReadRepository(testPool, logger)
}
