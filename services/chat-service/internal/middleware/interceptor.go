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

// FirebaseIDKey est la clé du contexte gRPC portant le Firebase UID,
// injecté par l'api-gateway via la metadata x-firebase-uid.
const FirebaseIDKey contextKey = "firebaseID"

// SupportIDKey porte l'ID support pour les endpoints réservés au support.
const SupportIDKey contextKey = "supportID"

var publicMethods = map[string]bool{
	"/chat.ChatService/Health":           true,
	"/chat.ChatService/CloseThread":      true, // appelé en interne par le worker
	"/grpc.health.v1.Health/Check":       true,
	"/grpc.health.v1.Health/Watch":       true,
}

var supportMethods = map[string]bool{
	"/chat.ChatService/GetFlaggedMessageContent": true,
}

// ChatInterceptor extrait Firebase UID (ou Support UID) selon la route.
func ChatInterceptor(secret []byte) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if publicMethods[info.FullMethod] {
			return handler(ctx, req)
		}

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing metadata")
		}

		// Endpoints support : nécessite x-support-uid
		if supportMethods[info.FullMethod] {
			supportUIDs := md.Get("x-support-uid")
			if len(supportUIDs) == 0 || supportUIDs[0] == "" {
				return nil, status.Error(codes.PermissionDenied, "support access required")
			}
			ctx = context.WithValue(ctx, SupportIDKey, supportUIDs[0])
			return handler(ctx, req)
		}

		// Routes Firebase : nécessite x-firebase-uid
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
