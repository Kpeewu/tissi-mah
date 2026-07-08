package handler_test

import (
	"context"
	"testing"

	"github.com/Kpeewu/tissi-mah/services/vehicle-service/internal/domain"
	grpcHandler "github.com/Kpeewu/tissi-mah/services/vehicle-service/internal/grpc"
	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/vehicle-service/internal/service/interfaces"
	vehicleErrors "github.com/Kpeewu/tissi-mah/services/vehicle-service/pkg/errors"
	vehiclepb "github.com/Kpeewu/tissi-mah/services/vehicle-service/proto/gen"
	"github.com/Kpeewu/tissi-mah/services/vehicle-service/tests/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func newMockAndHandler() (*mocks.MockVehicleService, *grpcHandler.VehicleHandler) {
	svc := new(mocks.MockVehicleService)
	handler := grpcHandler.NewVehicleHandler(svc, zap.NewNop())
	return svc, handler
}

func assertGRPCCode(t *testing.T, err error, want codes.Code) {
	t.Helper()
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, want, st.Code())
}

func TestAddVehicle_Handler(t *testing.T) {
	t.Run("succès", func(t *testing.T) {
		svc, h := newMockAndHandler()
		svc.On("AddVehicle", mock.Anything, mock.MatchedBy(func(in serviceInterfaces.AddVehicleInput) bool {
			return in.UserID == "u1" && in.Brand == "T" && in.LicencePlate == "AA" && in.NumberOfSeats == 4
		})).Return("veh-1", nil)

		resp, err := h.AddVehicle(context.Background(), &vehiclepb.AddVehicleRequest{
			UserId: "u1", Brand: "T", NumberOfSeats: 4, LicencePlate: "AA",
		})
		require.NoError(t, err)
		assert.Equal(t, "veh-1", resp.VehicleId)
	})

	t.Run("input invalide → InvalidArgument", func(t *testing.T) {
		svc, h := newMockAndHandler()
		svc.On("AddVehicle", mock.Anything, mock.Anything).Return("", vehicleErrors.ErrorInvalidInput)
		_, err := h.AddVehicle(context.Background(), &vehiclepb.AddVehicleRequest{})
		assertGRPCCode(t, err, codes.InvalidArgument)
	})

	t.Run("plaque conflit → AlreadyExists", func(t *testing.T) {
		svc, h := newMockAndHandler()
		svc.On("AddVehicle", mock.Anything, mock.Anything).Return("", vehicleErrors.ErrorLicencePlateConflict)
		_, err := h.AddVehicle(context.Background(), &vehiclepb.AddVehicleRequest{UserId: "u", Brand: "b", LicencePlate: "x"})
		assertGRPCCode(t, err, codes.AlreadyExists)
	})
}

func TestUpdateVehicle_Handler(t *testing.T) {
	t.Run("succès", func(t *testing.T) {
		svc, h := newMockAndHandler()
		svc.On("UpdateVehicle", mock.Anything, mock.MatchedBy(func(in serviceInterfaces.UpdateVehicleInput) bool {
			return in.VehicleID == "v1" && in.UserID == "u1" && in.Color == "Red"
		})).Return(nil)

		resp, err := h.UpdateVehicle(context.Background(), &vehiclepb.UpdateVehicleRequest{
			VehicleId: "v1", UserId: "u1", Color: "Red",
		})
		require.NoError(t, err)
		assert.True(t, resp.Success)
	})

	t.Run("unauthorized → PermissionDenied", func(t *testing.T) {
		svc, h := newMockAndHandler()
		svc.On("UpdateVehicle", mock.Anything, mock.Anything).Return(vehicleErrors.ErrorUnauthorized)
		_, err := h.UpdateVehicle(context.Background(), &vehiclepb.UpdateVehicleRequest{VehicleId: "v1", UserId: "u1"})
		assertGRPCCode(t, err, codes.PermissionDenied)
	})

	t.Run("not found → NotFound", func(t *testing.T) {
		svc, h := newMockAndHandler()
		svc.On("UpdateVehicle", mock.Anything, mock.Anything).Return(vehicleErrors.ErrorVehicleNotFound)
		_, err := h.UpdateVehicle(context.Background(), &vehiclepb.UpdateVehicleRequest{VehicleId: "v1", UserId: "u1"})
		assertGRPCCode(t, err, codes.NotFound)
	})
}

func TestDeleteVehicle_Handler(t *testing.T) {
	t.Run("succès", func(t *testing.T) {
		svc, h := newMockAndHandler()
		svc.On("DeleteVehicle", mock.Anything, "u1", "v1").Return(nil)
		resp, err := h.DeleteVehicle(context.Background(), &vehiclepb.DeleteVehicleRequest{UserId: "u1", VehicleId: "v1"})
		require.NoError(t, err)
		assert.True(t, resp.Success)
	})

	t.Run("non propriétaire → PermissionDenied", func(t *testing.T) {
		svc, h := newMockAndHandler()
		svc.On("DeleteVehicle", mock.Anything, mock.Anything, mock.Anything).
			Return(vehicleErrors.ErrorUnauthorized)
		_, err := h.DeleteVehicle(context.Background(), &vehiclepb.DeleteVehicleRequest{UserId: "u", VehicleId: "v"})
		assertGRPCCode(t, err, codes.PermissionDenied)
	})

	t.Run("invalid input → InvalidArgument", func(t *testing.T) {
		svc, h := newMockAndHandler()
		svc.On("DeleteVehicle", mock.Anything, mock.Anything, mock.Anything).
			Return(vehicleErrors.ErrorInvalidInput)
		_, err := h.DeleteVehicle(context.Background(), &vehiclepb.DeleteVehicleRequest{})
		assertGRPCCode(t, err, codes.InvalidArgument)
	})
}

func TestGetVehicleDetails_Handler(t *testing.T) {
	t.Run("succès - mappe tous les champs proto", func(t *testing.T) {
		svc, h := newMockAndHandler()
		details := &domain.VehicleDetails{
			Vehicle: &domain.Vehicle{
				VehicleID: "v1", UserID: "u1", Brand: "Toyota",
				NumberOfSeats: 4, BrandModel: "Corolla",
				Color: "Black", LicencePlate: "AA-1", IsVerified: true,
			},
			Documents: domain.VehicleDocuments{
				AssuranceURL: "a.url", VehicleRegistrationURL: "r.url",
			},
		}
		svc.On("GetVehicleDetails", mock.Anything, "u1", "v1").Return(details, nil)

		resp, err := h.GetVehicleDetails(context.Background(), &vehiclepb.GetVehicleDetailsRequest{UserId: "u1", VehicleId: "v1"})
		require.NoError(t, err)
		assert.Equal(t, "v1", resp.Vehicle.VehicleId)
		assert.Equal(t, "u1", resp.Vehicle.UserId)
		assert.Equal(t, "Toyota", resp.Vehicle.Brand)
		assert.Equal(t, int32(4), resp.Vehicle.NumberOfSeats)
		assert.True(t, resp.Vehicle.IsVerified)
		assert.Equal(t, "a.url", resp.Vehicle.Documents.AssuranceUrl)
		assert.Equal(t, "r.url", resp.Vehicle.Documents.VehicleRegistrationUrl)
	})

	t.Run("not found → NotFound", func(t *testing.T) {
		svc, h := newMockAndHandler()
		svc.On("GetVehicleDetails", mock.Anything, "u1", "bad").
			Return(nil, vehicleErrors.ErrorVehicleNotFound)
		_, err := h.GetVehicleDetails(context.Background(), &vehiclepb.GetVehicleDetailsRequest{UserId: "u1", VehicleId: "bad"})
		assertGRPCCode(t, err, codes.NotFound)
	})

	t.Run("non propriétaire → PermissionDenied", func(t *testing.T) {
		svc, h := newMockAndHandler()
		svc.On("GetVehicleDetails", mock.Anything, "u1", "v1").
			Return(nil, vehicleErrors.ErrorUnauthorized)
		_, err := h.GetVehicleDetails(context.Background(), &vehiclepb.GetVehicleDetailsRequest{UserId: "u1", VehicleId: "v1"})
		assertGRPCCode(t, err, codes.PermissionDenied)
	})
}

func TestGetUserVehicles_Handler(t *testing.T) {
	t.Run("succès - mappe previews avec statuts et TripCount", func(t *testing.T) {
		svc, h := newMockAndHandler()
		svc.On("GetUserVehicles", mock.Anything, "u1").Return([]*domain.VehiclePreview{
			{
				VehicleID: "v1", Brand: "T", BrandModel: "C", LicencePlate: "AA", IsVerified: true,
				AssuranceStatus: "PENDING", VehicleRegistrationStatus: "VALIDATED",
				DriverLicenceStatus: "MISSING", TripCount: 3,
			},
			{VehicleID: "v2", Brand: "T2", BrandModel: "C2", LicencePlate: "BB", IsVerified: false},
		}, nil)

		resp, err := h.GetUserVehicles(context.Background(), &vehiclepb.GetUserVehiclesRequest{UserId: "u1"})
		require.NoError(t, err)
		require.Len(t, resp.Vehicles, 2)
		assert.Equal(t, "v1", resp.Vehicles[0].VehicleId)
		assert.True(t, resp.Vehicles[0].IsVerified)
		assert.Equal(t, "PENDING", resp.Vehicles[0].AssuranceStatus)
		assert.Equal(t, "VALIDATED", resp.Vehicles[0].VehicleRegistrationStatus)
		assert.Equal(t, "MISSING", resp.Vehicles[0].DriverLicenceStatus)
		assert.Equal(t, int32(3), resp.Vehicles[0].TripCount)
		assert.Equal(t, "v2", resp.Vehicles[1].VehicleId)
	})

	t.Run("liste vide", func(t *testing.T) {
		svc, h := newMockAndHandler()
		svc.On("GetUserVehicles", mock.Anything, "u1").Return([]*domain.VehiclePreview{}, nil)
		resp, err := h.GetUserVehicles(context.Background(), &vehiclepb.GetUserVehiclesRequest{UserId: "u1"})
		require.NoError(t, err)
		assert.Empty(t, resp.Vehicles)
	})

	t.Run("invalid input → InvalidArgument", func(t *testing.T) {
		svc, h := newMockAndHandler()
		svc.On("GetUserVehicles", mock.Anything, "").Return(nil, vehicleErrors.ErrorInvalidInput)
		_, err := h.GetUserVehicles(context.Background(), &vehiclepb.GetUserVehiclesRequest{UserId: ""})
		assertGRPCCode(t, err, codes.InvalidArgument)
	})

	t.Run("data retrieval failed → Internal", func(t *testing.T) {
		svc, h := newMockAndHandler()
		svc.On("GetUserVehicles", mock.Anything, "u1").
			Return(nil, vehicleErrors.ErrorDataRetrievalFailed)
		_, err := h.GetUserVehicles(context.Background(), &vehiclepb.GetUserVehiclesRequest{UserId: "u1"})
		assertGRPCCode(t, err, codes.Internal)
	})
}

func TestHealth_Handler(t *testing.T) {
	_, h := newMockAndHandler()
	resp, err := h.Health(context.Background(), &vehiclepb.HealthRequest{})
	require.NoError(t, err)
	assert.Equal(t, "healthy", resp.Status)
	assert.NotEmpty(t, resp.Version)
	assert.NotZero(t, resp.Timestamp)
}

// Audit complet du mapping toGRPCError (via AddVehicle comme point d'entrée).
func TestErrorMapping_Handler(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want codes.Code
	}{
		{"VehicleNotFound", vehicleErrors.ErrorVehicleNotFound, codes.NotFound},
		{"LicencePlateConflict", vehicleErrors.ErrorLicencePlateConflict, codes.AlreadyExists},
		{"Unauthorized", vehicleErrors.ErrorUnauthorized, codes.PermissionDenied},
		{"InvalidInput", vehicleErrors.ErrorInvalidInput, codes.InvalidArgument},
		{"DataRetrievalFailed", vehicleErrors.ErrorDataRetrievalFailed, codes.Internal},
		{"InternalServer", vehicleErrors.ErrorInternalServer, codes.Internal},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc, h := newMockAndHandler()
			svc.On("AddVehicle", mock.Anything, mock.Anything).Return("", c.err)
			_, err := h.AddVehicle(context.Background(), &vehiclepb.AddVehicleRequest{UserId: "u", Brand: "b", LicencePlate: "p"})
			assertGRPCCode(t, err, c.want)
		})
	}
}
