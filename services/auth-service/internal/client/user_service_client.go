package client

import (
	"context"
	"fmt"

	"github.com/Kpeewu/tissi-mah/pkg/grpcutil"
	"github.com/Kpeewu/tissi-mah/services/auth-service/internal/domain"
	userpb "github.com/Kpeewu/tissi-mah/services/auth-service/proto/gen/userpb"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// UserServiceClient est le client gRPC vers user-service.
type UserServiceClient struct {
	conn       *grpc.ClientConn
	grpcClient userpb.UserServiceClient
	logger     *zap.Logger
}

// NewUserServiceClient établit la connexion gRPC vers user-service.
// address doit être au format "host:port" (ex: "user-service:50052").
func NewUserServiceClient(address string, logger *zap.Logger) (*UserServiceClient, error) {
	logger.Debug("connecting to user-service", zap.String("address", address))
	conn, err := grpcutil.NewClientConn(address, logger)
	if err != nil {
		logger.Error("failed to connect to user-service", zap.Error(err), zap.String("address", address))
		return nil, fmt.Errorf("user-service: failed to connect to %s: %w", address, err)
	}

	return &UserServiceClient{
		conn:       conn,
		grpcClient: userpb.NewUserServiceClient(conn),
		logger:     logger,
	}, nil
}

// Close libère la connexion gRPC. À appeler au shutdown du service.
func (c *UserServiceClient) Close() error {
	return c.conn.Close()
}

// CreateUser crée un profil utilisateur dans user-service après la création du compte auth.
// Retourne un UserPreview partiel (sans email/phone — ceux-ci appartiennent à auth-service).
func (c *UserServiceClient) CreateUser(ctx context.Context, authID string, firebaseID string, name string, firstName string, profilePhotoURL string) (*domain.UserPreview, error) {
	c.logger.Debug("client: CreateUser called",
		zap.String("authID", authID),
		zap.String("firebaseID", firebaseID),
		zap.String("name", name),
	)
	resp, err := c.grpcClient.CreateUser(ctx, &userpb.CreateUserRequest{
		AuthID:          authID,
		Name:            name,
		FirstName:       firstName,
		ProfilePhotoURL: profilePhotoURL,
		FirebaseID:      firebaseID,
	})
	if err != nil {
		c.logger.Error("client: CreateUser failed", zap.Error(err), zap.String("authID", authID))
		return nil, fmt.Errorf("user-service: CreateUser failed: %w", err)
	}

	c.logger.Debug("client: CreateUser success", zap.String("authID", authID))
	return toUserPreview(resp), nil
}

// GetUserByAuthID récupère le profil utilisateur depuis user-service.
// Retourne un UserPreview partiel (sans email/phone — ceux-ci appartiennent à auth-service).
func (c *UserServiceClient) GetUserByAuthID(ctx context.Context, authID string) (*domain.UserPreview, error) {
	c.logger.Debug("client: GetUserByAuthID called", zap.String("authID", authID))
	resp, err := c.grpcClient.GetUserByAuthID(ctx, &userpb.GetUserByAuthIDRequest{
		AuthID: authID,
	})
	if err != nil {
		c.logger.Error("client: GetUserByAuthID failed", zap.Error(err), zap.String("authID", authID))
		return nil, fmt.Errorf("user-service: GetUserByAuthID failed: %w", err)
	}

	c.logger.Debug("client: GetUserByAuthID success", zap.String("authID", authID))
	return toUserPreview(resp), nil
}

// SoftDeleteUser anonymise et soft-delete le profil utilisateur dans user-service.
func (c *UserServiceClient) SoftDeleteUser(ctx context.Context, authID string) error {
	c.logger.Debug("client: SoftDeleteUser called", zap.String("authID", authID))
	_, err := c.grpcClient.SoftDeleteUser(ctx, &userpb.SoftDeleteUserRequest{
		AuthID: authID,
	})
	if err != nil {
		c.logger.Error("client: SoftDeleteUser failed", zap.Error(err), zap.String("authID", authID))
		return fmt.Errorf("user-service: SoftDeleteUser failed: %w", err)
	}

	c.logger.Debug("client: SoftDeleteUser success", zap.String("authID", authID))
	return nil
}

// toUserPreview convertit la réponse user-service en domaine UserPreview.
// Email et PhoneNumber ne sont pas dans la réponse user-service — ils seront
// enrichis par auth-service à partir de son propre repository.
func toUserPreview(resp *userpb.UserProfileResponse) *domain.UserPreview {
	preview := &domain.UserPreview{
		AuthID:    resp.AuthID,
		UserID:    resp.UserID,
		Name:      resp.Name,
		FirstName: resp.FirstName,
	}

	if resp.ProfileImageURL != "" {
		url := resp.ProfileImageURL
		preview.ProfilePhotoURL = &url
	}

	return preview
}
