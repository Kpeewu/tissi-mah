package grpc

import (
	"fmt"
	"strconv"

	grpcutil "github.com/Kpeewu/tissi-mah/pkg/grpcutil"
	"github.com/Kpeewu/tissi-mah/services/moderation-service/internal/config"
	"github.com/Kpeewu/tissi-mah/services/moderation-service/internal/middleware"
	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/moderation-service/internal/service/interfaces"
	moderationpb "github.com/Kpeewu/tissi-mah/services/moderation-service/proto/gen"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
)

func NewModerationServer(cfg *config.Config, svc serviceInterfaces.ModerationService, logger *zap.Logger) (*grpcutil.Server, error) {
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
		grpc.UnaryInterceptor(middleware.ModerationInterceptor()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC server: %w", err)
	}

	handler := NewModerationHandler(svc, logger)
	moderationpb.RegisterModerationServiceServer(srv.Server(), handler)
	srv.SetServingStatus("moderation.ModerationService", grpc_health_v1.HealthCheckResponse_SERVING)

	return srv, nil
}
