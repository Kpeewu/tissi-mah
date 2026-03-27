package middleware

import (
	"context"

	"google.golang.org/grpc"
)

type contextKey string

// FirebaseIDKey est la clé de contexte pour le Firebase UID.
const FirebaseIDKey contextKey = "firebaseID"

// PaymentInterceptor retourne un intercepteur gRPC unaire.
// Actuellement tous les endpoints sont publics (IDs passés dans le body).
// Quand Firebase JWT sera activé, cet intercepteur extraira le Firebase UID
// depuis la metadata gRPC x-firebase-uid et l'injectera dans le contexte.
func PaymentInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		_ *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		return handler(ctx, req)
	}
}
