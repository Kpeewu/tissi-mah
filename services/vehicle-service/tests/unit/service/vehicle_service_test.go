package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Kpeewu/tissi-mah/services/vehicle-service/internal/domain"
	"github.com/Kpeewu/tissi-mah/services/vehicle-service/internal/service"
	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/vehicle-service/internal/service/interfaces"
	vehicleErrors "github.com/Kpeewu/tissi-mah/services/vehicle-service/pkg/errors"
	"github.com/Kpeewu/tissi-mah/services/vehicle-service/tests/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// newService crée un service véhicule avec mocks injectés et cache nil (dégradation gracieuse).
func newService() (*mocks.MockVehicleRepositoryRead, *mocks.MockVehicleRepositoryWrite, *mocks.MockFileServiceClient, serviceInterfaces.VehicleService) {
	r := new(mocks.MockVehicleRepositoryRead)
	w := new(mocks.MockVehicleRepositoryWrite)
	f := new(mocks.MockFileServiceClient)
	t := new(mocks.MockTripsServiceClient)
	svc := service.NewVehicleService(r, w, f, t, nil, zap.NewNop())
	return r, w, f, svc
}

// newServiceWithTrips crée un service avec accès aux mocks trips et file.
func newServiceWithTrips() (*mocks.MockVehicleRepositoryRead, *mocks.MockFileServiceClient, *mocks.MockTripsServiceClient, serviceInterfaces.VehicleService) {
	r := new(mocks.MockVehicleRepositoryRead)
	w := new(mocks.MockVehicleRepositoryWrite)
	f := new(mocks.MockFileServiceClient)
	t := new(mocks.MockTripsServiceClient)
	svc := service.NewVehicleService(r, w, f, t, nil, zap.NewNop())
	return r, f, t, svc
}

func newDomainVehicle(userID string) *domain.Vehicle {
	return &domain.Vehicle{
		VehicleID:     "veh-1",
		UserID:        userID,
		Brand:         "Toyota",
		NumberOfSeats: 4,
		BrandModel:    "Corolla",
		Color:         "Black",
		LicencePlate:  "AA-123",
	}
}

// ========== AddVehicle ==========

func TestAddVehicle(t *testing.T) {
	t.Run("succès", func(t *testing.T) {
		r, w, _, svc := newService()
		r.On("ExistsByLicencePlate", mock.Anything, "AA-123").Return(false, nil)
		w.On("Create", mock.Anything, mock.AnythingOfType("*domain.Vehicle")).
			Return("veh-created", nil)

		id, err := svc.AddVehicle(context.Background(), serviceInterfaces.AddVehicleInput{
			UserID: "u1", Brand: "Toyota", LicencePlate: "AA-123", NumberOfSeats: 4,
		})
		require.NoError(t, err)
		assert.Equal(t, "veh-created", id)
	})

	t.Run("champs obligatoires manquants → ErrorInvalidInput", func(t *testing.T) {
		cases := []serviceInterfaces.AddVehicleInput{
			{Brand: "T", LicencePlate: "AA"},
			{UserID: "u", LicencePlate: "AA"},
			{UserID: "u", Brand: "T"},
		}
		for _, in := range cases {
			_, _, _, svc := newService()
			_, err := svc.AddVehicle(context.Background(), in)
			assert.ErrorIs(t, err, vehicleErrors.ErrorInvalidInput)
		}
	})

	t.Run("plaque déjà utilisée → LicencePlateConflict", func(t *testing.T) {
		r, _, _, svc := newService()
		r.On("ExistsByLicencePlate", mock.Anything, "AA-123").Return(true, nil)
		_, err := svc.AddVehicle(context.Background(), serviceInterfaces.AddVehicleInput{
			UserID: "u1", Brand: "T", LicencePlate: "AA-123",
		})
		assert.ErrorIs(t, err, vehicleErrors.ErrorLicencePlateConflict)
	})

	t.Run("erreur ExistsByLicencePlate propagée", func(t *testing.T) {
		r, _, _, svc := newService()
		r.On("ExistsByLicencePlate", mock.Anything, "AA-123").
			Return(false, vehicleErrors.ErrorInternalServer)
		_, err := svc.AddVehicle(context.Background(), serviceInterfaces.AddVehicleInput{
			UserID: "u1", Brand: "T", LicencePlate: "AA-123",
		})
		assert.ErrorIs(t, err, vehicleErrors.ErrorInternalServer)
	})

	t.Run("erreur Create propagée", func(t *testing.T) {
		r, w, _, svc := newService()
		r.On("ExistsByLicencePlate", mock.Anything, "AA-123").Return(false, nil)
		w.On("Create", mock.Anything, mock.Anything).
			Return("", vehicleErrors.ErrorInternalServer)
		_, err := svc.AddVehicle(context.Background(), serviceInterfaces.AddVehicleInput{
			UserID: "u1", Brand: "T", LicencePlate: "AA-123",
		})
		assert.ErrorIs(t, err, vehicleErrors.ErrorInternalServer)
	})
}

// ========== GetVehicleDetails ==========

func TestGetVehicleDetails(t *testing.T) {
	t.Run("succès - docs via file-service", func(t *testing.T) {
		r, _, f, svc := newService()
		v := newDomainVehicle("u1")
		r.On("GetByID", mock.Anything, v.VehicleID).Return(v, nil)
		f.On("GetVehicleDocuments", mock.Anything, v.VehicleID).
			Return(domain.VehicleDocuments{AssuranceURL: "a.url", VehicleRegistrationURL: "r.url"}, nil)
		f.On("GetCurrentUserDocument", mock.Anything, "u1", "driverLicence").
			Return("https://example.com/permis.jpg", "VALIDATED", nil)

		got, err := svc.GetVehicleDetails(context.Background(), "u1", v.VehicleID)
		require.NoError(t, err)
		assert.Equal(t, v.VehicleID, got.Vehicle.VehicleID)
		assert.Equal(t, "a.url", got.Documents.AssuranceURL)
		assert.Equal(t, "r.url", got.Documents.VehicleRegistrationURL)
		assert.Equal(t, "https://example.com/permis.jpg", got.Documents.DriverLicenceURL)
	})

	t.Run("vehicleID vide → InvalidInput", func(t *testing.T) {
		_, _, _, svc := newService()
		_, err := svc.GetVehicleDetails(context.Background(), "u1", "")
		assert.ErrorIs(t, err, vehicleErrors.ErrorInvalidInput)
	})

	t.Run("utilisateur non propriétaire → Unauthorized", func(t *testing.T) {
		r, _, _, svc := newService()
		v := newDomainVehicle("other")
		r.On("GetByID", mock.Anything, v.VehicleID).Return(v, nil)
		_, err := svc.GetVehicleDetails(context.Background(), "u1", v.VehicleID)
		assert.ErrorIs(t, err, vehicleErrors.ErrorUnauthorized)
	})

	t.Run("vehicle introuvable → propagation", func(t *testing.T) {
		r, _, _, svc := newService()
		r.On("GetByID", mock.Anything, "bad").Return(nil, vehicleErrors.ErrorVehicleNotFound)
		_, err := svc.GetVehicleDetails(context.Background(), "u1", "bad")
		assert.ErrorIs(t, err, vehicleErrors.ErrorVehicleNotFound)
	})

	t.Run("dégradation gracieuse - file-service down", func(t *testing.T) {
		r, _, f, svc := newService()
		v := newDomainVehicle("u1")
		r.On("GetByID", mock.Anything, v.VehicleID).Return(v, nil)
		f.On("GetVehicleDocuments", mock.Anything, v.VehicleID).
			Return(domain.VehicleDocuments{}, errors.New("file service down"))
		f.On("GetCurrentUserDocument", mock.Anything, "u1", "driverLicence").
			Return("", "MISSING", errors.New("file service down"))

		got, err := svc.GetVehicleDetails(context.Background(), "u1", v.VehicleID)
		require.NoError(t, err)
		assert.Empty(t, got.Documents.AssuranceURL)
		assert.Empty(t, got.Documents.VehicleRegistrationURL)
		assert.Empty(t, got.Documents.DriverLicenceURL)
	})
}

// ========== GetUserVehicles ==========

func TestGetUserVehicles(t *testing.T) {
	t.Run("succès — statuts et TripCount populés", func(t *testing.T) {
		r, f, trips, svc := newServiceWithTrips()
		previews := []*domain.VehiclePreview{
			{VehicleID: "v1", Brand: "Toyota", BrandModel: "Corolla", LicencePlate: "AA", IsVerified: true},
		}
		r.On("GetByUserID", mock.Anything, "u1").Return(previews, nil)
		f.On("GetCurrentUserDocument", mock.Anything, "u1", "driverLicence").Return("", "PENDING", nil)
		f.On("GetVehicleDocuments", mock.Anything, "v1").Return(domain.VehicleDocuments{
			AssuranceStatus:           "PENDING",
			VehicleRegistrationStatus: "VALIDATED",
		}, nil)
		trips.On("GetVehicleCompletedTripCount", mock.Anything, "v1").Return(5, nil)

		got, err := svc.GetUserVehicles(context.Background(), "u1")
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, "v1", got[0].VehicleID)
		assert.Equal(t, "PENDING", got[0].AssuranceStatus)
		assert.Equal(t, "VALIDATED", got[0].VehicleRegistrationStatus)
		assert.Equal(t, "PENDING", got[0].DriverLicenceStatus)
		assert.Equal(t, int32(5), got[0].TripCount)
	})

	t.Run("dégradation gracieuse — file-service indisponible → statuts MISSING", func(t *testing.T) {
		r, f, trips, svc := newServiceWithTrips()
		previews := []*domain.VehiclePreview{
			{VehicleID: "v1", Brand: "Toyota", BrandModel: "Corolla", LicencePlate: "AA", IsVerified: true},
		}
		r.On("GetByUserID", mock.Anything, "u1").Return(previews, nil)
		f.On("GetCurrentUserDocument", mock.Anything, "u1", "driverLicence").Return("", "MISSING", errors.New("unavailable"))
		f.On("GetVehicleDocuments", mock.Anything, "v1").Return(domain.VehicleDocuments{}, errors.New("unavailable"))
		trips.On("GetVehicleCompletedTripCount", mock.Anything, "v1").Return(0, nil)

		got, err := svc.GetUserVehicles(context.Background(), "u1")
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, "MISSING", got[0].AssuranceStatus)
		assert.Equal(t, "MISSING", got[0].VehicleRegistrationStatus)
		assert.Equal(t, "MISSING", got[0].DriverLicenceStatus)
	})

	t.Run("liste vide — pas d'appels inter-service", func(t *testing.T) {
		r, f, trips, svc := newServiceWithTrips()
		r.On("GetByUserID", mock.Anything, "u1").Return([]*domain.VehiclePreview{}, nil)

		got, err := svc.GetUserVehicles(context.Background(), "u1")
		require.NoError(t, err)
		assert.Empty(t, got)
		f.AssertNotCalled(t, "GetCurrentUserDocument")
		f.AssertNotCalled(t, "GetVehicleDocuments")
		trips.AssertNotCalled(t, "GetVehicleCompletedTripCount")
	})

	t.Run("userID vide → InvalidInput", func(t *testing.T) {
		_, _, _, svc := newService()
		_, err := svc.GetUserVehicles(context.Background(), "")
		assert.ErrorIs(t, err, vehicleErrors.ErrorInvalidInput)
	})

	t.Run("erreur propagée depuis repo", func(t *testing.T) {
		r, _, _, svc := newService()
		r.On("GetByUserID", mock.Anything, "u1").
			Return(nil, vehicleErrors.ErrorInternalServer)
		_, err := svc.GetUserVehicles(context.Background(), "u1")
		assert.ErrorIs(t, err, vehicleErrors.ErrorInternalServer)
	})
}

// ========== UpdateVehicle ==========

func TestUpdateVehicle(t *testing.T) {
	t.Run("succès - change Color seul", func(t *testing.T) {
		r, w, _, svc := newService()
		v := newDomainVehicle("u1")
		r.On("GetByID", mock.Anything, v.VehicleID).Return(v, nil)
		w.On("Update", mock.Anything, mock.MatchedBy(func(vv *domain.Vehicle) bool {
			return vv.Color == "Red" && vv.LicencePlate == "AA-123"
		})).Return(v, nil)

		err := svc.UpdateVehicle(context.Background(), serviceInterfaces.UpdateVehicleInput{
			VehicleID: v.VehicleID, UserID: "u1", Color: "Red",
		})
		require.NoError(t, err)
		w.AssertExpectations(t)
	})

	t.Run("succès - change LicencePlate (non conflit)", func(t *testing.T) {
		r, w, _, svc := newService()
		v := newDomainVehicle("u1")
		r.On("GetByID", mock.Anything, v.VehicleID).Return(v, nil)
		r.On("ExistsByLicencePlate", mock.Anything, "BB-999").Return(false, nil)
		w.On("Update", mock.Anything, mock.MatchedBy(func(vv *domain.Vehicle) bool {
			return vv.LicencePlate == "BB-999"
		})).Return(v, nil)

		err := svc.UpdateVehicle(context.Background(), serviceInterfaces.UpdateVehicleInput{
			VehicleID: v.VehicleID, UserID: "u1", LicencePlate: "BB-999",
		})
		require.NoError(t, err)
	})

	t.Run("LicencePlate identique → pas de check conflit", func(t *testing.T) {
		r, w, _, svc := newService()
		v := newDomainVehicle("u1")
		r.On("GetByID", mock.Anything, v.VehicleID).Return(v, nil)
		w.On("Update", mock.Anything, mock.Anything).Return(v, nil)

		err := svc.UpdateVehicle(context.Background(), serviceInterfaces.UpdateVehicleInput{
			VehicleID: v.VehicleID, UserID: "u1", LicencePlate: v.LicencePlate,
		})
		require.NoError(t, err)
		r.AssertNotCalled(t, "ExistsByLicencePlate", mock.Anything, mock.Anything)
	})

	t.Run("champs obligatoires manquants → InvalidInput", func(t *testing.T) {
		_, _, _, svc := newService()
		err := svc.UpdateVehicle(context.Background(), serviceInterfaces.UpdateVehicleInput{
			VehicleID: "", UserID: "u1",
		})
		assert.ErrorIs(t, err, vehicleErrors.ErrorInvalidInput)

		_, _, _, svc2 := newService()
		err = svc2.UpdateVehicle(context.Background(), serviceInterfaces.UpdateVehicleInput{
			VehicleID: "v", UserID: "",
		})
		assert.ErrorIs(t, err, vehicleErrors.ErrorInvalidInput)
	})

	t.Run("non propriétaire → Unauthorized", func(t *testing.T) {
		r, _, _, svc := newService()
		v := newDomainVehicle("other")
		r.On("GetByID", mock.Anything, v.VehicleID).Return(v, nil)
		err := svc.UpdateVehicle(context.Background(), serviceInterfaces.UpdateVehicleInput{
			VehicleID: v.VehicleID, UserID: "u1",
		})
		assert.ErrorIs(t, err, vehicleErrors.ErrorUnauthorized)
	})

	t.Run("plaque conflit → LicencePlateConflict", func(t *testing.T) {
		r, _, _, svc := newService()
		v := newDomainVehicle("u1")
		r.On("GetByID", mock.Anything, v.VehicleID).Return(v, nil)
		r.On("ExistsByLicencePlate", mock.Anything, "BB-999").Return(true, nil)
		err := svc.UpdateVehicle(context.Background(), serviceInterfaces.UpdateVehicleInput{
			VehicleID: v.VehicleID, UserID: "u1", LicencePlate: "BB-999",
		})
		assert.ErrorIs(t, err, vehicleErrors.ErrorLicencePlateConflict)
	})

	t.Run("erreur ExistsByLicencePlate propagée", func(t *testing.T) {
		r, _, _, svc := newService()
		v := newDomainVehicle("u1")
		r.On("GetByID", mock.Anything, v.VehicleID).Return(v, nil)
		r.On("ExistsByLicencePlate", mock.Anything, "BB-999").
			Return(false, vehicleErrors.ErrorInternalServer)
		err := svc.UpdateVehicle(context.Background(), serviceInterfaces.UpdateVehicleInput{
			VehicleID: v.VehicleID, UserID: "u1", LicencePlate: "BB-999",
		})
		assert.ErrorIs(t, err, vehicleErrors.ErrorInternalServer)
	})

	t.Run("GetByID erreur propagée", func(t *testing.T) {
		r, _, _, svc := newService()
		r.On("GetByID", mock.Anything, "v1").
			Return(nil, vehicleErrors.ErrorVehicleNotFound)
		err := svc.UpdateVehicle(context.Background(), serviceInterfaces.UpdateVehicleInput{
			VehicleID: "v1", UserID: "u1",
		})
		assert.ErrorIs(t, err, vehicleErrors.ErrorVehicleNotFound)
	})

	t.Run("Update erreur propagée", func(t *testing.T) {
		r, w, _, svc := newService()
		v := newDomainVehicle("u1")
		r.On("GetByID", mock.Anything, v.VehicleID).Return(v, nil)
		w.On("Update", mock.Anything, mock.Anything).
			Return(nil, vehicleErrors.ErrorInternalServer)
		err := svc.UpdateVehicle(context.Background(), serviceInterfaces.UpdateVehicleInput{
			VehicleID: v.VehicleID, UserID: "u1", Color: "Red",
		})
		assert.ErrorIs(t, err, vehicleErrors.ErrorInternalServer)
	})
}

// ========== DeleteVehicle ==========

func TestDeleteVehicle(t *testing.T) {
	t.Run("succès", func(t *testing.T) {
		r, w, _, svc := newService()
		v := newDomainVehicle("u1")
		r.On("GetByID", mock.Anything, v.VehicleID).Return(v, nil)
		w.On("Delete", mock.Anything, v.VehicleID).Return(nil)

		err := svc.DeleteVehicle(context.Background(), "u1", v.VehicleID)
		require.NoError(t, err)
		w.AssertExpectations(t)
	})

	t.Run("champs vides → InvalidInput", func(t *testing.T) {
		_, _, _, svc := newService()
		assert.ErrorIs(t, svc.DeleteVehicle(context.Background(), "u1", ""), vehicleErrors.ErrorInvalidInput)
		assert.ErrorIs(t, svc.DeleteVehicle(context.Background(), "", "v"), vehicleErrors.ErrorInvalidInput)
	})

	t.Run("non propriétaire → Unauthorized", func(t *testing.T) {
		r, _, _, svc := newService()
		v := newDomainVehicle("other")
		r.On("GetByID", mock.Anything, v.VehicleID).Return(v, nil)
		err := svc.DeleteVehicle(context.Background(), "u1", v.VehicleID)
		assert.ErrorIs(t, err, vehicleErrors.ErrorUnauthorized)
	})

	t.Run("GetByID erreur propagée", func(t *testing.T) {
		r, _, _, svc := newService()
		r.On("GetByID", mock.Anything, "v1").
			Return(nil, vehicleErrors.ErrorVehicleNotFound)
		err := svc.DeleteVehicle(context.Background(), "u1", "v1")
		assert.ErrorIs(t, err, vehicleErrors.ErrorVehicleNotFound)
	})

	t.Run("Delete erreur propagée", func(t *testing.T) {
		r, w, _, svc := newService()
		v := newDomainVehicle("u1")
		r.On("GetByID", mock.Anything, v.VehicleID).Return(v, nil)
		w.On("Delete", mock.Anything, v.VehicleID).
			Return(vehicleErrors.ErrorInternalServer)
		err := svc.DeleteVehicle(context.Background(), "u1", v.VehicleID)
		assert.ErrorIs(t, err, vehicleErrors.ErrorInternalServer)
	})
}

// ========== VerifyVehicle ==========

func TestVerifyVehicle(t *testing.T) {
	t.Run("succès", func(t *testing.T) {
		_, w, _, svc := newService()
		w.On("SetVerified", mock.Anything, "v1", true).Return(nil)
		err := svc.VerifyVehicle(context.Background(), "v1", true)
		require.NoError(t, err)
	})

	t.Run("vehicleID vide → InvalidInput", func(t *testing.T) {
		_, _, _, svc := newService()
		err := svc.VerifyVehicle(context.Background(), "", true)
		assert.ErrorIs(t, err, vehicleErrors.ErrorInvalidInput)
	})

	t.Run("SetVerified erreur propagée", func(t *testing.T) {
		_, w, _, svc := newService()
		w.On("SetVerified", mock.Anything, "v1", false).
			Return(vehicleErrors.ErrorVehicleNotFound)
		err := svc.VerifyVehicle(context.Background(), "v1", false)
		assert.ErrorIs(t, err, vehicleErrors.ErrorVehicleNotFound)
	})
}
