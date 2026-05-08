package grpc

import (
	"fmt"
	"strconv"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"

	grpcutil "github.com/Kpeewu/tissi-mah/pkg/grpcutil"
	"github.com/Kpeewu/tissi-mah/services/chat-service/internal/config"
	"github.com/Kpeewu/tissi-mah/services/chat-service/internal/crypto"
	"github.com/Kpeewu/tissi-mah/services/chat-service/internal/middleware"
	svcInterfaces "github.com/Kpeewu/tissi-mah/services/chat-service/internal/service/interfaces"
	chatpb "github.com/Kpeewu/tissi-mah/services/chat-service/proto/gen"
)

// NewChatServer crée le serveur gRPC TLS du chat-service.
// Utilise un certificat self-signed auto-généré (clients via grpcutil.ClientTransportCredentials).
func NewChatServer(
	cfg *config.Config,
	svc svcInterfaces.ChatService,
	encryptor *crypto.MessageEncryptor,
	logger *zap.Logger,
) (*grpcutil.Server, error) {
	port, err := strconv.Atoi(cfg.Server.Port)
	if err != nil {
		return nil, fmt.Errorf("invalid gRPC port %q: %w", cfg.Server.Port, err)
	}

	srv, err := grpcutil.NewServer(
		grpcutil.ServerConfig{
			Port:              port,
			EnableHealthCheck: true,
			EnableReflection:  cfg.Environment.Mode != "prod",
		},
		logger,
		grpc.UnaryInterceptor(middleware.ChatInterceptor()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC server: %w", err)
	}

	chatpb.RegisterChatServiceServer(srv.Server(), NewChatHandler(svc, encryptor, logger))
	srv.SetServingStatus("chat.ChatService", grpc_health_v1.HealthCheckResponse_SERVING)

	return srv, nil
}
