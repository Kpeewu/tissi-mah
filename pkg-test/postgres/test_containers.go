package postgres

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	postgresContainer "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	defaultPostgresImage = "postgres:17-alpine"
	postgresUser         = "test_user"
	postgresPassword     = "test_password"
	postgresDB           = "test_db"
	postgresPort         = "5432/tcp"
)

// Option configure SetupTestPostgres.
type Option func(*testPostgresConfig)

type testPostgresConfig struct {
	image string
}

// WithImage permet de spécifier une image Docker personnalisée (ex: postgis/postgis).
func WithImage(image string) Option {
	return func(c *testPostgresConfig) {
		c.image = image
	}
}

type TestPostgres struct {
	Pool             *pgxpool.Pool
	Container        *postgresContainer.PostgresContainer
	ConnectionString string
}

func (tp *TestPostgres) CleanUp(ctx context.Context) error {
	if tp.Pool != nil {
		tp.Pool.Close()
	}
	if tp.Container != nil {
		tp.Container.Terminate(ctx)
	}

	return nil
}

func startPostgresContainer(ctx context.Context, image string) (*postgresContainer.PostgresContainer, error) {
	container, err := postgresContainer.Run(ctx,
		image,
		postgresContainer.WithDatabase(postgresDB),
		postgresContainer.WithUsername(postgresUser),
		postgresContainer.WithPassword(postgresPassword),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to start postgres container: %w", err)
	}

	return container, nil

}

func connectToPostgres(ctx context.Context, connString string) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(connString)

	if err != nil {
		return nil, fmt.Errorf("failed to parse connection string: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)

	if err != nil {
		return nil, fmt.Errorf("failed to connect to postgres: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping postgres: %w", err)
	}

	return pool, nil
}

// runMigrations exécute tous les fichiers *.up.sql du répertoire donné dans l'ordre alphabétique.
// Compatible avec la convention golang-migrate (NNNNNN_name.up.sql / .down.sql).
func runMigrations(ctx context.Context, pool *pgxpool.Pool, migrationPath string) error {
	absPath, err := filepath.Abs(migrationPath)
	if err != nil {
		return fmt.Errorf("failed to resolve migration path: %w", err)
	}

	entries, err := os.ReadDir(absPath)
	if err != nil {
		return fmt.Errorf("failed to read migration directory %s: %w", absPath, err)
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".up.sql") {
			files = append(files, filepath.Join(absPath, e.Name()))
		}
	}
	sort.Strings(files)

	if len(files) == 0 {
		return fmt.Errorf("no .sql files found in %s", absPath)
	}

	for _, f := range files {
		sql, err := os.ReadFile(f)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", f, err)
		}
		if _, err := pool.Exec(ctx, string(sql)); err != nil {
			return fmt.Errorf("failed to execute %s: %w", filepath.Base(f), err)
		}
	}

	return nil
}

func SetupTestPostgres(ctx context.Context, migrationsPath string, opts ...Option) (*TestPostgres, error) {
	cfg := &testPostgresConfig{image: defaultPostgresImage}
	for _, o := range opts {
		o(cfg)
	}

	container, err := startPostgresContainer(ctx, cfg.image)

	if err != nil {
		return nil, err
	}

	connString, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		container.Terminate(ctx)
		return nil, fmt.Errorf("failed to get connection string: %w", err)
	}

	pool, err := connectToPostgres(ctx, connString)

	if err != nil {
		container.Terminate(ctx)
		return nil, err
	}

	if err := runMigrations(ctx, pool, migrationsPath); err != nil {
		pool.Close()
		container.Terminate(ctx)
		return nil, err
	}

	return &TestPostgres{
		Pool:             pool,
		Container:        container,
		ConnectionString: connString,
	}, nil
}
