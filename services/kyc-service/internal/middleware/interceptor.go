package middleware

import (
	"context"

	grpcutil "github.com/Kpeewu/tissi-mah/pkg/grpcutil"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type contextKey string

// FirebaseIDKey est la clé du contexte gRPC où est stocké le Firebase UID,
// injecté par l'api-gateway via la metadata gRPC x-firebase-uid.
const FirebaseIDKey contextKey = "firebaseID"

// SupportIDKey est la clé du contexte gRPC où est stocké l'UID de l'agent support,
// injecté par l'api-gateway via la metadata gRPC x-support-uid.
const SupportIDKey contextKey = "supportID"

// Routes gRPC publiques (pas de JWT requis)
var publicMethods = map[string]bool{
	"/kyc.KYCService/Health":         true,
	"/kyc.KYCService/ProcessWebhook": true,
	"/grpc.health.v1.Health/Check":   true, // Readiness probe Kubernetes
	"/grpc.health.v1.Health/Watch":   true, // Liveness probe Kubernetes
}

// Méthodes réservées aux agents support (JWT support, pas Firebase)
var adminMethods = map[string]bool{
	"/kyc.KYCService/GetAdminReviews":              true,
	"/kyc.KYCService/GetAdminReview":               true,
	"/kyc.KYCService/OverrideReview":               true,
	"/kyc.KYCService/ValidateDocument":             true,
	"/kyc.KYCService/GetManualReviewRequests":      true,
	"/kyc.KYCService/GetManualReviewRequestDetail": true,
}

// KYCInterceptor retourne un intercepteur gRPC unaire qui :
//  1. Laisse passer les routes publiques (Health, ProcessWebhook)
//  2. Pour les méthodes admin : extrait x-support-uid → SupportIDKey
//  3. Pour les autres méthodes : extrait x-firebase-uid → FirebaseIDKey
func KYCInterceptor(secret []byte) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if publicMethods[info.FullMethod] {
			return handler(ctx, req)
		}

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing metadata")
		}

		if adminMethods[info.FullMethod] {
			uids := md.Get("x-support-uid")
			if len(uids) == 0 || uids[0] == "" {
				return nil, status.Error(codes.Unauthenticated, "missing support uid")
			}
			ctx = context.WithValue(ctx, SupportIDKey, uids[0])
			return handler(ctx, req)
		}

		uids := md.Get("x-firebase-uid")
		if len(uids) == 0 || uids[0] == "" {
			return nil, status.Error(codes.Unauthenticated, "missing firebase uid")
		}

		sig := md.Get(grpcutil.MetadataUIDSig)
		if len(sig) > 0 && len(secret) > 0 {
			if !grpcutil.VerifyUID(secret, uids[0], sig[0]) {
				return nil, status.Error(codes.Unauthenticated, "invalid uid signature")
			}
		}

		ctx = context.WithValue(ctx, FirebaseIDKey, uids[0])
		return handler(ctx, req)
	}
}
