package client

import (
	"context"
	"fmt"

	"github.com/Kpeewu/tissi-mah/services/auth-service/internal/domain"
	userpb "github.com/Kpeewu/tissi-mah/services/auth-service/proto/gen/userpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// UserServiceClient est le client gRPC vers user-service.
// Utilise insecure.NewCredentials() pour la communication intra-cluster
// (le chiffrement est géré au niveau du service mesh / mTLS Kubernetes).
type UserServiceClient struct {
	conn       *grpc.ClientConn
	grpcClient userpb.UserServiceClient
}

// NewUserServiceClient établit la connexion gRPC vers user-service.
// address doit être au format "host:port" (ex: "user-service:50052").
func NewUserServiceClient(address string) (*UserServiceClient, error) {
	conn, err := grpc.NewClient(
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("user-service: failed to connect to %s: %w", address, err)
	}

	return &UserServiceClient{
		conn:       conn,
		grpcClient: userpb.NewUserServiceClient(conn),
	}, nil
}

// Close libère la connexion gRPC. À appeler au shutdown du service.
func (c *UserServiceClient) Close() error {
	return c.conn.Close()
}

// CreateUser crée un profil utilisateur dans user-service après la création du compte auth.
// Retourne un UserPreview partiel (sans email/phone — ceux-ci appartiennent à auth-service).
func (c *UserServiceClient) CreateUser(ctx context.Context, authID string, name string, firstName string, profilePhotoURL string) (*domain.UserPreview, error) {
	resp, err := c.grpcClient.CreateUser(ctx, &userpb.CreateUserRequest{
		AuthId:          authID,
		Name:            name,
		FirstName:       firstName,
		ProfilePhotoUrl: profilePhotoURL,
	})
	if err != nil {
		return nil, fmt.Errorf("user-service: CreateUser failed: %w", err)
	}

	return toUserPreview(resp), nil
}

// GetUserByAuthID récupère le profil utilisateur depuis user-service.
// Retourne un UserPreview partiel (sans email/phone — ceux-ci appartiennent à auth-service).
func (c *UserServiceClient) GetUserByAuthID(ctx context.Context, authID string) (*domain.UserPreview, error) {
	resp, err := c.grpcClient.GetUserByAuthID(ctx, &userpb.GetUserByAuthIDRequest{
		AuthId: authID,
	})
	if err != nil {
		return nil, fmt.Errorf("user-service: GetUserByAuthID failed: %w", err)
	}

	return toUserPreview(resp), nil
}

// toUserPreview convertit la réponse user-service en domaine UserPreview.
// Email et PhoneNumber ne sont pas dans la réponse user-service — ils seront
// enrichis par auth-service à partir de son propre repository.
func toUserPreview(resp *userpb.UserProfileResponse) *domain.UserPreview {
	preview := &domain.UserPreview{
		AuthID:    resp.AuthId,
		UserID:    resp.UserId,
		Name:      resp.Name,
		FirstName: resp.FirstName,
	}

	if resp.ProfileImageUrl != "" {
		url := resp.ProfileImageUrl
		preview.ProfilePhotoURL = &url
	}

	return preview
}
