package client

import (
	"context"
	"fmt"

	authpb "github.com/Kpeewu/tissi-mah/services/user-service/proto/gen/authpb"
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
}

// NewAuthServiceClient établit la connexion gRPC vers auth-service.
// address doit être au format "host:port" (ex: "auth-service:50051").
func NewAuthServiceClient(address string) (*AuthServiceClient, error) {
	conn, err := grpc.NewClient(
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("auth-service: failed to connect to %s: %w", address, err)
	}

	return &AuthServiceClient{
		conn:       conn,
		grpcClient: authpb.NewAuthServiceClient(conn),
	}, nil
}

// Close libère la connexion gRPC. À appeler au shutdown du service.
func (c *AuthServiceClient) Close() error {
	return c.conn.Close()
}

// GetAuthInfo récupère les données d'authentification d'un utilisateur par son AuthID.
func (c *AuthServiceClient) GetAuthInfo(ctx context.Context, authID string) (*AuthInfo, error) {
	resp, err := c.grpcClient.GetAuthInfo(ctx, &authpb.GetAuthInfoRequest{
		AuthID: authID,
	})
	if err != nil {
		return nil, fmt.Errorf("auth-service: GetAuthInfo failed: %w", err)
	}

	return &AuthInfo{
		AuthID:            resp.AuthID,
		Email:             resp.Email,
		PhoneNumber:       resp.PhoneNumber,
		IsActive:          resp.IsActive,
		IsSuspended:       resp.IsSuspended,
		SuspensionEndDate: resp.SuspensionEndDate,
	}, nil
}
