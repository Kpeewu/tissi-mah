package grpc

import (
	"fmt"
	"strconv"

	grpcutil "github.com/Kpeewu/tissi-mah/pkg/grpcutil"
	"github.com/Kpeewu/tissi-mah/services/vehicle-service/internal/config"
	"github.com/Kpeewu/tissi-mah/services/vehicle-service/internal/middleware"
	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/vehicle-service/internal/service/interfaces"
	vehiclepb "github.com/Kpeewu/tissi-mah/services/vehicle-service/proto/gen"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
)

// NewVehicleServer crée et configure le serveur gRPC de vehicle-service.
// Il enregistre l'intercepteur metadata (x-firebase-uid), le handler VehicleService,
// le health check gRPC v1, et la reflection (hors prod).
func NewVehicleServer(
	cfg *config.Config,
	service serviceInterfaces.VehicleService,
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
		grpc.UnaryInterceptor(middleware.VehicleInterceptor()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC server: %w", err)
	}

	handler := NewVehicleHandler(service, logger)
	vehiclepb.RegisterVehicleServiceServer(srv.Server(), handler)
	srv.SetServingStatus("vehicle.VehicleService", grpc_health_v1.HealthCheckResponse_SERVING)

	return srv, nil
}
