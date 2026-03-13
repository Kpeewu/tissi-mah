package middleware

import (
	"context"

	"google.golang.org/grpc"
)

type contextKey string

// FirebaseIDKey est la clé du contexte gRPC où sera stocké le Firebase UID.
// Conservé pour intégration future quand Firebase JWT sera activé.
const FirebaseIDKey contextKey = "firebaseID"

// TripInterceptor retourne un intercepteur gRPC unaire.
// Actuellement tous les endpoints sont publics (driver_id passé dans le body).
// Quand Firebase JWT sera activé, cet intercepteur extraira le Firebase UID
// depuis la metadata gRPC x-firebase-uid et l'injectera dans le contexte.
func TripInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		return handler(ctx, req)
	}
}
