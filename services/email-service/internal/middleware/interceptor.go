package middleware

import (
	"context"

	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// publicMethods liste les routes gRPC qui ne requièrent pas d'authentification.
// L'email-service est un service interne : toutes les routes sont publiques.
var publicMethods = map[string]bool{
	"/email.EmailService/SendEmail": true,
	"/email.EmailService/Health":    true,
	"/grpc.health.v1.Health/Check":  true,
	"/grpc.health.v1.Health/Watch":  true,
}

// EmailInterceptor retourne un intercepteur gRPC unaire.
// Toutes les routes étant publiques (inter-service uniquement),
// il log la requête et la laisse passer.
func EmailInterceptor(logger *zap.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		logger.Debug("grpc request", zap.String("method", info.FullMethod))

		if !publicMethods[info.FullMethod] {
			logger.Warn("unknown method called", zap.String("method", info.FullMethod))
		}

		return handler(ctx, req)
	}
}
