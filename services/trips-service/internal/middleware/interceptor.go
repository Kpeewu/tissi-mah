package middleware

import (
	"context"
	"errors"

	grpcutil "github.com/Kpeewu/tissi-mah/pkg/grpcutil"
	tripErrors "github.com/Kpeewu/tissi-mah/services/trips-service/pkg/errors"
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
// tout contrôle d'appartenance (les trajets référencent leur conducteur par UserID interne).
const UserIDKey contextKey = "userID"

// UserIDResolver résout un Firebase UID en UserID interne (implémenté par le client user-service).
type UserIDResolver interface {
	GetUserIDByFirebaseID(ctx context.Context, firebaseUID string) (string, error)
}

// Routes gRPC publiques ou internes (pas de JWT requis)
var publicMethods = map[string]bool{
	"/trip.TripService/Health":                       true,
	"/trip.TripService/GetTripByID":                  true, // Route interne (booking-service)
	"/trip.TripService/UpdateAvailableSeats":         true, // Route interne (booking-service)
	"/trip.TripService/IncrementLegBookedSeats":      true, // Route interne (booking-service)
	"/trip.TripService/SyncLegBookedSeats":           true, // Route interne (booking-service)
	"/trip.TripService/GetScheduledTripsPreviews":    true, // Route publique passager
	"/trip.TripService/GetPassengerTripDetails":      true, // Route publique passager
	"/trip.TripService/CheckDeletionEligibility":     true, // Route interne (auth-service, suppression de compte)
	"/trip.TripService/AnonymizeUserData":            true, // Route interne (auth-service, suppression de compte)
	"/trip.TripService/GetVehicleCompletedTripCount": true, // Route interne (vehicle-service)
	"/grpc.health.v1.Health/Check":                   true, // Readiness probe Kubernetes
	"/grpc.health.v1.Health/Watch":                   true, // Liveness probe Kubernetes
}

// TripInterceptor retourne un intercepteur gRPC unaire qui :
//  1. Laisse passer les routes publiques et internes (Health, GetTripByID, UpdateAvailableSeats)
//  2. Extrait le Firebase UID depuis la metadata gRPC x-firebase-uid
//     (injectée par l'api-gateway après validation JWT Firebase)
//  3. Résout l'UserID interne via user-service et injecte FirebaseIDKey + UserIDKey dans le contexte
func TripInterceptor(secret []byte, resolver UserIDResolver) grpc.UnaryServerInterceptor {
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
		if errors.Is(err, tripErrors.ErrorUserNotFound) {
			return nil, status.Error(codes.PermissionDenied, "user not found")
		}
		return nil, status.Error(codes.Unavailable, "user resolution failed")
	}

	ctx = context.WithValue(ctx, FirebaseIDKey, uids[0])
	ctx = context.WithValue(ctx, UserIDKey, userID)
	return ctx, nil
}
