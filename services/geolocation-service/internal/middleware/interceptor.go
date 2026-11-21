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

// publicMethods : routes gRPC accessibles sans Firebase JWT.
var publicMethods = map[string]bool{
	"/geolocation.GeolocationService/Health": true,
	"/grpc.health.v1.Health/Check":           true,
	"/grpc.health.v1.Health/Watch":           true,
}

// GeolocationInterceptor retourne un intercepteur gRPC unaire qui :
//  1. Laisse passer les routes publiques (Health).
//  2. Extrait le Firebase UID depuis la metadata gRPC x-firebase-uid
//     (injectée par l'api-gateway après validation JWT Firebase).
//  3. Injecte le Firebase UID dans le contexte via FirebaseIDKey.
//
// Le service ne fait pas appel au user-service pour résoudre l'UUID interne — il n'en a
// pas besoin (ComputeRoute/Geocode sont des opérations purement géographiques sans
// notion d'identité business). Le Firebase UID sert uniquement à journaliser les requêtes.
func GeolocationInterceptor(secret []byte) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if publicMethods[info.FullMethod] {
			return handler(ctx, req)
		}

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing metadata")
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
