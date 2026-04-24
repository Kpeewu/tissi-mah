package client

import "context"

// UserClient est l'interface pour communiquer avec user-service via gRPC.
// Le file-service reçoit un Firebase UID injecté par l'api-gateway dans la
// metadata gRPC (x-firebase-uid) et doit le résoudre en UUID interne avant
// de stocker les documents (convention partagée avec user-service et kyc-service).
type UserClient interface {
	// GetInternalUserIDByFirebaseID résout un Firebase UID en UserID interne MongoDB.
	GetInternalUserIDByFirebaseID(ctx context.Context, firebaseUID string) (string, error)

	// Close libère la connexion gRPC.
	Close() error
}
