package client

import (
	"context"
	"fmt"

	authpb "github.com/Kpeewu/tissi-mah/services/user-service/proto/gen/authpb"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// AuthInfo contient les données d'authentification récupérées depuis auth-service.
type AuthInfo struct {
	AuthID            string
	Email             string
	PhoneNumber       string
	IsActive          bool
	IsSuspended       bool
	SuspensionEndDate string
}

// AuthServiceClient est le client gRPC vers auth-service.
// Utilise insecure.NewCredentials() pour la communication intra-cluster
// (le chiffrement est géré au niveau du service mesh / mTLS Kubernetes).
type AuthServiceClient struct {
	conn       *grpc.ClientConn
	grpcClient authpb.AuthServiceClient
	logger     *zap.Logger
}

// NewAuthServiceClient établit la connexion gRPC vers auth-service.
// address doit être au format "host:port" (ex: "auth-service:50051").
func NewAuthServiceClient(address string, logger *zap.Logger) (*AuthServiceClient, error) {
	log := logger.Named("auth-client")
	log.Debug("connexion au service auth", zap.String("address", address))

	conn, err := grpc.NewClient(
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Error("échec de la connexion au service auth", zap.Error(err), zap.String("address", address))
		return nil, fmt.Errorf("auth-service: failed to connect to %s: %w", address, err)
	}

	return &AuthServiceClient{
		conn:       conn,
		grpcClient: authpb.NewAuthServiceClient(conn),
		logger:     log,
	}, nil
}

// Close libère la connexion gRPC. À appeler au shutdown du service.
func (c *AuthServiceClient) Close() error {
	return c.conn.Close()
}

// GetAuthInfo récupère les données d'authentification d'un utilisateur par son AuthID.
func (c *AuthServiceClient) GetAuthInfo(ctx context.Context, authID string) (*AuthInfo, error) {
	c.logger.Debug("récupération des données auth", zap.String("auth_id", authID))

	resp, err := c.grpcClient.GetAuthInfo(ctx, &authpb.GetAuthInfoRequest{
		AuthID: authID,
	})
	if err != nil {
		c.logger.Error("échec de GetAuthInfo", zap.Error(err), zap.String("auth_id", authID))
		return nil, fmt.Errorf("auth-service: GetAuthInfo failed: %w", err)
	}

	c.logger.Debug("données auth récupérées avec succès", zap.String("auth_id", authID))
	return &AuthInfo{
		AuthID:            resp.AuthID,
		Email:             resp.Email,
		PhoneNumber:       resp.PhoneNumber,
		IsActive:          resp.IsActive,
		IsSuspended:       resp.IsSuspended,
		SuspensionEndDate: resp.SuspensionEndDate,
	}, nil
}
