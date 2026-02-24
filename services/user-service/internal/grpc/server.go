package grpc

import (
	"fmt"
	"strconv"

	grpcutil "github.com/Kpeewu/tissi-mah/pkg/grpcutil"
	"github.com/Kpeewu/tissi-mah/services/user-service/internal/config"
	"github.com/Kpeewu/tissi-mah/services/user-service/internal/middleware"
	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/user-service/internal/service/interfaces"
	userpb "github.com/Kpeewu/tissi-mah/services/user-service/proto/gen"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
)

// NewUserServer crée et configure le serveur gRPC de user-service.
// Il enregistre l'intercepteur metadata (x-firebase-uid), le handler UserService,
// le health check gRPC v1, et la reflection (hors prod).
func NewUserServer(
	cfg *config.Config,
	service serviceInterfaces.UserService,
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
		grpc.UnaryInterceptor(middleware.AuthInterceptor()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC server: %w", err)
	}

	handler := NewUserHandler(service)
	userpb.RegisterUserServiceServer(srv.Server(), handler)
	srv.SetServingStatus("user.UserService", grpc_health_v1.HealthCheckResponse_SERVING)

	return srv, nil
}
