package middleware

import (
	"context"

	supportErrors "github.com/Kpeewu/tissi-mah/services/support-service/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type contextKey string

const (
	SupportUIDKey  contextKey = "supportUID"
	SupportRoleKey contextKey = "supportRole"
)

const (
	headerSupportUID  = "x-support-uid"
	headerSupportRole = "x-support-role"
)

// publicMethods : routes publiques (pas besoin de JWT support).
var publicMethods = map[string]bool{
	"/support.SupportService/Login":        true,
	"/support.SupportService/VerifyOTP":    true,
	"/support.SupportService/ResendOTP":    true,
	"/support.SupportService/RefreshToken": true,
	"/support.SupportService/Health":       true,
	"/grpc.health.v1.Health/Check":         true,
	"/grpc.health.v1.Health/Watch":         true,
}

// SupportInterceptor lit x-support-uid / x-support-role depuis la metadata gRPC injectée par api-gateway
// et les place dans le context. Refuse les méthodes protégées sans UID.
func SupportInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if publicMethods[info.FullMethod] {
			return handler(ctx, req)
		}
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, supportErrors.ErrUnauthenticated
		}
		uids := md.Get(headerSupportUID)
		if len(uids) == 0 || uids[0] == "" {
			return nil, supportErrors.ErrUnauthenticated
		}
		ctx = context.WithValue(ctx, SupportUIDKey, uids[0])
		if roles := md.Get(headerSupportRole); len(roles) > 0 {
			ctx = context.WithValue(ctx, SupportRoleKey, roles[0])
		}
		return handler(ctx, req)
	}
}

// UIDFromContext récupère l'UID support depuis le context.
func UIDFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(SupportUIDKey).(string)
	return v, ok
}

// RoleFromContext récupère le rôle support depuis le context.
func RoleFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(SupportRoleKey).(string)
	return v, ok
}
