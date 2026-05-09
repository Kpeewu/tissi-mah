package grpc

import (
	"fmt"
	"strconv"

	grpcutil "github.com/Kpeewu/tissi-mah/pkg/grpcutil"
	"github.com/Kpeewu/tissi-mah/services/booking-service/internal/config"
	"github.com/Kpeewu/tissi-mah/services/booking-service/internal/middleware"
	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/booking-service/internal/service/interfaces"
	bookingpb "github.com/Kpeewu/tissi-mah/services/booking-service/proto/gen"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
)

// NewBookingServer crée et configure le serveur gRPC de booking-service.
func NewBookingServer(
	cfg *config.Config,
	service serviceInterfaces.BookingService,
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
		grpc.UnaryInterceptor(middleware.BookingInterceptor([]byte(cfg.InternalHMACSecret))),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC server: %w", err)
	}

	handler := NewBookingHandler(service, logger)
	bookingpb.RegisterBookingServiceServer(srv.Server(), handler)
	srv.SetServingStatus("booking.BookingService", grpc_health_v1.HealthCheckResponse_SERVING)

	return srv, nil
}
