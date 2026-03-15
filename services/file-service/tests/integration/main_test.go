package integration

import (
	"context"
	"log"
	"os"
	"testing"
	"time"

	postgresHelper "github.com/Kpeewu/tissi-mah/pkg-test/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

var (
	testPool   *pgxpool.Pool
	testLogger *zap.Logger
)

func TestMain(m *testing.M) {
	ctx := context.Background()

	testLogger = zap.NewNop()

	testPostgres, err := postgresHelper.SetupTestPostgres(ctx, "../../migrations/up")
	if err != nil {
		log.Fatalf("Failed to setup test database: %v", err)
	}

	testPool = testPostgres.Pool

	code := m.Run()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := testPostgres.CleanUp(ctx); err != nil {
		log.Printf("Failed to cleanup test database: %v", err)
	}

	os.Exit(code)
}
