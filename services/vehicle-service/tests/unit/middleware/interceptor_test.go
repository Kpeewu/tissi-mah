package middleware_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Kpeewu/tissi-mah/services/vehicle-service/internal/middleware"
	vehicleErrors "github.com/Kpeewu/tissi-mah/services/vehicle-service/pkg/errors"
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
		return "", vehicleErrors.ErrorUserNotFound
	}
	return userID, nil
}

// call fait passer une requête par l'intercepteur avec la metadata md et indique si le
// handler a été atteint, ainsi que l'UserID qu'il a reçu dans son contexte.
func call(
	t *testing.T, resolver middleware.UserIDResolver, method string, md metadata.MD,
) (reached bool, userID string, err error) {
	t.Helper()
	interceptor := middleware.VehicleInterceptor(nil, resolver)
	ctx := metadata.NewIncomingContext(context.Background(), md)
	_, err = interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: method},
		func(ctx context.Context, _ interface{}) (interface{}, error) {
			reached = true
			if v, ok := ctx.Value(middleware.UserIDKey).(string); ok {
				userID = v
			}
			return nil, nil
		})
	return reached, userID, err
}

// Cas constaté sur VPS Dev le 22/09 : kyc-service propage la vérification d'un
// véhicule sans utilisateur final ; l'appel était rejeté et aucun véhicule ne
// passait jamais vérifié, ce qui empêchait tout conducteur de publier un trajet.
func TestVehicleInterceptor_SetVehicleVerificationAccepteLesAppelsInterServices(t *testing.T) {
	reached, _, err := call(t, &stubResolver{}, "/vehicle.VehicleService/SetVehicleVerification", metadata.MD{})
	require.NoError(t, err)
	assert.True(t, reached)
}

// trips-service et file-service lisent un véhicule (et son propriétaire) sans utilisateur final.
func TestVehicleInterceptor_GetVehicleInfoAccepteLesAppelsInterServices(t *testing.T) {
	resolver := &stubResolver{}
	reached, _, err := call(t, resolver, "/vehicle.VehicleService/GetVehicleInfo", metadata.MD{})
	require.NoError(t, err)
	assert.True(t, reached)
	assert.Zero(t, resolver.calls)
}

func TestVehicleInterceptor_LesAutresMethodesExigentUnUtilisateur(t *testing.T) {
	for _, method := range []string{
		"/vehicle.VehicleService/AddVehicle",
		"/vehicle.VehicleService/UpdateVehicle",
		"/vehicle.VehicleService/DeleteVehicle",
		"/vehicle.VehicleService/GetVehicleDetails",
		"/vehicle.VehicleService/GetUserVehicles",
	} {
		reached, _, err := call(t, &stubResolver{}, method, metadata.MD{})
		assert.False(t, reached, method)
		assert.Equal(t, codes.Unauthenticated, status.Code(err), method)
	}
}

func TestVehicleInterceptor_ResoutLUserIDDeLAppelant(t *testing.T) {
	resolver := &stubResolver{users: map[string]string{"fb-1": "user-1"}}
	reached, userID, err := call(t, resolver, "/vehicle.VehicleService/GetUserVehicles",
		metadata.Pairs("x-firebase-uid", "fb-1"))

	require.NoError(t, err)
	assert.True(t, reached)
	assert.Equal(t, "user-1", userID)
}

func TestVehicleInterceptor_EchecsDeResolution(t *testing.T) {
	cases := []struct {
		name     string
		resolver *stubResolver
		want     codes.Code
	}{
		{"utilisateur inconnu", &stubResolver{users: map[string]string{}}, codes.PermissionDenied},
		{"user-service indisponible", &stubResolver{err: errors.New("unavailable")}, codes.Unavailable},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			reached, _, err := call(t, c.resolver, "/vehicle.VehicleService/AddVehicle",
				metadata.Pairs("x-firebase-uid", "fb-1"))
			assert.False(t, reached)
			assert.Equal(t, c.want, status.Code(err))
		})
	}
}
