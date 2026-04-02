package grpc

import (
	"fmt"
	"strconv"

	grpcutil "github.com/Kpeewu/tissi-mah/pkg/grpcutil"
	"github.com/Kpeewu/tissi-mah/services/email-service/internal/config"
	"github.com/Kpeewu/tissi-mah/services/email-service/internal/middleware"
	"github.com/Kpeewu/tissi-mah/services/email-service/internal/provider"
	emailpb "github.com/Kpeewu/tissi-mah/services/email-service/proto/gen"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
)

// NewEmailServer crée et configure le serveur gRPC de email-service.
func NewEmailServer(
	cfg *config.Config,
	emailProvider provider.EmailProvider,
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
		grpc.UnaryInterceptor(middleware.EmailInterceptor(logger)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC server: %w", err)
	}

	handler := NewEmailHandler(emailProvider, logger)
	emailpb.RegisterEmailServiceServer(srv.Server(), handler)
	srv.SetServingStatus("email.EmailService", grpc_health_v1.HealthCheckResponse_SERVING)

	return srv, nil
}
