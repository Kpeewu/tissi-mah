package grpcsrv

import (
	"fmt"
	"strconv"

	grpcutil "github.com/Kpeewu/tissi-mah/pkg/grpcutil"
	"github.com/Kpeewu/tissi-mah/services/support-service/internal/config"
	"github.com/Kpeewu/tissi-mah/services/support-service/internal/middleware"
	svcIfaces "github.com/Kpeewu/tissi-mah/services/support-service/internal/service/interfaces"
	supportpb "github.com/Kpeewu/tissi-mah/services/support-service/proto/gen"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
)

// NewSupportServer crée et configure le serveur gRPC support-service.
func NewSupportServer(
	cfg *config.Config,
	service svcIfaces.SupportService,
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
		grpc.UnaryInterceptor(middleware.SupportInterceptor()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC server: %w", err)
	}

	handler := NewSupportHandler(service, logger)
	supportpb.RegisterSupportServiceServer(srv.Server(), handler)
	srv.SetServingStatus("support.SupportService", grpc_health_v1.HealthCheckResponse_SERVING)

	return srv, nil
}
