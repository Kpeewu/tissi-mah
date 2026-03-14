package integration

import (
	"context"
	"log"
	"os"
	"testing"
	"time"

	postgresHelper "github.com/Kpeewu/tissi-mah/pkg-test/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	testPool *pgxpool.Pool
)

func TestMain(m *testing.M) {
	ctx := context.Background()

	// Setup : démarrer PostgreSQL avec les migrations du trips-service
	testPostgres, err := postgresHelper.SetupTestPostgres(ctx, "../../migrations/up",
		postgresHelper.WithImage("postgis/postgis:16-3.4"),
	)
	if err != nil {
		log.Fatalf("Failed to setup test database: %v", err)
	}

	// Stocker le pool dans la variable globale
	testPool = testPostgres.Pool

	// Exécuter tous les tests
	code := m.Run()

	// Cleanup : arrêter PostgreSQL
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := testPostgres.CleanUp(ctx); err != nil {
		log.Printf("Failed to cleanup test database: %v", err)
	}

	os.Exit(code)
}
