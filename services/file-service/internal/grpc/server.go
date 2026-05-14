package grpc

import (
	"fmt"
	"strconv"

	grpcutil "github.com/Kpeewu/tissi-mah/pkg/grpcutil"
	"github.com/Kpeewu/tissi-mah/services/file-service/internal/client"
	"github.com/Kpeewu/tissi-mah/services/file-service/internal/config"
	"github.com/Kpeewu/tissi-mah/services/file-service/internal/middleware"
	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/file-service/internal/service/interfaces"
	"github.com/Kpeewu/tissi-mah/services/file-service/internal/storage"
	filepb "github.com/Kpeewu/tissi-mah/services/file-service/proto/gen"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
)

// NewFileServer crée et configure le serveur gRPC de file-service.
func NewFileServer(
	cfg *config.Config,
	service serviceInterfaces.FileService,
	userClient client.UserClient,
	storageClient storage.StorageClient,
	logger *zap.Logger,
) (*grpcutil.Server, error) {
	port, err := strconv.Atoi(cfg.Server.Port)
	if err != nil {
		return nil, fmt.Errorf("invalid gRPC port %q: %w", cfg.Server.Port, err)
	}

	serverCfg := grpcutil.ServerConfig{
		Port:              port,
		EnableHealthCheck: true,
		EnableReflection:  cfg.Environment.Mode != "prod",
	}

	srv, err := grpcutil.NewServer(
		serverCfg,
		logger,
		grpc.UnaryInterceptor(middleware.FileServiceInterceptor()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC server: %w", err)
	}

	handler := NewFileHandler(service, userClient, storageClient, logger)
	filepb.RegisterFileServiceServer(srv.Server(), handler)
	srv.SetServingStatus("file.FileService", grpc_health_v1.HealthCheckResponse_SERVING)

	return srv, nil
}
