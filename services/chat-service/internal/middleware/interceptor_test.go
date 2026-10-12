package middleware

import (
	"context"
	"errors"
	"testing"

	chatErrors "github.com/Kpeewu/tissi-mah/services/chat-service/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// stubResolver résout un Firebase UID en UserID à partir d'une table, ou renvoie err.
type stubResolver struct {
	users map[string]string
	err   error
	calls int
}

func (r *stubResolver) GetUserIDByFirebaseID(_ context.Context, firebaseUID string) (string, error) {
	r.calls++
	if r.err != nil {
		return "", r.err
	}
	userID, ok := r.users[firebaseUID]
	if !ok {
		return "", chatErrors.ErrUserNotFound
	}
	return userID, nil
}

var protectedInfo = &grpc.UnaryServerInfo{FullMethod: "/chat.ChatService/SendMessage"}

func failHandler(t *testing.T) grpc.UnaryHandler {
	return func(_ context.Context, _ interface{}) (interface{}, error) {
		t.Fatal("le handler ne doit pas être appelé")
		return nil, nil
	}
}

// Les routes internes sont appelées sans metadata x-firebase-uid (appels inter-services) :
// l'intercepteur doit les laisser passer sans solliciter user-service.
func TestChatInterceptor_InternalRoutesWithoutMetadata(t *testing.T) {
	resolver := &stubResolver{}
	interceptor := ChatInterceptor(nil, resolver)
	internalMethods := []string{
		"/chat.ChatService/AnonymizeUserData",
	}

	for _, method := range internalMethods {
		t.Run(method, func(t *testing.T) {
			info := &grpc.UnaryServerInfo{FullMethod: method}
			okHandler := func(_ context.Context, _ interface{}) (interface{}, error) { return "ok", nil }
			resp, err := interceptor(context.Background(), nil, info, okHandler)

			require.NoError(t, err)
			assert.Equal(t, "ok", resp)
		})
	}
	assert.Zero(t, resolver.calls)
}

func TestChatInterceptor_ProtectedRouteWithoutMetadata(t *testing.T) {
	interceptor := ChatInterceptor(nil, &stubResolver{})

	_, err := interceptor(context.Background(), nil, protectedInfo, failHandler(t))

	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.Unauthenticated, st.Code())
}

func TestChatInterceptor_ResolvesUserID(t *testing.T) {
	interceptor := ChatInterceptor(nil, &stubResolver{users: map[string]string{"fb-1": "user-1"}})

	var gotUserID string
	handler := func(ctx context.Context, _ interface{}) (interface{}, error) {
		if v, ok := ctx.Value(UserIDKey).(string); ok {
			gotUserID = v
		}
		return "ok", nil
	}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-firebase-uid", "fb-1"))
	_, err := interceptor(ctx, nil, protectedInfo, handler)

	require.NoError(t, err)
	assert.Equal(t, "user-1", gotUserID)
}

func TestChatInterceptor_ResolutionErrors(t *testing.T) {
	tests := []struct {
		name     string
		resolver *stubResolver
		wantCode codes.Code
	}{
		{"utilisateur inconnu", &stubResolver{users: map[string]string{}}, codes.PermissionDenied},
		{"user-service indisponible", &stubResolver{err: errors.New("unavailable")}, codes.Unavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			interceptor := ChatInterceptor(nil, tt.resolver)
			ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-firebase-uid", "fb-1"))

			_, err := interceptor(ctx, nil, protectedInfo, failHandler(t))

			assert.Equal(t, tt.wantCode, status.Code(err))
		})
	}
}

// Les routes support reposent sur x-support-uid, sans résolution Firebase.
func TestChatInterceptor_SupportRouteSkipsResolution(t *testing.T) {
	resolver := &stubResolver{}
	interceptor := ChatInterceptor(nil, resolver)
	info := &grpc.UnaryServerInfo{FullMethod: "/chat.ChatService/GetFlaggedMessageContent"}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-support-uid", "agent-1"))

	_, err := interceptor(ctx, nil, info, func(_ context.Context, _ interface{}) (interface{}, error) { return "ok", nil })

	require.NoError(t, err)
	assert.Zero(t, resolver.calls)
}
