package middleware

import (
	"context"

	"google.golang.org/grpc"
)

type contextKey string

// FirebaseIDKey est la clé du contexte gRPC où est stocké le Firebase UID.
// Conservé pour compatibilité future quand Firebase JWT sera activé.
const FirebaseIDKey contextKey = "firebaseID"

// RatingInterceptor retourne un intercepteur gRPC unaire.
// Actuellement tous les endpoints sont publics car le rater_id est passé dans le body.
// Quand Firebase JWT sera activé, cet intercepteur extraira le Firebase UID
// depuis la metadata gRPC x-firebase-uid et l'injectera dans le contexte.
func RatingInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		return handler(ctx, req)
	}
}
