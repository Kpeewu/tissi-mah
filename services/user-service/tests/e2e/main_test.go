package e2e

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"testing"

	mongoHelper "github.com/Kpeewu/tissi-mah/pkg-test/mongodb"
	grpcHandler "github.com/Kpeewu/tissi-mah/services/user-service/internal/grpc"
	"github.com/Kpeewu/tissi-mah/services/user-service/internal/middleware"
	"github.com/Kpeewu/tissi-mah/services/user-service/internal/repository/implementations"
	userService "github.com/Kpeewu/tissi-mah/services/user-service/internal/service"
	userpb "github.com/Kpeewu/tissi-mah/services/user-service/proto/gen"
	"github.com/Kpeewu/tissi-mah/services/user-service/tests/mocks"
	"github.com/stretchr/testify/mock"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	grpcClient     userpb.UserServiceClient
	grpcConn       *grpc.ClientConn
	mockAuthClient *mocks.MockAuthClient
	mockFileClient *mocks.MockFileClient
	testCollection *mongo.Collection
)

func TestMain(m *testing.M) {
	ctx := context.Background()

	// --- MongoDB via testcontainers ---
	testMongo, err := mongoHelper.SetupTestMongoDB(ctx)
	if err != nil {
		log.Fatalf("Failed to setup test MongoDB: %v", err)
	}
	testCollection = testMongo.Database().Collection("users")

	// Créer les index nécessaires
	if err := implementations.EnsureIndexes(ctx, testCollection); err != nil {
		log.Fatalf("Failed to ensure indexes: %v", err)
	}

	// --- Mock auth client ---
	mockAuthClient = new(mocks.MockAuthClient)

	// --- Mock file client ---
	mockFileClient = new(mocks.MockFileClient)
	mockFileClient.On("GetDocumentExpiry", mock.Anything, mock.Anything, mock.Anything).Return("").Maybe()

	// --- Repositories réels ---
	logger := zap.NewNop()
	readRepo := implementations.NewUserReadRepository(testCollection, logger)
	writeRepo := implementations.NewUserWriteRepository(testCollection, logger)

	// --- Service ---
	svc := userService.NewUserService(readRepo, writeRepo, mockAuthClient, mockFileClient, logger)

	// --- Serveur gRPC sur port dynamique ---
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	srv := grpc.NewServer(grpc.UnaryInterceptor(middleware.AuthInterceptor(nil)))
	handler := grpcHandler.NewUserHandler(svc, logger)
	userpb.RegisterUserServiceServer(srv, handler)

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
	grpcClient = userpb.NewUserServiceClient(grpcConn)

	fmt.Printf("E2E test server running on %s\n", addr)

	// --- Run tests ---
	code := m.Run()

	// --- Cleanup ---
	grpcConn.Close()
	srv.GracefulStop()
	testMongo.CleanUp(ctx)

	os.Exit(code)
}
