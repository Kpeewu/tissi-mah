package middleware

import (
	"context"
	"errors"

	grpcutil "github.com/Kpeewu/tissi-mah/pkg/grpcutil"
	paymentErrors "github.com/Kpeewu/tissi-mah/services/payment-service/pkg/errors"
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
// tout contrôle d'appartenance (les réservations et payouts référencent des UserID internes).
const UserIDKey contextKey = "userID"

// SupportUIDKey est la clé du contexte gRPC où est stocké le support UID,
// injecté par l'api-gateway via la metadata gRPC x-support-uid après validation JWT support.
const SupportUIDKey contextKey = "supportUID"

// SupportRoleKey est la clé du contexte gRPC où est stocké le rôle de l'agent support,
// injecté par l'api-gateway via la metadata gRPC x-support-role.
const SupportRoleKey contextKey = "supportRole"

// UserIDResolver résout un Firebase UID en UserID interne (implémenté par le client user-service).
type UserIDResolver interface {
	GetUserIDByFirebaseID(ctx context.Context, firebaseUID string) (string, error)
}

// Routes gRPC publiques ou internes (pas de JWT requis)
var publicMethods = map[string]bool{
	"/payment.PaymentService/Health":         true,
	"/payment.PaymentService/ProcessWebhook": true, // Webhook FedaPay (vérifié par HMAC)
	"/payment.PaymentService/RequestRefund":  true, // Route interne (booking-service)
	"/payment.PaymentService/ReleasePayment": true, // Route interne (booking-service)
	// Retour de la page FedaPay (gRPC uniquement, via le handler HTTP brut de l'api-gateway) :
	// public car le navigateur n'a pas de JWT ; le statut est toujours revérifié auprès de FedaPay.
	"/payment.PaymentService/HandlePaymentCallback": true,
	// Route interne (booking-service, gRPC uniquement — pas d'annotation HTTP)
	"/payment.PaymentService/CancelPaymentForBooking": true,
	// Route interne (sweeper paymentPending de booking-service) — GetPaymentByBooking exige l'identité du passager
	"/payment.PaymentService/GetPaymentByBookingInternal": true,
	// Routes internes (auth-service, suppression de compte — pas d'annotation HTTP)
	"/payment.PaymentService/CheckDeletionEligibility": true,
	"/payment.PaymentService/AnonymizeUserData":        true,
	"/grpc.health.v1.Health/Check":                     true, // Readiness probe Kubernetes
	"/grpc.health.v1.Health/Watch":                     true, // Liveness probe Kubernetes
}

// Routes gRPC protégées par JWT support (x-support-uid injecté par l'api-gateway)
var supportMethods = map[string]bool{
	"/payment.PaymentService/TriggerManualPayout": true,
	"/payment.PaymentService/GetPaymentHistory":   true,
	"/payment.PaymentService/GetRefundHistory":    true,
	"/payment.PaymentService/GetPayoutHistory":    true,
}

// PaymentInterceptor retourne un intercepteur gRPC unaire qui :
//  1. Laisse passer les routes publiques et internes (Health, ProcessWebhook, RequestRefund, ReleasePayment)
//  2. Pour les routes support, extrait x-support-uid et l'injecte via SupportUIDKey
//  3. Pour les autres routes, extrait le Firebase UID depuis x-firebase-uid, résout l'UserID interne
//     via user-service et injecte FirebaseIDKey + UserIDKey dans le contexte
func PaymentInterceptor(secret []byte, resolver UserIDResolver) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if publicMethods[info.FullMethod] {
			return handler(ctx, req)
		}

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing metadata")
		}

		if supportMethods[info.FullMethod] {
			supportUIDs := md.Get("x-support-uid")
			if len(supportUIDs) == 0 || supportUIDs[0] == "" {
				return nil, status.Error(codes.Unauthenticated, "missing support uid")
			}
			ctx = context.WithValue(ctx, SupportUIDKey, supportUIDs[0])
			if roles := md.Get("x-support-role"); len(roles) > 0 {
				ctx = context.WithValue(ctx, SupportRoleKey, roles[0])
			}
			return handler(ctx, req)
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
		if errors.Is(err, paymentErrors.ErrorUserNotFound) {
			return nil, status.Error(codes.PermissionDenied, "user not found")
		}
		return nil, status.Error(codes.Unavailable, "user resolution failed")
	}
	if userID == "" {
		return nil, status.Error(codes.PermissionDenied, "user not found")
	}

	ctx = context.WithValue(ctx, FirebaseIDKey, uids[0])
	ctx = context.WithValue(ctx, UserIDKey, userID)
	return ctx, nil
}
