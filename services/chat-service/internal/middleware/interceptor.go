package middleware

import (
	"context"
	"errors"

	grpcutil "github.com/Kpeewu/tissi-mah/pkg/grpcutil"
	chatErrors "github.com/Kpeewu/tissi-mah/services/chat-service/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type contextKey string

// FirebaseIDKey est la clé du contexte gRPC portant le Firebase UID,
// injecté par l'api-gateway via la metadata x-firebase-uid.
const FirebaseIDKey contextKey = "firebaseID"

// UserIDKey est la clé du contexte gRPC portant l'UserID interne de l'appelant,
// résolu depuis le Firebase UID via user-service. Toutes les opérations chat
// (autorisation, sender_id, comparaisons avec booking.DriverID/PassengerID)
// utilisent uniquement cet UserID interne — jamais le Firebase UID.
const UserIDKey contextKey = "userID"

// UserIDResolver résout un Firebase UID en UserID interne (implémenté par le client user-service).
type UserIDResolver interface {
	GetUserIDByFirebaseID(ctx context.Context, firebaseUID string) (string, error)
}

// SupportIDKey porte l'ID support pour les endpoints réservés au support.
const SupportIDKey contextKey = "supportID"

var publicMethods = map[string]bool{
	"/chat.ChatService/Health":            true,
	"/chat.ChatService/CloseThread":       true, // appelé en interne par le worker
	"/chat.ChatService/AnonymizeUserData": true, // Route interne (auth-service, suppression de compte)
	"/grpc.health.v1.Health/Check":        true,
	"/grpc.health.v1.Health/Watch":        true,
}

var supportMethods = map[string]bool{
	"/chat.ChatService/GetFlaggedMessageContent": true,
}

// ChatInterceptor extrait le Support UID ou, pour les routes Firebase, résout l'UserID
// interne de l'appelant via user-service (FirebaseIDKey + UserIDKey).
func ChatInterceptor(secret []byte, resolver UserIDResolver) grpc.UnaryServerInterceptor {
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
		ctx, err := authenticateFirebaseCaller(ctx, md, secret, resolver)
		if err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

// authenticateFirebaseCaller vérifie x-firebase-uid (et sa signature HMAC si présente),
// résout l'UserID interne correspondant et les injecte dans le contexte.
func authenticateFirebaseCaller(
	ctx context.Context, md metadata.MD, secret []byte, resolver UserIDResolver,
) (context.Context, error) {
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

	userID, err := resolver.GetUserIDByFirebaseID(ctx, uids[0])
	if err != nil {
		if errors.Is(err, chatErrors.ErrUserNotFound) {
			return nil, status.Error(codes.PermissionDenied, "user not found")
		}
		return nil, status.Error(codes.Unavailable, "user resolution failed")
	}

	ctx = context.WithValue(ctx, FirebaseIDKey, uids[0])
	ctx = context.WithValue(ctx, UserIDKey, userID)
	return ctx, nil
}
