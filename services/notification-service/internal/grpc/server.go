package grpc

import (
	"fmt"
	"strconv"

	grpcutil "github.com/Kpeewu/tissi-mah/pkg/grpcutil"
	"github.com/Kpeewu/tissi-mah/services/notification-service/internal/config"
	"github.com/Kpeewu/tissi-mah/services/notification-service/internal/middleware"
	svcInterfaces "github.com/Kpeewu/tissi-mah/services/notification-service/internal/service/interfaces"
	notifpb "github.com/Kpeewu/tissi-mah/services/notification-service/proto/gen"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
)

func NewNotificationServer(
	cfg *config.Config,
	service svcInterfaces.NotificationService,
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
		grpc.UnaryInterceptor(middleware.NotificationInterceptor(logger)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC server: %w", err)
	}

	handler := NewNotificationHandler(service, logger)
	notifpb.RegisterNotificationServiceServer(srv.Server(), handler)
	srv.SetServingStatus("notification.NotificationService", grpc_health_v1.HealthCheckResponse_SERVING)

	return srv, nil
}
