package middleware_test

import (
	"context"
	"testing"

	"github.com/Kpeewu/tissi-mah/services/vehicle-service/internal/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// call fait passer une requête par l'intercepteur, sans utilisateur final (comme un
// appel entre services), et indique si le handler a été atteint.
func call(t *testing.T, method string) (reached bool, err error) {
	t.Helper()
	interceptor := middleware.VehicleInterceptor(nil)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.MD{})
	_, err = interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: method},
		func(context.Context, interface{}) (interface{}, error) {
			reached = true
			return nil, nil
		})
	return reached, err
}

// Cas constaté sur VPS Dev le 22/09 : kyc-service propage la vérification d'un
// véhicule sans utilisateur final ; l'appel était rejeté et aucun véhicule ne
// passait jamais vérifié, ce qui empêchait tout conducteur de publier un trajet.
func TestVehicleInterceptor_SetVehicleVerificationAccepteLesAppelsInterServices(t *testing.T) {
	reached, err := call(t, "/vehicle.VehicleService/SetVehicleVerification")
	require.NoError(t, err)
	assert.True(t, reached)
}

func TestVehicleInterceptor_LesAutresMethodesExigentUnUtilisateur(t *testing.T) {
	for _, method := range []string{
		"/vehicle.VehicleService/AddVehicle",
		"/vehicle.VehicleService/UpdateVehicle",
		"/vehicle.VehicleService/DeleteVehicle",
		"/vehicle.VehicleService/GetUserVehicles",
	} {
		reached, err := call(t, method)
		assert.False(t, reached, method)
		assert.Equal(t, codes.Unauthenticated, status.Code(err), method)
	}
}
