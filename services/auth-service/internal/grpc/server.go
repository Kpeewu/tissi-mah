package grpc

import (
	"fmt"
	"strconv"

	grpcutil "github.com/Kpeewu/tissi-mah/pkg/grpcutil"
	"github.com/Kpeewu/tissi-mah/services/auth-service/internal/config"
	"github.com/Kpeewu/tissi-mah/services/auth-service/internal/middleware"
	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/auth-service/internal/service/interfaces"
	firebaseValidator "github.com/Kpeewu/tissi-mah/services/auth-service/pkg/firebase"
	authpb "github.com/Kpeewu/tissi-mah/services/auth-service/proto/gen"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
)

// NewAuthServer crée et configure le serveur gRPC de auth-service.
// Il enregistre l'intercepteur JWT, le handler AuthService,
// le health check gRPC v1, et la reflection (hors prod).
func NewAuthServer(
	cfg *config.Config,
	service serviceInterfaces.AuthService,
	validator *firebaseValidator.JWTValidator,
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
		grpc.UnaryInterceptor(middleware.AuthInterceptor(validator)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC server: %w", err)
	}

	handler := NewAuthHandler(service)
	authpb.RegisterAuthServiceServer(srv.Server(), handler)
	srv.SetServingStatus("auth.AuthService", grpc_health_v1.HealthCheckResponse_SERVING)

	return srv, nil
}
