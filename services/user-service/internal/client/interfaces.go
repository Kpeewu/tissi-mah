package client

import "context"

// AuthClient définit les opérations inter-service vers auth-service.
// Implémenté par AuthServiceClient en production, mockable pour les tests.
type AuthClient interface {
	GetAuthInfo(ctx context.Context, authID string) (*AuthInfo, error)
	Close() error
}

// FileClient définit les opérations inter-service vers file-service.
// Les appels sont non-bloquants : retourne "" si file-service est indisponible.
type FileClient interface {
	// GetDocumentExpiry retourne expired_at du document courant, "" si non trouvé ou erreur.
	GetDocumentExpiry(ctx context.Context, userID, documentType string) string
	Close() error
}
