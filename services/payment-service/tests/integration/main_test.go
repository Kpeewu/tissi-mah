package integration

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"sort"
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

	// Setup : Demarrer PostgreSQL via testcontainers
	testPostgres, err := postgresHelper.SetupTestPostgres(ctx, "../../migrations/up")
	if err != nil {
		log.Printf("SetupTestPostgres returned error: %v, trying manual setup...", err)
	}

	if testPostgres == nil {
		log.Fatal("Failed to setup test database")
	}

	testPool = testPostgres.Pool

	// Verifier si la table payments existe, sinon appliquer les migrations manuellement
	var tableExists bool
	err = testPool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_name = 'payments')").Scan(&tableExists)
	if err != nil {
		log.Fatalf("Failed to check table existence: %v", err)
	}

	if !tableExists {
		log.Println("Table 'payments' not found, applying migrations manually...")
		if err := applyMigrationsManually(ctx, testPool, "../../migrations/up"); err != nil {
			log.Fatalf("Failed to apply migrations manually: %v", err)
		}
		log.Println("Migrations applied successfully")
	}

	// Executer tous les tests
	code := m.Run()

	// Cleanup
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := testPostgres.CleanUp(cleanupCtx); err != nil {
		log.Printf("Failed to cleanup test database: %v", err)
	}

	os.Exit(code)
}

// applyMigrationsManually lit et execute les fichiers SQL du repertoire de migrations
func applyMigrationsManually(ctx context.Context, pool *pgxpool.Pool, migrationsDir string) error {
	files, err := filepath.Glob(filepath.Join(migrationsDir, "*.sql"))
	if err != nil {
		return err
	}

	sort.Strings(files)

	for _, f := range files {
		sql, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		if _, err := pool.Exec(ctx, string(sql)); err != nil {
			return err
		}
		log.Printf("Applied migration: %s", filepath.Base(f))
	}

	return nil
}
