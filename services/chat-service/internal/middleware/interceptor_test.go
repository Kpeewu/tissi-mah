package middleware

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Les routes internes sont appelées sans metadata x-firebase-uid (appels inter-services) :
// l'intercepteur doit les laisser passer.
func TestChatInterceptor_InternalRoutesWithoutMetadata(t *testing.T) {
	interceptor := ChatInterceptor(nil)
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
}

func TestChatInterceptor_ProtectedRouteWithoutMetadata(t *testing.T) {
	interceptor := ChatInterceptor(nil)
	info := &grpc.UnaryServerInfo{FullMethod: "/chat.ChatService/SendMessage"}

	_, err := interceptor(context.Background(), nil, info, func(_ context.Context, _ interface{}) (interface{}, error) {
		t.Fatal("le handler ne doit pas être appelé")
		return nil, nil
	})

	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.Unauthenticated, st.Code())
}
