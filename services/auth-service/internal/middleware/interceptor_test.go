package middleware

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// dummyHandler est un handler gRPC qui retourne le contexte reçu
func dummyHandler(ctx context.Context, req interface{}) (interface{}, error) {
	return ctx, nil
}

// setupProtectedMethods configure protectedMethods pour les tests et le restaure après
func setupProtectedMethods(t *testing.T, methods map[string]bool) {
	original := protectedMethods
	protectedMethods = methods
	t.Cleanup(func() {
		protectedMethods = original
	})
}

func TestAuthInterceptor_PublicRoute(t *testing.T) {
	setupProtectedMethods(t, map[string]bool{
		"/auth.AuthService/CreateAccount": true,
	})

	interceptor := AuthInterceptor()
	info := &grpc.UnaryServerInfo{FullMethod: "/auth.AuthService/Health"}

	resp, err := interceptor(context.Background(), nil, info, dummyHandler)

	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestAuthInterceptor_ProtectedRoute_Success(t *testing.T) {
	setupProtectedMethods(t, map[string]bool{
		"/auth.AuthService/CreateAccount": true,
	})

	interceptor := AuthInterceptor()
	info := &grpc.UnaryServerInfo{FullMethod: "/auth.AuthService/CreateAccount"}

	// Créer un contexte avec la metadata x-firebase-uid
	md := metadata.Pairs("x-firebase-uid", "test-uid-123")
	ctx := metadata.NewIncomingContext(context.Background(), md)

	resp, err := interceptor(ctx, nil, info, func(ctx context.Context, req interface{}) (interface{}, error) {
		// Vérifier que le FirebaseIDKey est injecté dans le contexte
		uid, ok := ctx.Value(FirebaseIDKey).(string)
		assert.True(t, ok)
		assert.Equal(t, "test-uid-123", uid)
		return "ok", nil
	})

	require.NoError(t, err)
	assert.Equal(t, "ok", resp)
}

func TestAuthInterceptor_ProtectedRoute_MissingMetadata(t *testing.T) {
	setupProtectedMethods(t, map[string]bool{
		"/auth.AuthService/CreateAccount": true,
	})

	interceptor := AuthInterceptor()
	info := &grpc.UnaryServerInfo{FullMethod: "/auth.AuthService/CreateAccount"}

	// Contexte sans metadata
	resp, err := interceptor(context.Background(), nil, info, dummyHandler)

	assert.Nil(t, resp)
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, st.Code())
	assert.Contains(t, st.Message(), "missing metadata")
}

func TestAuthInterceptor_ProtectedRoute_MissingUID(t *testing.T) {
	setupProtectedMethods(t, map[string]bool{
		"/auth.AuthService/CreateAccount": true,
	})

	interceptor := AuthInterceptor()
	info := &grpc.UnaryServerInfo{FullMethod: "/auth.AuthService/CreateAccount"}

	// Metadata présente mais sans x-firebase-uid
	md := metadata.Pairs("other-header", "value")
	ctx := metadata.NewIncomingContext(context.Background(), md)

	resp, err := interceptor(ctx, nil, info, dummyHandler)

	assert.Nil(t, resp)
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, st.Code())
	assert.Contains(t, st.Message(), "missing firebase uid")
}

func TestAuthInterceptor_ProtectedRoute_EmptyUID(t *testing.T) {
	setupProtectedMethods(t, map[string]bool{
		"/auth.AuthService/CreateAccount": true,
	})

	interceptor := AuthInterceptor()
	info := &grpc.UnaryServerInfo{FullMethod: "/auth.AuthService/CreateAccount"}

	// Metadata avec x-firebase-uid vide
	md := metadata.Pairs("x-firebase-uid", "")
	ctx := metadata.NewIncomingContext(context.Background(), md)

	resp, err := interceptor(ctx, nil, info, dummyHandler)

	assert.Nil(t, resp)
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, st.Code())
	assert.Contains(t, st.Message(), "missing firebase uid")
}
