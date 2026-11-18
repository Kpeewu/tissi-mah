package middleware

import (
	"context"
	"errors"

	grpcutil "github.com/Kpeewu/tissi-mah/pkg/grpcutil"
	notifErrors "github.com/Kpeewu/tissi-mah/services/notification-service/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type contextKey string

const FirebaseIDKey contextKey = "firebase_uid"

// UserIDKey porte l'UserID interne de l'appelant, résolu depuis le Firebase UID via
// user-service : device tokens, préférences et inbox sont keyés sur l'UserID interne.
const UserIDKey contextKey = "user_id"

// UserIDResolver résout un Firebase UID en UserID interne (implémenté par le client user-service).
type UserIDResolver interface {
	GetUserIDByFirebaseID(ctx context.Context, firebaseUID string) (string, error)
}

// publicMethods : routes accessibles sans authentification.
var publicMethods = map[string]bool{
	"/notification.NotificationService/Health":                true,
	"/notification.NotificationService/InvalidateDeviceToken": true,
	"/grpc.health.v1.Health/Check":                            true,
	"/grpc.health.v1.Health/Watch":                            true,
}

// NotificationInterceptor vérifie le x-firebase-uid pour les routes protégées et résout
// l'UserID interne de l'appelant (FirebaseIDKey + UserIDKey).
func NotificationInterceptor(logger *zap.Logger, secret []byte, resolver UserIDResolver) grpc.UnaryServerInterceptor {
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

		sig := md.Get(grpcutil.MetadataUIDSig)
		if len(sig) > 0 && len(secret) > 0 {
			if !grpcutil.VerifyUID(secret, firebaseUIDs[0], sig[0]) {
				return nil, status.Error(codes.Unauthenticated, "invalid uid signature")
			}
		}

		userID, err := resolver.GetUserIDByFirebaseID(ctx, firebaseUIDs[0])
		if err != nil {
			// Utilisateur pas encore provisionné (inscription en cours : le compte backend
			// n'existe qu'après la vérification e-mail) : NotFound, le client réessaie plus tard.
			if errors.Is(err, notifErrors.ErrorUserNotProvisioned) {
				return nil, status.Error(codes.NotFound, "user not provisioned yet")
			}
			return nil, status.Error(codes.Unavailable, "user resolution failed")
		}

		ctx = context.WithValue(ctx, FirebaseIDKey, firebaseUIDs[0])
		ctx = context.WithValue(ctx, UserIDKey, userID)
		return handler(ctx, req)
	}
}
