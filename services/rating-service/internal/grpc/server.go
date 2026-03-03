package grpc

import (
	"fmt"
	"strconv"

	grpcutil "github.com/Kpeewu/tissi-mah/pkg/grpcutil"
	"github.com/Kpeewu/tissi-mah/services/rating-service/internal/config"
	"github.com/Kpeewu/tissi-mah/services/rating-service/internal/middleware"
	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/rating-service/internal/service/interfaces"
	ratingpb "github.com/Kpeewu/tissi-mah/services/rating-service/proto/gen"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
)

// NewRatingServer crée et configure le serveur gRPC de rating-service.
// Il enregistre l'intercepteur metadata (x-firebase-uid), le handler RatingService,
// le health check gRPC v1, et la reflection (hors prod).
func NewRatingServer(
	cfg *config.Config,
	service serviceInterfaces.RatingService,
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
		grpc.UnaryInterceptor(middleware.RatingInterceptor()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC server: %w", err)
	}

	handler := NewRatingHandler(service, logger)
	ratingpb.RegisterRatingServiceServer(srv.Server(), handler)
	srv.SetServingStatus("rating.RatingService", grpc_health_v1.HealthCheckResponse_SERVING)

	return srv, nil
}
