package middleware

import (
	"context"

	"google.golang.org/grpc"
)

// FileServiceInterceptor retourne un intercepteur gRPC unaire.
// Le file-service est inter-service uniquement, pas de validation JWT nécessaire.
// Cet intercepteur est prévu pour de futurs besoins (logging, tracing, etc.)
func FileServiceInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		return handler(ctx, req)
	}
}
