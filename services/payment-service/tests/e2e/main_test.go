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
	grpcHandler "github.com/Kpeewu/tissi-mah/services/payment-service/internal/grpc"
	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/middleware"
	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/repository/implementations"
	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/service"
	paymentpb "github.com/Kpeewu/tissi-mah/services/payment-service/proto/gen"
	"github.com/Kpeewu/tissi-mah/services/payment-service/tests/mocks"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/config"
)

var (
	testPool          *pgxpool.Pool
	grpcClient        paymentpb.PaymentServiceClient
	grpcConn          *grpc.ClientConn
	mockSupportClient *mocks.MockSupportClient
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

	// Verifier si la table payments existe, sinon appliquer manuellement
	var tableExists bool
	err = testPool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_name = 'payments')").Scan(&tableExists)
	if err != nil {
		log.Fatalf("Failed to check table existence: %v", err)
	}
	if !tableExists {
		log.Println("Table 'payments' not found, applying migrations manually...")
		if err := applyMigrationsManually(ctx, testPool, "../../migrations"); err != nil {
			log.Fatalf("Failed to apply migrations manually: %v", err)
		}
	}

	// --- Mock clients (inter-service) ---
	mockBookingClient := new(mocks.MockBookingClient)
	mockBookingClient.On("ConfirmPayment", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	mockBookingClient.On("GetBookingDetails", mock.Anything, mock.Anything).Return(nil, nil).Maybe()

	mockUserClient := new(mocks.MockUserClient)
	mockUserClient.On("GetUserByUserID", mock.Anything, mock.Anything).Return(nil, nil).Maybe()

	mockSupportClient = new(mocks.MockSupportClient)

	// --- Service layer ---
	logger := zap.NewNop()
	paymentReadRepo := implementations.NewPaymentReadRepository(testPool, logger)
	paymentWriteRepo := implementations.NewPaymentWriteRepository(testPool, logger)
	refundReadRepo := implementations.NewRefundReadRepository(testPool, logger)
	refundWriteRepo := implementations.NewRefundWriteRepository(testPool, logger)
	payoutReadRepo := implementations.NewPayoutReadRepository(testPool, logger)
	payoutWriteRepo := implementations.NewPayoutWriteRepository(testPool, logger)
	payoutHistoryRepo := implementations.NewPayoutHistoryWriteRepository(testPool, logger)

	cfg := &config.Config{
		Refund: config.RefundConfig{
			CancellationFullRefundHours:    24,
			CancellationGracePeriodMinutes: 30,
			NoShowDriverDelayMinutes:       15,
			NoShowPassengerDelayMinutes:    15,
		},
		Payout: config.PayoutConfig{
			IntervalSeconds:        1800,
			ContestationDelayHours: 2,
			PlatformFeePercent:     10,
		},
	}

	paymentService := service.NewPaymentService(
		paymentReadRepo, paymentWriteRepo,
		refundReadRepo, refundWriteRepo,
		payoutReadRepo, payoutWriteRepo,
		payoutHistoryRepo,
		mockBookingClient, mockUserClient,
		mockSupportClient,
		nil, // fedapayClient
		nil, // cache
		nil, // notifRedis
		cfg, logger,
	)

	// --- gRPC server sur port dynamique ---
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	srv := grpc.NewServer(grpc.UnaryInterceptor(middleware.PaymentInterceptor()))
	handler := grpcHandler.NewPaymentHandler(paymentService, logger)
	paymentpb.RegisterPaymentServiceServer(srv, handler)

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
	grpcClient = paymentpb.NewPaymentServiceClient(grpcConn)

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
