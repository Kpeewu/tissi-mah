package client

import "context"

// UserClient définit le contrat pour appeler user-service depuis trips-service.
type UserClient interface {
	// IsVerifiedDriver vérifie que l'utilisateur existe et a un profil conducteur vérifié.
	IsVerifiedDriver(ctx context.Context, userID string) (bool, error)
	Close() error
}
