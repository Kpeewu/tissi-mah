package grpc

import (
	"fmt"
	"strconv"

	grpcutil "github.com/Kpeewu/tissi-mah/pkg/grpcutil"
	"github.com/Kpeewu/tissi-mah/services/trips-service/internal/config"
	"github.com/Kpeewu/tissi-mah/services/trips-service/internal/middleware"
	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/trips-service/internal/service/interfaces"
	trippb "github.com/Kpeewu/tissi-mah/services/trips-service/proto/gen"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
)

// NewTripServer crée et configure le serveur gRPC de trips-service.
func NewTripServer(
	cfg *config.Config,
	service serviceInterfaces.TripService,
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
		grpc.UnaryInterceptor(middleware.TripInterceptor([]byte(cfg.InternalHMACSecret))),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC server: %w", err)
	}

	handler := NewTripHandler(service, logger)
	trippb.RegisterTripServiceServer(srv.Server(), handler)
	srv.SetServingStatus("trip.TripService", grpc_health_v1.HealthCheckResponse_SERVING)

	return srv, nil
}
