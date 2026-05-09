package grpc

import (
	"fmt"
	"strconv"

	grpcutil "github.com/Kpeewu/tissi-mah/pkg/grpcutil"
	"github.com/Kpeewu/tissi-mah/services/geolocation-service/internal/config"
	"github.com/Kpeewu/tissi-mah/services/geolocation-service/internal/middleware"
	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/geolocation-service/internal/service/interfaces"
	geolocationpb "github.com/Kpeewu/tissi-mah/services/geolocation-service/proto/gen"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
)

// NewGeolocationServer crée et configure le serveur gRPC.
// Enregistre le handler GeolocationService, l'intercepteur metadata,
// le health check gRPC v1 et la reflection (hors prod).
func NewGeolocationServer(
	cfg *config.Config,
	service serviceInterfaces.GeolocationService,
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
		grpc.UnaryInterceptor(middleware.GeolocationInterceptor([]byte(cfg.InternalHMACSecret))),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC server: %w", err)
	}

	handler := NewGeolocationHandler(service, logger)
	geolocationpb.RegisterGeolocationServiceServer(srv.Server(), handler)
	srv.SetServingStatus("geolocation.GeolocationService", grpc_health_v1.HealthCheckResponse_SERVING)

	return srv, nil
}
