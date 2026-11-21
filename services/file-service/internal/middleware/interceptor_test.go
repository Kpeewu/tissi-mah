package middleware

import (
	"context"
	"errors"
	"fmt"
	"testing"

	fileErrors "github.com/Kpeewu/tissi-mah/services/file-service/pkg/errors"
	filepb "github.com/Kpeewu/tissi-mah/services/file-service/proto/gen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
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
		return "", fileErrors.ErrorUserNotFound
	}
	return userID, nil
}

func failHandler(t *testing.T) grpc.UnaryHandler {
	return func(_ context.Context, _ interface{}) (interface{}, error) {
		t.Fatal("le handler ne doit pas être appelé")
		return nil, nil
	}
}

func info(method string) *grpc.UnaryServerInfo {
	return &grpc.UnaryServerInfo{FullMethod: "/file.FileService/" + method}
}

// Garde-fou : un RPC sans annotation HTTP est interne (appelé sans metadata par les autres
// services) et doit figurer dans publicMethods ; un RPC exposé en HTTP ne doit jamais y figurer.
func TestPublicMethods_MatchHTTPAnnotations(t *testing.T) {
	methods := filepb.File_file_proto.Services().ByName("FileService").Methods()
	for i := 0; i < methods.Len(); i++ {
		m := methods.Get(i)
		if m.IsStreamingClient() || m.IsStreamingServer() {
			continue // hors intercepteur unaire
		}
		fullMethod := fmt.Sprintf("/file.FileService/%s", m.Name())
		rule, ok := proto.GetExtension(m.Options(), annotations.E_Http).(*annotations.HttpRule)
		if !ok || rule == nil {
			assert.True(t, publicMethods[fullMethod], "%s est interne : à ajouter dans publicMethods", fullMethod)
		} else {
			assert.False(t, publicMethods[fullMethod], "%s est exposé en HTTP : il ne doit pas être public", fullMethod)
		}
	}
}

func TestFileServiceInterceptor_InternalRouteWithoutMetadata(t *testing.T) {
	resolver := &stubResolver{}
	interceptor := FileServiceInterceptor(nil, resolver)

	resp, err := interceptor(context.Background(), nil, info("GetCurrentUserDocument"),
		func(_ context.Context, _ interface{}) (interface{}, error) { return "ok", nil })

	require.NoError(t, err)
	assert.Equal(t, "ok", resp)
	assert.Zero(t, resolver.calls)
}

func TestFileServiceInterceptor_ProtectedRouteWithoutMetadata(t *testing.T) {
	interceptor := FileServiceInterceptor(nil, &stubResolver{})

	_, err := interceptor(context.Background(), nil, info("DeleteFile"), failHandler(t))

	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestFileServiceInterceptor_ResolvesUserID(t *testing.T) {
	interceptor := FileServiceInterceptor(nil, &stubResolver{users: map[string]string{"fb-1": "user-1"}})

	var gotUserID, gotFirebaseID string
	handler := func(ctx context.Context, _ interface{}) (interface{}, error) {
		if v, ok := ctx.Value(UserIDKey).(string); ok {
			gotUserID = v
		}
		if v, ok := ctx.Value(FirebaseIDKey).(string); ok {
			gotFirebaseID = v
		}
		return "ok", nil
	}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-firebase-uid", "fb-1"))
	_, err := interceptor(ctx, nil, info("ChangeDocument"), handler)

	require.NoError(t, err)
	assert.Equal(t, "user-1", gotUserID)
	assert.Equal(t, "fb-1", gotFirebaseID)
}

// GetDocument est ouvert aux agents support (x-support-uid), sans résolution Firebase.
func TestFileServiceInterceptor_SupportOnGetDocument(t *testing.T) {
	resolver := &stubResolver{}
	interceptor := FileServiceInterceptor(nil, resolver)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-support-uid", "agent-1"))

	resp, err := interceptor(ctx, nil, info("GetDocument"),
		func(_ context.Context, _ interface{}) (interface{}, error) { return "ok", nil })

	require.NoError(t, err)
	assert.Equal(t, "ok", resp)
	assert.Zero(t, resolver.calls)
}

// x-support-uid n'ouvre que les routes support : ailleurs, l'identité Firebase reste exigée.
func TestFileServiceInterceptor_SupportUIDIgnoredElsewhere(t *testing.T) {
	interceptor := FileServiceInterceptor(nil, &stubResolver{})
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-support-uid", "agent-1"))

	_, err := interceptor(ctx, nil, info("DeleteFile"), failHandler(t))

	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestFileServiceInterceptor_ResolutionErrors(t *testing.T) {
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
			interceptor := FileServiceInterceptor(nil, tt.resolver)
			ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-firebase-uid", "fb-1"))

			_, err := interceptor(ctx, nil, info("DeleteFile"), failHandler(t))

			assert.Equal(t, tt.wantCode, status.Code(err))
		})
	}
}
