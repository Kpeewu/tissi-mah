package middleware

import (
	"context"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type contextKey string

const FirebaseIDKey contextKey = "firebase_uid"

// publicMethods : routes accessibles sans authentification.
var publicMethods = map[string]bool{
	"/notification.NotificationService/Health":              true,
	"/notification.NotificationService/InvalidateDeviceToken": true,
	"/grpc.health.v1.Health/Check":                          true,
	"/grpc.health.v1.Health/Watch":                          true,
}

// NotificationInterceptor vérifie le x-firebase-uid pour les routes protégées.
func NotificationInterceptor(logger *zap.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		logger.Debug("grpc request", zap.String("method", info.FullMethod))

		if publicMethods[info.FullMethod] {
			return handler(ctx, req)
		}

		// Extraire le firebase_uid depuis les metadata gRPC (injecté par api-gateway)
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing metadata")
		}

		firebaseUIDs := md.Get("x-firebase-uid")
		if len(firebaseUIDs) == 0 || firebaseUIDs[0] == "" {
			return nil, status.Error(codes.Unauthenticated, "missing firebase uid")
		}

		ctx = context.WithValue(ctx, FirebaseIDKey, firebaseUIDs[0])
		return handler(ctx, req)
	}
}
