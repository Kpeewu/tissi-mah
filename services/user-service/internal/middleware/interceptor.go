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

// Routes gRPC qui requièrent un Firebase UID (transmis par l'api-gateway)
//
// TODO(test): remettre les méthodes protégées après les tests
var protectedMethods = map[string]bool{
	// "/user.UserService/GetMyProfile":       true,
	// "/user.UserService/CreateDriverAccount": true,
	// "/user.UserService/AddTripPreferences":  true,
	// "/user.UserService/UpdateProfile":       true,
}

// AuthInterceptor retourne un intercepteur gRPC unaire qui :
//  1. Laisse passer les routes publiques (CreateUser, GetUserByAuthID, Health)
//  2. Extrait le Firebase UID depuis la metadata gRPC x-firebase-uid
//     (injectée par l'api-gateway après validation JWT Firebase)
//  3. Injecte le Firebase UID dans le contexte via FirebaseIDKey
func AuthInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if !protectedMethods[info.FullMethod] {
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

		ctx = context.WithValue(ctx, FirebaseIDKey, uids[0])
		return handler(ctx, req)
	}
}
