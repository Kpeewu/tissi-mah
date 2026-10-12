package middleware_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/Kpeewu/tissi-mah/services/notification-service/internal/middleware"
	notifErrors "github.com/Kpeewu/tissi-mah/services/notification-service/pkg/errors"
)

// stubResolver résout un Firebase UID en UserID, ou renvoie err.
type stubResolver struct {
	userID string
	err    error
	calls  int
}

func (r *stubResolver) GetUserIDByFirebaseID(_ context.Context, _ string) (string, error) {
	r.calls++
	return r.userID, r.err
}

var protectedInfo = &grpc.UnaryServerInfo{FullMethod: "/notification.NotificationService/RegisterDeviceToken"}

func firebaseCtx() context.Context {
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-firebase-uid", "fb-1"))
}

func failHandler(t *testing.T) grpc.UnaryHandler {
	return func(_ context.Context, _ interface{}) (interface{}, error) {
		t.Fatal("le handler ne doit pas être appelé")
		return nil, nil
	}
}

func TestNotificationInterceptor_ResolvesUserID(t *testing.T) {
	interceptor := middleware.NotificationInterceptor(zap.NewNop(), nil, &stubResolver{userID: "user-1"})

	var gotUserID string
	handler := func(ctx context.Context, _ interface{}) (interface{}, error) {
		if v, ok := ctx.Value(middleware.UserIDKey).(string); ok {
			gotUserID = v
		}
		return "ok", nil
	}
	_, err := interceptor(firebaseCtx(), nil, protectedInfo, handler)

	require.NoError(t, err)
	assert.Equal(t, "user-1", gotUserID)
}

// Inscription en cours : user-service ne connaît pas encore le Firebase UID → NotFound
// (le client réessaie plus tard) plutôt qu'une erreur générique.
func TestNotificationInterceptor_UserNotProvisioned_NotFound(t *testing.T) {
	resolver := &stubResolver{err: fmt.Errorf("%w: firebaseUID fb-1", notifErrors.ErrorUserNotProvisioned)}
	interceptor := middleware.NotificationInterceptor(zap.NewNop(), nil, resolver)

	_, err := interceptor(firebaseCtx(), nil, protectedInfo, failHandler(t))

	assert.Equal(t, codes.NotFound, status.Code(err))
}

func TestNotificationInterceptor_UserServiceDown_Unavailable(t *testing.T) {
	interceptor := middleware.NotificationInterceptor(zap.NewNop(), nil, &stubResolver{err: errors.New("down")})

	_, err := interceptor(firebaseCtx(), nil, protectedInfo, failHandler(t))

	assert.Equal(t, codes.Unavailable, status.Code(err))
}

func TestNotificationInterceptor_InternalRouteSkipsResolution(t *testing.T) {
	resolver := &stubResolver{}
	interceptor := middleware.NotificationInterceptor(zap.NewNop(), nil, resolver)
	info := &grpc.UnaryServerInfo{FullMethod: "/notification.NotificationService/InvalidateDeviceToken"}

	okHandler := func(_ context.Context, _ interface{}) (interface{}, error) { return "ok", nil }
	_, err := interceptor(context.Background(), nil, info, okHandler)

	require.NoError(t, err)
	assert.Zero(t, resolver.calls)
}
