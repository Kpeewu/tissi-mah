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
func TestPaymentInterceptor_InternalRoutesWithoutMetadata(t *testing.T) {
	interceptor := PaymentInterceptor(nil)
	internalMethods := []string{
		"/payment.PaymentService/CheckDeletionEligibility",
		"/payment.PaymentService/AnonymizeUserData",
	}

	for _, method := range internalMethods {
		t.Run(method, func(t *testing.T) {
			info := &grpc.UnaryServerInfo{FullMethod: method}
			resp, err := interceptor(context.Background(), nil, info, func(ctx context.Context, req interface{}) (interface{}, error) {
				return "ok", nil
			})

			require.NoError(t, err)
			assert.Equal(t, "ok", resp)
		})
	}
}

func TestPaymentInterceptor_ProtectedRouteWithoutMetadata(t *testing.T) {
	interceptor := PaymentInterceptor(nil)
	info := &grpc.UnaryServerInfo{FullMethod: "/payment.PaymentService/CreatePayment"}

	_, err := interceptor(context.Background(), nil, info, func(ctx context.Context, req interface{}) (interface{}, error) {
		t.Fatal("le handler ne doit pas être appelé")
		return nil, nil
	})

	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.Unauthenticated, st.Code())
}
