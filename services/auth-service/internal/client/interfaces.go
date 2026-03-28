package client

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/auth-service/internal/domain"
)

// UserClient définit les opérations inter-service vers user-service.
// Implémenté par UserServiceClient en production, mockable pour les tests.
type UserClient interface {
	CreateUser(ctx context.Context, authID string, firebaseID string, name string, firstName string, profilePhotoURL string) (*domain.UserPreview, error)
	GetUserByAuthID(ctx context.Context, authID string) (*domain.UserPreview, error)
	SoftDeleteUser(ctx context.Context, authID string) error
	Close() error
}
