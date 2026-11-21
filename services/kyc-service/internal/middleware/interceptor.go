package middleware

import (
	"context"
	"errors"

	grpcutil "github.com/Kpeewu/tissi-mah/pkg/grpcutil"
	kycErrors "github.com/Kpeewu/tissi-mah/services/kyc-service/pkg/errors"
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
// résolu depuis le Firebase UID via user-service (le file-service stocke les documents
// avec l'UserID interne, jamais avec le Firebase UID).
const UserIDKey contextKey = "userID"

// UserIDResolver résout un Firebase UID en UserID interne (implémenté par le client user-service).
type UserIDResolver interface {
	GetUserIDByFirebaseID(ctx context.Context, firebaseUID string) (string, error)
}

// SupportIDKey est la clé du contexte gRPC où est stocké l'UID de l'agent support,
// injecté par l'api-gateway via la metadata gRPC x-support-uid.
const SupportIDKey contextKey = "supportID"

// SupportRoleKey est la clé du contexte gRPC où est stocké le rôle de l'agent support,
// injecté par l'api-gateway via la metadata gRPC x-support-role.
const SupportRoleKey contextKey = "supportRole"

// Routes gRPC publiques (pas de JWT requis)
var publicMethods = map[string]bool{
	"/kyc.KYCService/Health":       true,
	"/grpc.health.v1.Health/Check": true, // Readiness probe Kubernetes
	"/grpc.health.v1.Health/Watch": true, // Liveness probe Kubernetes
}

// Méthodes réservées aux agents support (JWT support, pas Firebase)
var adminMethods = map[string]bool{
	"/kyc.KYCService/GetAdminReviews":              true,
	"/kyc.KYCService/GetAdminReview":               true,
	"/kyc.KYCService/OverrideReview":               true,
	"/kyc.KYCService/ValidateDocument":             true,
	"/kyc.KYCService/GetManualReviewRequests":      true,
	"/kyc.KYCService/GetManualReviewRequestDetail": true,
	"/kyc.KYCService/GetDocumentHistory":           true,
}

// adminOnlyMethods : sous-ensemble des méthodes admin réservé au rôle "admin".
// L'override d'une décision (annulation d'un rejet) est une action sensible.
var adminOnlyMethods = map[string]bool{
	"/kyc.KYCService/OverrideReview": true,
}

// allowedSupportRoles : rôles support acceptés sur les méthodes admin.
var allowedSupportRoles = map[string]bool{
	"admin":   true,
	"support": true,
}

// KYCInterceptor retourne un intercepteur gRPC unaire qui :
//  1. Laisse passer les routes publiques (Health)
//  2. Pour les méthodes admin : extrait x-support-uid → SupportIDKey et
//     vérifie le rôle (x-support-role) — OverrideReview exige "admin"
//  3. Pour les autres méthodes : extrait x-firebase-uid, résout l'UserID interne via
//     user-service et injecte FirebaseIDKey + UserIDKey dans le contexte
func KYCInterceptor(secret []byte, resolver UserIDResolver) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if publicMethods[info.FullMethod] {
			return handler(ctx, req)
		}

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing metadata")
		}

		if adminMethods[info.FullMethod] {
			uids := md.Get("x-support-uid")
			if len(uids) == 0 || uids[0] == "" {
				return nil, status.Error(codes.Unauthenticated, "missing support uid")
			}
			var role string
			if roles := md.Get("x-support-role"); len(roles) > 0 {
				role = roles[0]
			}
			if !allowedSupportRoles[role] {
				return nil, status.Error(codes.PermissionDenied, "invalid support role")
			}
			if adminOnlyMethods[info.FullMethod] && role != "admin" {
				return nil, status.Error(codes.PermissionDenied, "admin role required")
			}
			ctx = context.WithValue(ctx, SupportIDKey, uids[0])
			ctx = context.WithValue(ctx, SupportRoleKey, role)
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
		if errors.Is(err, kycErrors.ErrorUserNotFound) {
			return nil, status.Error(codes.PermissionDenied, "user not found")
		}
		return nil, status.Error(codes.Unavailable, "user resolution failed")
	}

	ctx = context.WithValue(ctx, FirebaseIDKey, uids[0])
	ctx = context.WithValue(ctx, UserIDKey, userID)
	return ctx, nil
}
