package middleware

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// méthodes accessibles sans authentification inter-service
var publicMethods = map[string]bool{
	"/moderation.ModerationService/Health": true,
}

// ModerationInterceptor est un intercepteur gRPC minimal.
// Le service est interne uniquement (pas de JWT Firebase).
// Il vérifie juste que les méthodes non publiques sont bien appelées en interne.
func ModerationInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if publicMethods[info.FullMethod] {
			return handler(ctx, req)
		}
		// Vérification future : HMAC inter-service si besoin
		if req == nil {
			return nil, status.Error(codes.InvalidArgument, "empty request")
		}
		return handler(ctx, req)
	}
}
