package middleware

import (
	"context"
	"errors"

	grpcutil "github.com/Kpeewu/tissi-mah/pkg/grpcutil"
	vehicleErrors "github.com/Kpeewu/tissi-mah/services/vehicle-service/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type contextKey string

// FirebaseIDKey est la clé du contexte gRPC où est stocké le Firebase UID,
// injecté par l'api-gateway via la metadata gRPC x-firebase-uid.
const FirebaseIDKey contextKey = "firebaseID"

// UserIDKey est la clé du contexte gRPC où est stocké l'UserID interne de l'appelant,
// résolu depuis le Firebase UID via user-service. C'est l'identité à utiliser pour
// tout contrôle d'appartenance (les véhicules référencent leur propriétaire par UserID interne).
const UserIDKey contextKey = "userID"

// UserIDResolver résout un Firebase UID en UserID interne (implémenté par le client user-service).
type UserIDResolver interface {
	GetUserIDByFirebaseID(ctx context.Context, firebaseUID string) (string, error)
}

// Routes gRPC publiques (pas de JWT requis)
var publicMethods = map[string]bool{
	"/vehicle.VehicleService/Health":         true,
	"/vehicle.VehicleService/GetVehicleInfo": true, // Appelé par file-service et trips-service (inter-service)
	// Appelé par kyc-service après chaque décision support, sans utilisateur final.
	// Sans cette exemption, l'appel échouait (« missing firebase uid ») et aucun
	// véhicule ne passait jamais vérifié. Aucune route HTTP ne l'expose : il n'est
	// joignable qu'en gRPC depuis le cluster.
	"/vehicle.VehicleService/SetVehicleVerification": true,
	"/grpc.health.v1.Health/Check":                   true, // Readiness probe Kubernetes
	"/grpc.health.v1.Health/Watch":                   true, // Liveness probe Kubernetes
}

// VehicleInterceptor retourne un intercepteur gRPC unaire qui :
//  1. Laisse passer les routes publiques et internes (Health, GetVehicleInfo, SetVehicleVerification)
//  2. Extrait le Firebase UID depuis la metadata gRPC x-firebase-uid
//     (injectée par l'api-gateway après validation JWT Firebase)
//  3. Résout l'UserID interne via user-service et injecte FirebaseIDKey + UserIDKey dans le contexte
func VehicleInterceptor(secret []byte, resolver UserIDResolver) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if publicMethods[info.FullMethod] {
			return handler(ctx, req)
		}

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing metadata")
		}

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
		if errors.Is(err, vehicleErrors.ErrorUserNotFound) {
			return nil, status.Error(codes.PermissionDenied, "user not found")
		}
		return nil, status.Error(codes.Unavailable, "user resolution failed")
	}

	ctx = context.WithValue(ctx, FirebaseIDKey, uids[0])
	ctx = context.WithValue(ctx, UserIDKey, userID)
	return ctx, nil
}
