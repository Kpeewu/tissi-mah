package middleware

import (
	"context"
	"errors"
	"testing"

	grpcutil "github.com/Kpeewu/tissi-mah/pkg/grpcutil"
	paymentErrors "github.com/Kpeewu/tissi-mah/services/payment-service/pkg/errors"
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
		return "", paymentErrors.ErrorUserNotFound
	}
	return userID, nil
}

func firebaseCtx(md ...string) context.Context {
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs(md...))
}

func failHandler(t *testing.T) grpc.UnaryHandler {
	return func(_ context.Context, _ interface{}) (interface{}, error) {
		t.Fatal("le handler ne doit pas être appelé")
		return nil, nil
	}
}

var protectedInfo = &grpc.UnaryServerInfo{FullMethod: "/payment.PaymentService/CreatePayment"}

// Les routes internes sont appelées sans metadata x-firebase-uid (appels inter-services) :
// l'intercepteur doit les laisser passer sans solliciter user-service.
func TestPaymentInterceptor_InternalRoutesWithoutMetadata(t *testing.T) {
	resolver := &stubResolver{}
	interceptor := PaymentInterceptor(nil, resolver)
	internalMethods := []string{
		"/payment.PaymentService/CheckDeletionEligibility",
		"/payment.PaymentService/AnonymizeUserData",
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

func TestPaymentInterceptor_ProtectedRouteWithoutMetadata(t *testing.T) {
	interceptor := PaymentInterceptor(nil, &stubResolver{})

	_, err := interceptor(context.Background(), nil, protectedInfo, failHandler(t))

	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.Unauthenticated, st.Code())
}

func TestPaymentInterceptor_ResolvesUserID(t *testing.T) {
	interceptor := PaymentInterceptor(nil, &stubResolver{users: map[string]string{"fb-1": "user-1"}})

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
	_, err := interceptor(firebaseCtx("x-firebase-uid", "fb-1"), nil, protectedInfo, handler)

	require.NoError(t, err)
	assert.Equal(t, "user-1", gotUserID)
	assert.Equal(t, "fb-1", gotFirebaseID)
}

func TestPaymentInterceptor_ResolutionErrors(t *testing.T) {
	tests := []struct {
		name     string
		resolver *stubResolver
		wantCode codes.Code
	}{
		{"utilisateur inconnu", &stubResolver{users: map[string]string{}}, codes.PermissionDenied},
		{"UserID vide", &stubResolver{users: map[string]string{"fb-1": ""}}, codes.PermissionDenied},
		{"user-service indisponible", &stubResolver{err: errors.New("unavailable")}, codes.Unavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			interceptor := PaymentInterceptor(nil, tt.resolver)

			_, err := interceptor(firebaseCtx("x-firebase-uid", "fb-1"), nil, protectedInfo, failHandler(t))

			st, _ := status.FromError(err)
			assert.Equal(t, tt.wantCode, st.Code())
		})
	}
}

func TestPaymentInterceptor_InvalidSignature(t *testing.T) {
	resolver := &stubResolver{users: map[string]string{"fb-1": "user-1"}}
	interceptor := PaymentInterceptor([]byte("secret"), resolver)

	ctx := firebaseCtx("x-firebase-uid", "fb-1", grpcutil.MetadataUIDSig, "bad-signature")
	_, err := interceptor(ctx, nil, protectedInfo, failHandler(t))

	st, _ := status.FromError(err)
	assert.Equal(t, codes.Unauthenticated, st.Code())
	assert.Zero(t, resolver.calls, "pas de résolution pour une identité non authentifiée")
}

// Les routes support s'appuient sur x-support-uid, sans résolution Firebase.
func TestPaymentInterceptor_SupportRouteSkipsResolution(t *testing.T) {
	resolver := &stubResolver{}
	interceptor := PaymentInterceptor(nil, resolver)
	info := &grpc.UnaryServerInfo{FullMethod: "/payment.PaymentService/GetPaymentHistory"}

	var gotSupportUID string
	handler := func(ctx context.Context, _ interface{}) (interface{}, error) {
		if v, ok := ctx.Value(SupportUIDKey).(string); ok {
			gotSupportUID = v
		}
		return "ok", nil
	}
	_, err := interceptor(firebaseCtx("x-support-uid", "agent-1"), nil, info, handler)

	require.NoError(t, err)
	assert.Equal(t, "agent-1", gotSupportUID)
	assert.Zero(t, resolver.calls)
}
