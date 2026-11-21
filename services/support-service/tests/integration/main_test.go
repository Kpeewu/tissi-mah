package integration

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	postgresHelper "github.com/Kpeewu/tissi-mah/pkg-test/postgres"
	"github.com/alicebob/miniredis/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

var (
	testPool    *pgxpool.Pool
	testRedis   *redis.Client
	testMiniSrv *miniredis.Miniredis
)

func TestMain(m *testing.M) {
	ctx := context.Background()

	testPostgres, err := postgresHelper.SetupTestPostgres(ctx, "../../migrations")
	if err != nil {
		log.Printf("SetupTestPostgres returned error: %v, trying manual setup...", err)
	}
	if testPostgres == nil {
		log.Fatal("Failed to setup test database")
	}
	testPool = testPostgres.Pool

	var tableExists bool
	if err := testPool.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_name = 'support_users')",
	).Scan(&tableExists); err != nil {
		log.Fatalf("check table existence: %v", err)
	}
	if !tableExists {
		if err := applyUpMigrations(ctx, testPool, "../../migrations"); err != nil {
			log.Fatalf("apply migrations: %v", err)
		}
	}

	// Miniredis suffit pour otp/refresh store : même API que Redis.
	testMiniSrv, err = miniredis.Run()
	if err != nil {
		log.Fatalf("miniredis: %v", err)
	}
	testRedis = redis.NewClient(&redis.Options{Addr: testMiniSrv.Addr()})

	code := m.Run()

	_ = testRedis.Close()
	testMiniSrv.Close()
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = testPostgres.CleanUp(cleanupCtx)
	os.Exit(code)
}

func applyUpMigrations(ctx context.Context, pool *pgxpool.Pool, dir string) error {
	files, err := filepath.Glob(filepath.Join(dir, "*.up.sql"))
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

// cleanTables vide support_users (sauf le seed admin) entre sous-tests.
func cleanTables(t *testing.T) {
	t.Helper()
	_, err := testPool.Exec(context.Background(), "DELETE FROM support_users")
	if err != nil {
		t.Fatalf("cleanTables: %v", err)
	}
	testMiniSrv.FlushAll()
}

// isUniqueErr détecte une violation d'unicité pgx sans importer le package complet dans chaque test.
func isUniqueErr(err error) bool {
	return err != nil && strings.Contains(err.Error(), "23505")
}
