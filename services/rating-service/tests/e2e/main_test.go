package e2e

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	postgresHelper "github.com/Kpeewu/tissi-mah/pkg-test/postgres"
	grpcHandler "github.com/Kpeewu/tissi-mah/services/rating-service/internal/grpc"
	"github.com/Kpeewu/tissi-mah/services/rating-service/internal/client"
	"github.com/Kpeewu/tissi-mah/services/rating-service/internal/middleware"
	"github.com/Kpeewu/tissi-mah/services/rating-service/internal/repository/implementations"
	"github.com/Kpeewu/tissi-mah/services/rating-service/internal/service"
	ratingpb "github.com/Kpeewu/tissi-mah/services/rating-service/proto/gen"
	"github.com/Kpeewu/tissi-mah/services/rating-service/tests/mocks"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	testPool       *pgxpool.Pool
	grpcClient     ratingpb.RatingServiceClient
	grpcConn       *grpc.ClientConn
	mockUserClient *mocks.MockUserClient
)

func TestMain(m *testing.M) {
	ctx := context.Background()

	// --- PostgreSQL via testcontainers ---
	testPostgres, err := postgresHelper.SetupTestPostgres(ctx, "../../migrations")
	if err != nil {
		log.Printf("SetupTestPostgres returned error: %v, trying manual setup...", err)
	}
	if testPostgres == nil {
		log.Fatal("Failed to setup test database")
	}
	testPool = testPostgres.Pool

	// Vérifier si la table ratings existe, sinon appliquer manuellement
	var tableExists bool
	err = testPool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_name = 'ratings')").Scan(&tableExists)
	if err != nil {
		log.Fatalf("Failed to check table existence: %v", err)
	}
	if !tableExists {
		log.Println("Table 'ratings' not found, applying migrations manually...")
		if err := applyMigrationsManually(ctx, testPool, "../../migrations"); err != nil {
			log.Fatalf("Failed to apply migrations manually: %v", err)
		}
	}

	// --- Mock user-service client (toujours valide par défaut) ---
	mockUserClient = new(mocks.MockUserClient)
	mockUserClient.On("UserExists", mock.Anything, mock.Anything).Return(true, nil)
	mockUserClient.On("GetUsersByUserIDs", mock.Anything, mock.Anything).Return(map[string]*client.UserProfile{}, nil)
	mockUserClient.On("Close").Return(nil)

	// --- Service layer ---
	logger := zap.NewNop()
	readRepo := implementations.NewRatingReadRepository(testPool, logger)
	writeRepo := implementations.NewRatingWriteRepository(testPool, logger)
	ratingService := service.NewRatingService(readRepo, writeRepo, mockUserClient, nil, nil, logger)

	// --- gRPC server sur port dynamique ---
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	srv := grpc.NewServer(grpc.UnaryInterceptor(middleware.RatingInterceptor(nil)))
	handler := grpcHandler.NewRatingHandler(ratingService, logger)
	ratingpb.RegisterRatingServiceServer(srv, handler)

	go func() {
		if err := srv.Serve(listener); err != nil {
			log.Printf("gRPC server stopped: %v", err)
		}
	}()

	// --- gRPC client ---
	addr := listener.Addr().String()
	grpcConn, err = grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("Failed to connect to gRPC server: %v", err)
	}
	grpcClient = ratingpb.NewRatingServiceClient(grpcConn)

	fmt.Printf("E2E test server running on %s\n", addr)

	// --- Run tests ---
	code := m.Run()

	// --- Cleanup ---
	grpcConn.Close()
	srv.GracefulStop()

	cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := testPostgres.CleanUp(cleanupCtx); err != nil {
		log.Printf("Failed to cleanup test database: %v", err)
	}

	os.Exit(code)
}

// applyMigrationsManually lit et exécute les fichiers SQL du répertoire de migrations
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
