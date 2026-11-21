package client

import (
	"context"
	"fmt"

	"github.com/Kpeewu/tissi-mah/pkg/grpcutil"
	"github.com/Kpeewu/tissi-mah/services/kyc-service/internal/domain"
	supportpb "github.com/Kpeewu/tissi-mah/services/support-service/proto/gen"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// SupportServiceClient est le client gRPC vers support-service.
// Utilise TLS avec skip-verify pour la communication intra-cluster.
type SupportServiceClient struct {
	conn       *grpc.ClientConn
	grpcClient supportpb.SupportServiceClient
	logger     *zap.Logger
}

// NewSupportServiceClient établit la connexion gRPC vers support-service.
// address doit être au format "host:port" (ex: "support-service:50063").
func NewSupportServiceClient(address string, logger *zap.Logger) (*SupportServiceClient, error) {
	logger.Debug("connecting to support-service", zap.String("address", address))

	conn, err := grpc.NewClient(
		address,
		grpc.WithTransportCredentials(grpcutil.ClientTransportCredentials()),
	)
	if err != nil {
		logger.Error("failed to connect to support-service", zap.Error(err), zap.String("address", address))
		return nil, fmt.Errorf("support-service: failed to connect to %s: %w", address, err)
	}

	return &SupportServiceClient{
		conn:       conn,
		grpcClient: supportpb.NewSupportServiceClient(conn),
		logger:     logger,
	}, nil
}

// Close libère la connexion gRPC. À appeler au shutdown du service.
func (c *SupportServiceClient) Close() error {
	return c.conn.Close()
}

// GetSupportUserByID récupère le prénom/nom/rôle d'un agent support par son UID.
func (c *SupportServiceClient) GetSupportUserByID(ctx context.Context, userID string) (*domain.SupportAgent, error) {
	c.logger.Debug("client: GetSupportUserByID called", zap.String("userID", userID))

	resp, err := c.grpcClient.GetSupportUserByID(ctx, &supportpb.GetSupportUserByIDRequest{
		UserId: userID,
	})
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
			return nil, fmt.Errorf("support-service: agent not found for userID %s", userID)
		}
		c.logger.Error("client: GetSupportUserByID failed", zap.Error(err), zap.String("userID", userID))
		return nil, fmt.Errorf("support-service: GetSupportUserByID failed: %w", err)
	}

	return &domain.SupportAgent{
		UserID:    resp.GetUserId(),
		FirstName: resp.GetFirstName(),
		LastName:  resp.GetLastName(),
		Role:      resp.GetRole(),
	}, nil
}
