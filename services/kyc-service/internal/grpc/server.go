package grpc

import (
	"fmt"
	"strconv"

	grpcutil "github.com/Kpeewu/tissi-mah/pkg/grpcutil"
	"github.com/Kpeewu/tissi-mah/services/kyc-service/internal/config"
	"github.com/Kpeewu/tissi-mah/services/kyc-service/internal/middleware"
	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/kyc-service/internal/service/interfaces"
	kycpb "github.com/Kpeewu/tissi-mah/services/kyc-service/proto/gen"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
)

// NewKYCServer crée et configure le serveur gRPC de kyc-service.
// Il enregistre l'intercepteur metadata (x-firebase-uid), le handler KYCService,
// le health check gRPC v1, et la reflection (hors prod).
func NewKYCServer(
	cfg *config.Config,
	service serviceInterfaces.KYCService,
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
		grpc.UnaryInterceptor(middleware.KYCInterceptor([]byte(cfg.InternalHMACSecret))),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC server: %w", err)
	}

	handler := NewKYCHandler(service, logger)
	kycpb.RegisterKYCServiceServer(srv.Server(), handler)
	srv.SetServingStatus("kyc.KYCService", grpc_health_v1.HealthCheckResponse_SERVING)

	return srv, nil
}
