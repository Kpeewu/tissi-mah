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
	grpcHandler "github.com/Kpeewu/tissi-mah/services/booking-service/internal/grpc"
	"github.com/Kpeewu/tissi-mah/services/booking-service/internal/repository/implementations"
	"github.com/Kpeewu/tissi-mah/services/booking-service/internal/service"
	bookingpb "github.com/Kpeewu/tissi-mah/services/booking-service/proto/gen"
	"github.com/Kpeewu/tissi-mah/services/booking-service/tests/mocks"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	testPool   *pgxpool.Pool
	grpcClient bookingpb.BookingServiceClient
	grpcConn   *grpc.ClientConn
)

func TestMain(m *testing.M) {
	ctx := context.Background()

	// --- PostgreSQL via testcontainers ---
	testPostgres, err := postgresHelper.SetupTestPostgres(ctx, "../../migrations/up")
	if err != nil {
		log.Printf("SetupTestPostgres returned error: %v, trying manual setup...", err)
	}
	if testPostgres == nil {
		log.Fatal("Failed to setup test database")
	}
	testPool = testPostgres.Pool

	// Verifier si la table bookings existe, sinon appliquer manuellement
	var tableExists bool
	err = testPool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_name = 'bookings')").Scan(&tableExists)
	if err != nil {
		log.Fatalf("Failed to check table existence: %v", err)
	}
	if !tableExists {
		log.Println("Table 'bookings' not found, applying migrations manually...")
		if err := applyMigrationsManually(ctx, testPool, "../../migrations/up"); err != nil {
			log.Fatalf("Failed to apply migrations manually: %v", err)
		}
	}

	// --- Mock clients (inter-service) ---
	mockTripClient := new(mocks.MockTripClient)
	mockTripClient.On("GetTripDetails", mock.Anything, mock.Anything).Return(nil, nil).Maybe()
	mockTripClient.On("UpdateAvailableSeats", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	mockUserClient := new(mocks.MockUserClient)
	mockUserClient.On("UserExists", mock.Anything, mock.Anything).Return(true, nil).Maybe()

	mockPaymentClient := new(mocks.MockPaymentClient)
	mockPaymentClient.On("RequestRefund", mock.Anything, mock.Anything).Return(nil).Maybe()
	mockPaymentClient.On("ReleasePayment", mock.Anything, mock.Anything).Return(nil).Maybe()

	// --- Service layer ---
	logger := zap.NewNop()
	readRepo := implementations.NewBookingReadRepository(testPool, logger)
	writeRepo := implementations.NewBookingWriteRepository(testPool, logger)

	bookingService := service.NewBookingService(
		readRepo, writeRepo,
		mockTripClient, mockUserClient, mockPaymentClient,
		nil, // cache
		10,  // serviceFeePercent
		logger,
	)

	// --- gRPC server sur port dynamique ---
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	srv := grpc.NewServer()
	handler := grpcHandler.NewBookingHandler(bookingService, logger)
	bookingpb.RegisterBookingServiceServer(srv, handler)

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
	grpcClient = bookingpb.NewBookingServiceClient(grpcConn)

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
