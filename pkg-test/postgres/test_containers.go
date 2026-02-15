package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	postgresContainer "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	postgresImage    = "postgres:17-alpine"
	postgresUser     = "test_user"
	postgresPassword = "test_password"
	postgresDB       = "test_db"
	postgresPort     = "5432/tcp"
)

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

func startPostgresContainer(ctx context.Context) (*postgresContainer.PostgresContainer, error) {
	container, err := postgresContainer.Run(ctx,
		postgresImage,
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

func runMigrations(connString string, migrationPath string) error {

	m, err := migrate.New("file://"+migrationPath, connString)

	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}

	defer m.Close()

	if err := m.Up(); err != nil && err == migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

func SetupTestPostgres(ctx context.Context, migrationsPath string) (*TestPostgres, error) {
	container, err := startPostgresContainer(ctx)

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

	if err := runMigrations(connString, migrationsPath); err != nil {
		pool.Close()
		container.Terminate(ctx)
		return nil, err
	}

	// 5. Retourner la struct TestPostgres
	return &TestPostgres{
		Pool:             pool,
		Container:        container,
		ConnectionString: connString,
	}, nil
}
