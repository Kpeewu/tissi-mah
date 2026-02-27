package client

import "context"

// AuthClient définit les opérations inter-service vers auth-service.
// Implémenté par AuthServiceClient en production, mockable pour les tests.
type AuthClient interface {
	GetAuthInfo(ctx context.Context, authID string) (*AuthInfo, error)
	Close() error
}
