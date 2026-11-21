package middleware

import (
	"context"
	"errors"

	grpcutil "github.com/Kpeewu/tissi-mah/pkg/grpcutil"
	fileErrors "github.com/Kpeewu/tissi-mah/services/file-service/pkg/errors"
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
// tout contrôle d'appartenance (les documents référencent leur propriétaire par UserID interne).
const UserIDKey contextKey = "userID"

// UserIDResolver résout un Firebase UID en UserID interne (implémenté par le client user-service).
type UserIDResolver interface {
	GetUserIDByFirebaseID(ctx context.Context, firebaseUID string) (string, error)
}

// Routes gRPC internes (pas de JWT requis) : tous les RPC sans annotation HTTP, appelés
// sans metadata par kyc-service, user-service, vehicle-service, auth-service et
// moderation-service. Une route HTTP n'a rien à faire ici (cf. test de cohérence).
var publicMethods = map[string]bool{
	"/file.FileService/Health":                      true,
	"/file.FileService/GetUserDocuments":            true,
	"/file.FileService/GetUserDocument":             true,
	"/file.FileService/GetCurrentUserDocument":      true,
	"/file.FileService/GetVehicleDocuments":         true,
	"/file.FileService/GetVehicleDocument":          true,
	"/file.FileService/GetVehicleDocumentsByUserID": true,
	"/file.FileService/ListKycDocuments":            true,
	"/file.FileService/DeleteUserDocument":          true,
	"/file.FileService/DeleteVehicleDocument":       true,
	"/file.FileService/CreateDocumentReview":        true,
	"/file.FileService/GetDocumentReview":           true,
	"/file.FileService/GetDocumentReviews":          true,
	"/file.FileService/GetDocumentReviewsByUserID":  true,
	"/file.FileService/UpdateDocumentReview":        true,
	"/file.FileService/ListDocumentReviews":         true,
	"/file.FileService/GetDocumentReviewHistory":    true,
	"/file.FileService/DeleteAllUserFiles":          true, // auth-service, suppression de compte
	"/grpc.health.v1.Health/Check":                  true, // Readiness probe Kubernetes
	"/grpc.health.v1.Health/Watch":                  true, // Liveness probe Kubernetes
}

// Routes ouvertes aux agents support (JWT support, x-support-uid) en plus des utilisateurs Firebase
// (DualProtectedRoutes côté api-gateway).
var supportMethods = map[string]bool{
	"/file.FileService/GetDocument": true,
}

// FileServiceInterceptor retourne un intercepteur gRPC unaire qui :
//  1. Laisse passer les routes internes (sans annotation HTTP)
//  2. Laisse passer un agent support (x-support-uid) sur les routes qui l'acceptent
//  3. Pour les autres routes, extrait le Firebase UID depuis x-firebase-uid, résout l'UserID
//     interne via user-service et injecte FirebaseIDKey + UserIDKey dans le contexte
func FileServiceInterceptor(secret []byte, resolver UserIDResolver) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if publicMethods[info.FullMethod] {
			return handler(ctx, req)
		}

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing metadata")
		}

		if supportMethods[info.FullMethod] {
			if suids := md.Get("x-support-uid"); len(suids) > 0 && suids[0] != "" {
				return handler(ctx, req)
			}
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
		if errors.Is(err, fileErrors.ErrorUserNotFound) {
			return nil, status.Error(codes.PermissionDenied, "user not found")
		}
		return nil, status.Error(codes.Unavailable, "user resolution failed")
	}

	ctx = context.WithValue(ctx, FirebaseIDKey, uids[0])
	ctx = context.WithValue(ctx, UserIDKey, userID)
	return ctx, nil
}
