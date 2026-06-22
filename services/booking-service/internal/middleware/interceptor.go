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

// Méthodes réservées aux agents support (JWT support, pas Firebase)
var adminMethods = map[string]bool{
	"/booking.BookingService/ListBookings":          true,
	"/booking.BookingService/GetBookingDetailAdmin": true,
}

// Routes gRPC publiques ou internes (pas de JWT requis)
var publicMethods = map[string]bool{
	"/booking.BookingService/Health":                      true,
	"/booking.BookingService/StartBookingsForWaypoint":    true, // Route interne (trips-service)
	"/booking.BookingService/CompleteBookingsForWaypoint": true, // Route interne (trips-service)
	"/booking.BookingService/ConfirmPayment":              true, // Route interne (payment-service)
	"/booking.BookingService/FailPayment":                 true, // Route interne (payment-service)
	"/booking.BookingService/GetBookingDetails":           true, // Route interne (payment-service)
	"/booking.BookingService/CancelBookingsForWaypoint":   true, // Route interne (trips-service)
	"/booking.BookingService/CancelBookingsForTrip":       true, // Route interne (trips-service)
	"/grpc.health.v1.Health/Check":                        true, // Readiness probe Kubernetes
	"/grpc.health.v1.Health/Watch":                        true, // Liveness probe Kubernetes
}

// BookingInterceptor retourne un intercepteur gRPC unaire qui :
//  1. Laisse passer les routes publiques et internes (Health, StartBookingsForWaypoint, CompleteBookingsForWaypoint)
//  2. Extrait le Firebase UID depuis la metadata gRPC x-firebase-uid
//     (injectée par l'api-gateway après validation JWT Firebase)
//  3. Injecte le Firebase UID dans le contexte via FirebaseIDKey
func BookingInterceptor(secret []byte) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if publicMethods[info.FullMethod] {
			return handler(ctx, req)
		}

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing metadata")
		}

		// Méthodes admin support : lire x-support-uid → SupportIDKey
		if adminMethods[info.FullMethod] {
			suids := md.Get("x-support-uid")
			if len(suids) == 0 || suids[0] == "" {
				return nil, status.Error(codes.Unauthenticated, "missing support uid")
			}
			ctx = context.WithValue(ctx, SupportIDKey, suids[0])
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
