package middleware

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type contextKey string

// FirebaseIDKey est la clé du contexte gRPC où est stocké le Firebase UID,
// injecté par l'api-gateway via la metadata gRPC x-firebase-uid.
const FirebaseIDKey contextKey = "firebaseID"

// Routes gRPC publiques (pas de JWT requis)
var publicMethods = map[string]bool{
	"/kyc.KYCService/Health":         true,
	"/kyc.KYCService/ProcessWebhook": true,
}

// KYCInterceptor retourne un intercepteur gRPC unaire qui :
//  1. Laisse passer les routes publiques (Health, ProcessWebhook)
//  2. Extrait le Firebase UID depuis la metadata gRPC x-firebase-uid
//     (injectée par l'api-gateway après validation JWT Firebase)
//  3. Injecte le Firebase UID dans le contexte via FirebaseIDKey
//
// TODO: réactiver la vérification Firebase UID une fois l'authentification implémentée
func KYCInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if publicMethods[info.FullMethod] {
			return handler(ctx, req)
		}

		// Extraire le Firebase UID si présent, sans bloquer si absent
		md, ok := metadata.FromIncomingContext(ctx)
		if ok {
			uids := md.Get("x-firebase-uid")
			if len(uids) > 0 && uids[0] != "" {
				ctx = context.WithValue(ctx, FirebaseIDKey, uids[0])
			}
		}

		return handler(ctx, req)
	}
}
