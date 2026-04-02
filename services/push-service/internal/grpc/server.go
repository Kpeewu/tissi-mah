package grpc

import (
	"fmt"
	"strconv"

	grpcutil "github.com/Kpeewu/tissi-mah/pkg/grpcutil"
	"github.com/Kpeewu/tissi-mah/services/push-service/internal/config"
	"github.com/Kpeewu/tissi-mah/services/push-service/internal/fcm"
	"github.com/Kpeewu/tissi-mah/services/push-service/internal/middleware"
	pushpb "github.com/Kpeewu/tissi-mah/services/push-service/proto/gen"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
)

// NewPushServer crée et configure le serveur gRPC de push-service.
func NewPushServer(
	cfg *config.Config,
	fcmClient fcm.FCMClient,
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
		grpc.UnaryInterceptor(middleware.PushInterceptor(logger)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC server: %w", err)
	}

	handler := NewPushHandler(fcmClient, logger)
	pushpb.RegisterPushServiceServer(srv.Server(), handler)
	srv.SetServingStatus("push.PushService", grpc_health_v1.HealthCheckResponse_SERVING)

	return srv, nil
}
