package integration

import (
	"context"
	"testing"

	"github.com/Kpeewu/tissi-mah/services/auth-service/internal/repository/implementations"
	"github.com/Kpeewu/tissi-mah/services/auth-service/internal/repository/interfaces"
)

// cleanupAuthTable vide la table auth (TRUNCATE)
func cleanupAuthTable(t *testing.T, ctx context.Context) {
	t.Helper() // Marque cette fonction comme helper

	_, err := testPool.Exec(ctx, "TRUNCATE TABLE auth CASCADE")
	if err != nil {
		t.Fatalf("Failed to cleanup auth table: %v", err)
	}
}

// newTestAuthReadRepository crée une instance du repository pour les tests
func newTestAuthReadRepository() interfaces.AuthRepositoryRead {
	return implementations.NewAuthReadRepository(testPool)
}
