package e2e

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"testing"
	"time"

	postgresHelper "github.com/Kpeewu/tissi-mah/pkg-test/postgres"
	grpcHandler "github.com/Kpeewu/tissi-mah/services/file-service/internal/grpc"
	"github.com/Kpeewu/tissi-mah/services/file-service/internal/repository/implementations"
	fileService "github.com/Kpeewu/tissi-mah/services/file-service/internal/service"
	filepb "github.com/Kpeewu/tissi-mah/services/file-service/proto/gen"
	"github.com/Kpeewu/tissi-mah/services/file-service/tests/mocks"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	testPool    *pgxpool.Pool
	grpcClient  filepb.FileServiceClient
	grpcConn    *grpc.ClientConn
	mockStorage *mocks.MockStorageClient
)

func TestMain(m *testing.M) {
	ctx := context.Background()

	// --- PostgreSQL via testcontainers ---
	testPostgres, err := postgresHelper.SetupTestPostgres(ctx, "../../migrations")
	if err != nil {
		log.Fatalf("Failed to setup test database: %v", err)
	}
	testPool = testPostgres.Pool

	// --- Mock storage (S3/MinIO non requis en test) ---
	mockStorage = new(mocks.MockStorageClient)
	mockStorage.On("Upload", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return("https://storage.example.com/file.jpg", nil)
	mockStorage.On("Delete", mock.Anything, mock.Anything).Return(nil)
	mockStorage.On("GenerateURL", mock.Anything).Return("https://storage.example.com/file.jpg")

	// --- Repositories réels ---
	logger := zap.NewNop()
	userDocRead := implementations.NewUserDocumentReadRepository(testPool, logger)
	userDocWrite := implementations.NewUserDocumentWriteRepository(testPool, logger)
	vehicleDocRead := implementations.NewVehicleDocumentReadRepository(testPool, logger)
	vehicleDocWrite := implementations.NewVehicleDocumentWriteRepository(testPool, logger)
	reviewRead := implementations.NewDocumentReviewReadRepository(testPool, logger)
	reviewWrite := implementations.NewDocumentReviewWriteRepository(testPool, logger)

	// --- Service avec storage mocké ---
	svc := fileService.NewFileService(
		userDocRead, userDocWrite,
		vehicleDocRead, vehicleDocWrite,
		reviewRead, reviewWrite,
		mockStorage,
		logger,
	)

	// --- Serveur gRPC sur port dynamique ---
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	srv := grpc.NewServer()
	handler := grpcHandler.NewFileHandler(svc, logger)
	filepb.RegisterFileServiceServer(srv, handler)

	go func() {
		if err := srv.Serve(listener); err != nil {
			log.Printf("gRPC server stopped: %v", err)
		}
	}()

	// --- Client gRPC ---
	addr := listener.Addr().String()
	grpcConn, err = grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("Failed to connect to gRPC server: %v", err)
	}
	grpcClient = filepb.NewFileServiceClient(grpcConn)

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
