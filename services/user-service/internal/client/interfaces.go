package client

import "context"

// AuthClient définit les opérations inter-service vers auth-service.
// Implémenté par AuthServiceClient en production, mockable pour les tests.
type AuthClient interface {
	GetAuthInfo(ctx context.Context, authID string) (*AuthInfo, error)
	Close() error
}

// FileClient définit les opérations inter-service vers file-service.
type FileClient interface {
	// GetDocumentExpiry retourne expired_at du document courant, "" si non trouvé ou erreur (non-bloquant).
	GetDocumentExpiry(ctx context.Context, userID, documentType string) string
	// GetCurrentDocumentURL retourne l'URL présignée FRAÎCHE du document courant
	// du type donné, "" si non trouvé ou erreur (non-bloquant). Utilisé pour
	// résoudre la photo de profil (selfie) à la lecture — les URLs présignées
	// expirent et ne doivent jamais être persistées.
	GetCurrentDocumentURL(ctx context.Context, userID, documentType string) string
	Close() error
}
