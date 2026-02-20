package middleware

import (
	"context"
	"strings"

	firebaseValidator "github.com/Kpeewu/tissi-mah/services/auth-service/pkg/firebase"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type contextKey string

// FirebaseIDKey est la clé du contexte gRPC où est stocké le Firebase UID,
// injecté par l'intercepteur après validation complète du JWT.
const FirebaseIDKey contextKey = "firebaseID"

// Routes gRPC qui requièrent un JWT Firebase valide
var protectedMethods = map[string]bool{
	"/auth.AuthService/Login":         true,
	"/auth.AuthService/CreateAccount": true,
	"/auth.AuthService/DeleteAccount": true,
}

// AuthInterceptor retourne un intercepteur gRPC unaire qui :
//  1. Laisse passer les routes publiques (Health, CheckEmail, CheckPhoneNumber)
//  2. Extrait le Bearer token du header Authorization (metadata gRPC)
//  3. Valide le token avec Firebase Admin SDK (signature, issuer, audience, expiration)
//  4. Injecte le Firebase UID dans le contexte via FirebaseIDKey
func AuthInterceptor(validator *firebaseValidator.JWTValidator) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if !protectedMethods[info.FullMethod] {
			return handler(ctx, req)
		}

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing metadata")
		}

		authHeaders := md.Get("authorization")
		if len(authHeaders) == 0 {
			return nil, status.Error(codes.Unauthenticated, "missing authorization header")
		}

		rawToken := authHeaders[0]
		if !strings.HasPrefix(rawToken, "Bearer ") {
			return nil, status.Error(codes.Unauthenticated, "authorization header must be Bearer token")
		}
		idToken := strings.TrimPrefix(rawToken, "Bearer ")

		firebaseID, err := validator.VerifyToken(ctx, idToken)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid or expired token")
		}

		ctx = context.WithValue(ctx, FirebaseIDKey, firebaseID)
		return handler(ctx, req)
	}
}
