package client

import "context"

// UserProfile contient les champs d'identité d'un utilisateur utilisés pour enrichir les avis.
type UserProfile struct {
	FirstName       string
	LastName        string
	ProfileImageURL string
}

// UserClient définit les opérations inter-service vers user-service.
// Implémenté par UserServiceClient en production, mockable pour les tests.
type UserClient interface {
	// UserExists vérifie qu'un utilisateur existe dans user-service via son UserID.
	UserExists(ctx context.Context, userID string) (bool, error)
	// GetUsersByUserIDs récupère en batch les profils (nom/prénom/photo) de plusieurs utilisateurs.
	// Retourne une map indexée par UserID. En cas d'erreur, retourne une map vide (dégradation gracieuse).
	GetUsersByUserIDs(ctx context.Context, userIDs []string) (map[string]*UserProfile, error)
	Close() error
}
