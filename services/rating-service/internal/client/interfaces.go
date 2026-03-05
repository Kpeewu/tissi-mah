package client

import "context"

// UserClient définit les opérations inter-service vers user-service.
// Implémenté par UserServiceClient en production, mockable pour les tests.
type UserClient interface {
	// UserExists vérifie qu'un utilisateur existe dans user-service via son AuthID.
	UserExists(ctx context.Context, authID string) (bool, error)
	Close() error
}
