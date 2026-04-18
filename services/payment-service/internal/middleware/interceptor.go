package middleware

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type contextKey string

// FirebaseIDKey est la clé du contexte gRPC où est stocké le Firebase UID,
// injecté par l'api-gateway via la metadata gRPC x-firebase-uid.
const FirebaseIDKey contextKey = "firebaseID"

// SupportUIDKey est la clé du contexte gRPC où est stocké le support UID,
// injecté par l'api-gateway via la metadata gRPC x-support-uid après validation JWT support.
const SupportUIDKey contextKey = "supportUID"

// Routes gRPC publiques ou internes (pas de JWT requis)
var publicMethods = map[string]bool{
	"/payment.PaymentService/Health":         true,
	"/payment.PaymentService/ProcessWebhook": true, // Webhook FedaPay (vérifié par HMAC)
	"/payment.PaymentService/RequestRefund":  true, // Route interne (booking-service)
	"/payment.PaymentService/ReleasePayment": true, // Route interne (booking-service)
	"/grpc.health.v1.Health/Check":           true, // Readiness probe Kubernetes
	"/grpc.health.v1.Health/Watch":           true, // Liveness probe Kubernetes
}

// Routes gRPC protégées par JWT support (x-support-uid injecté par l'api-gateway)
var supportMethods = map[string]bool{
	"/payment.PaymentService/TriggerManualPayout": true,
}

// PaymentInterceptor retourne un intercepteur gRPC unaire qui :
//  1. Laisse passer les routes publiques et internes (Health, ProcessWebhook, RequestRefund, ReleasePayment)
//  2. Pour les routes support, extrait x-support-uid et l'injecte via SupportUIDKey
//  3. Pour les autres routes, extrait le Firebase UID depuis x-firebase-uid et l'injecte via FirebaseIDKey
func PaymentInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if publicMethods[info.FullMethod] {
			return handler(ctx, req)
		}

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing metadata")
		}

		if supportMethods[info.FullMethod] {
			supportUIDs := md.Get("x-support-uid")
			if len(supportUIDs) == 0 || supportUIDs[0] == "" {
				return nil, status.Error(codes.Unauthenticated, "missing support uid")
			}
			ctx = context.WithValue(ctx, SupportUIDKey, supportUIDs[0])
			return handler(ctx, req)
		}

		uids := md.Get("x-firebase-uid")
		if len(uids) == 0 || uids[0] == "" {
			return nil, status.Error(codes.Unauthenticated, "missing firebase uid")
		}

		ctx = context.WithValue(ctx, FirebaseIDKey, uids[0])
		return handler(ctx, req)
	}
}
