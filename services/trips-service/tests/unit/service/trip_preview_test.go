package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Kpeewu/tissi-mah/services/trips-service/internal/domain"
	repoInterfaces "github.com/Kpeewu/tissi-mah/services/trips-service/internal/repository/interfaces"
	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/trips-service/internal/service/interfaces"
	tripErrors "github.com/Kpeewu/tissi-mah/services/trips-service/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func samplePreview(tripID, driverID, vehicleID string) *domain.TripPreview {
	return &domain.TripPreview{
		TripID:                tripID,
		DriverID:              driverID,
		VehicleID:             vehicleID,
		DepartureDatetime:     time.Now().UTC().Add(2 * time.Hour),
		TotalSeats:            4,
		AvailableSeats:        3,
		DepartureLocationName: "Lomé",
		ArrivalLocationName:   "Kpalimé",
	}
}

func TestGetTripsPreviews_Fallbacks(t *testing.T) {
	t.Run("repo échoue → propage", func(t *testing.T) {
		readRepo, _, _, _, svc := newTestService()
		ctx := context.Background()
		readRepo.On("GetDriverTripsPreviews", ctx, "driver-1", 0).
			Return([]*domain.TripPreview(nil), errors.New("db down"))

		_, err := svc.GetTripsPreviews(ctx, &serviceInterfaces.GetTripsPreviewsInput{DriverID: "driver-1"})
		require.Error(t, err)
	})

	t.Run("vehicle client échoue → champs vides", func(t *testing.T) {
		readRepo, _, userClient, vehicleClient, svc := newTestService()
		ctx := context.Background()
		previews := []*domain.TripPreview{samplePreview("trip-1", "driver-1", "vehicle-bad")}
		readRepo.On("GetDriverTripsPreviews", ctx, "driver-1", 0).Return(previews, nil)
		userClient.On("GetDriverName", ctx, "driver-1").Return("Jean", nil)
		vehicleClient.On("GetVehicleInfo", ctx, "driver-1", "vehicle-bad").
			Return("", "", 0, errors.New("vehicle svc down"))

		res, err := svc.GetTripsPreviews(ctx, &serviceInterfaces.GetTripsPreviewsInput{DriverID: "driver-1"})
		require.NoError(t, err)
		assert.Equal(t, "", res[0].VehicleBrand)
		assert.Equal(t, "Jean", res[0].DriverName)
	})

	t.Run("user client échoue → driver name vide", func(t *testing.T) {
		readRepo, _, userClient, vehicleClient, svc := newTestService()
		ctx := context.Background()
		previews := []*domain.TripPreview{samplePreview("trip-1", "driver-1", "vehicle-1")}
		readRepo.On("GetDriverTripsPreviews", ctx, "driver-1", 0).Return(previews, nil)
		userClient.On("GetDriverName", ctx, "driver-1").Return("", errors.New("user svc down"))
		vehicleClient.On("GetVehicleInfo", ctx, "driver-1", "vehicle-1").Return("Toyota", "AA", 4, nil)

		res, err := svc.GetTripsPreviews(ctx, &serviceInterfaces.GetTripsPreviewsInput{DriverID: "driver-1"})
		require.NoError(t, err)
		assert.Equal(t, "", res[0].DriverName)
	})

	t.Run("driverID vide → Unauthorized", func(t *testing.T) {
		_, _, _, _, svc := newTestService()
		_, err := svc.GetTripsPreviews(context.Background(), &serviceInterfaces.GetTripsPreviewsInput{DriverID: ""})
		assert.ErrorIs(t, err, tripErrors.ErrorUnauthorized)
	})

	t.Run("deux previews même véhicule → une seule fetch vehicle", func(t *testing.T) {
		readRepo, _, userClient, vehicleClient, svc := newTestService()
		ctx := context.Background()
		previews := []*domain.TripPreview{
			samplePreview("trip-1", "driver-1", "vehicle-1"),
			samplePreview("trip-2", "driver-1", "vehicle-1"),
		}
		readRepo.On("GetDriverTripsPreviews", ctx, "driver-1", 0).Return(previews, nil)
		userClient.On("GetDriverName", ctx, "driver-1").Return("Jean", nil)
		vehicleClient.On("GetVehicleInfo", ctx, "driver-1", "vehicle-1").Return("Toyota", "AA", 4, nil).Once()

		res, err := svc.GetTripsPreviews(ctx, &serviceInterfaces.GetTripsPreviewsInput{DriverID: "driver-1"})
		require.NoError(t, err)
		assert.Len(t, res, 2)
		vehicleClient.AssertExpectations(t)
	})
}

func TestGetCompletedTripsPreviews(t *testing.T) {
	t.Run("succès", func(t *testing.T) {
		readRepo, _, userClient, vehicleClient, svc := newTestService()
		ctx := context.Background()
		previews := []*domain.TripPreview{samplePreview("trip-done", "driver-1", "vehicle-1")}
		readRepo.On("GetDriverCompletedTripsPreviews", ctx, "driver-1", 0).Return(previews, nil)
		userClient.On("GetDriverName", ctx, "driver-1").Return("Jean", nil)
		vehicleClient.On("GetVehicleInfo", ctx, "driver-1", "vehicle-1").Return("Toyota", "AA-1234", 4, nil)

		res, err := svc.GetCompletedTripsPreviews(ctx, &serviceInterfaces.GetTripsPreviewsInput{DriverID: "driver-1"})
		require.NoError(t, err)
		assert.Len(t, res, 1)
		assert.Equal(t, "AA-1234", res[0].VehiclePlateNumber)
	})

	t.Run("succès - vide", func(t *testing.T) {
		readRepo, _, _, _, svc := newTestService()
		ctx := context.Background()
		readRepo.On("GetDriverCompletedTripsPreviews", ctx, "driver-1", 0).Return([]*domain.TripPreview{}, nil)

		res, err := svc.GetCompletedTripsPreviews(ctx, &serviceInterfaces.GetTripsPreviewsInput{DriverID: "driver-1"})
		require.NoError(t, err)
		assert.Empty(t, res)
	})

	t.Run("erreur - driverID vide", func(t *testing.T) {
		_, _, _, _, svc := newTestService()
		_, err := svc.GetCompletedTripsPreviews(context.Background(), &serviceInterfaces.GetTripsPreviewsInput{DriverID: ""})
		assert.ErrorIs(t, err, tripErrors.ErrorUnauthorized)
	})
}

func TestGetScheduledTripsPreviews(t *testing.T) {
	t.Run("succès - enrichissement multi-driver", func(t *testing.T) {
		readRepo, _, userClient, vehicleClient, svc := newTestService()
		ctx := context.Background()

		previews := []*domain.TripPreview{
			samplePreview("trip-1", "driver-A", "veh-A"),
			samplePreview("trip-2", "driver-B", "veh-B"),
		}
		readRepo.On("SearchScheduledTripSegments", ctx, mock.AnythingOfType("*interfaces.SearchTripsParams")).
			Return(&repoInterfaces.SearchTripsResult{Previews: previews, TotalCount: 2}, nil)

		userClient.On("GetDriverInfo", ctx, "driver-A").Return("Alice", "urlA", nil)
		userClient.On("GetDriverInfo", ctx, "driver-B").Return("Bob", "urlB", nil)
		vehicleClient.On("GetVehicleInfo", ctx, "driver-A", "veh-A").Return("Toyota", "AA-1", 4, nil)
		vehicleClient.On("GetVehicleInfo", ctx, "driver-B", "veh-B").Return("Honda", "BB-2", 4, nil)

		res, err := svc.GetScheduledTripsPreviews(ctx, &serviceInterfaces.GetScheduledTripsPreviewsInput{
			DepartureLocationName: "Lomé",
			ArrivalLocationName:   "Kpalimé",
		})
		require.NoError(t, err)
		assert.Len(t, res.Previews, 2)
		assert.Equal(t, 2, res.TotalCount)
		assert.Equal(t, -1, res.NextIndex)
	})

	t.Run("succès - pagination NextIndex", func(t *testing.T) {
		readRepo, _, userClient, vehicleClient, svc := newTestService()
		ctx := context.Background()
		previews := []*domain.TripPreview{samplePreview("t1", "d1", "v1")}
		readRepo.On("SearchScheduledTripSegments", ctx, mock.AnythingOfType("*interfaces.SearchTripsParams")).
			Return(&repoInterfaces.SearchTripsResult{Previews: previews, TotalCount: 100}, nil)
		userClient.On("GetDriverInfo", ctx, "d1").Return("D1", "", nil)
		vehicleClient.On("GetVehicleInfo", ctx, "d1", "v1").Return("T", "P", 4, nil)

		res, err := svc.GetScheduledTripsPreviews(ctx, &serviceInterfaces.GetScheduledTripsPreviewsInput{
			DepartureLocationName: "A", ArrivalLocationName: "B", PageIndex: 0,
		})
		require.NoError(t, err)
		assert.Equal(t, 1, res.NextIndex)
	})

	t.Run("succès - aucun résultat", func(t *testing.T) {
		readRepo, _, _, _, svc := newTestService()
		ctx := context.Background()
		readRepo.On("SearchScheduledTripSegments", ctx, mock.AnythingOfType("*interfaces.SearchTripsParams")).
			Return(&repoInterfaces.SearchTripsResult{Previews: nil, TotalCount: 0}, nil)

		res, err := svc.GetScheduledTripsPreviews(ctx, &serviceInterfaces.GetScheduledTripsPreviewsInput{
			DepartureLocationName: "A", ArrivalLocationName: "B",
		})
		require.NoError(t, err)
		assert.Empty(t, res.Previews)
		assert.Equal(t, -1, res.NextIndex)
	})

	t.Run("erreur - DepartureLocationName vide → InvalidInput", func(t *testing.T) {
		_, _, _, _, svc := newTestService()
		_, err := svc.GetScheduledTripsPreviews(context.Background(), &serviceInterfaces.GetScheduledTripsPreviewsInput{
			DepartureLocationName: "", ArrivalLocationName: "B",
		})
		assert.ErrorIs(t, err, tripErrors.ErrorInvalidInput)
	})

	t.Run("erreur - ArrivalLocationName vide → InvalidInput", func(t *testing.T) {
		_, _, _, _, svc := newTestService()
		_, err := svc.GetScheduledTripsPreviews(context.Background(), &serviceInterfaces.GetScheduledTripsPreviewsInput{
			DepartureLocationName: "A", ArrivalLocationName: "",
		})
		assert.ErrorIs(t, err, tripErrors.ErrorInvalidInput)
	})

	t.Run("erreur - repo échoue → propage", func(t *testing.T) {
		readRepo, _, _, _, svc := newTestService()
		ctx := context.Background()
		readRepo.On("SearchScheduledTripSegments", ctx, mock.AnythingOfType("*interfaces.SearchTripsParams")).
			Return((*repoInterfaces.SearchTripsResult)(nil), errors.New("db error"))

		_, err := svc.GetScheduledTripsPreviews(ctx, &serviceInterfaces.GetScheduledTripsPreviewsInput{
			DepartureLocationName: "A", ArrivalLocationName: "B",
		})
		require.Error(t, err)
	})

	t.Run("erreur - SortBy invalide → InvalidInput", func(t *testing.T) {
		_, _, _, _, svc := newTestService()
		_, err := svc.GetScheduledTripsPreviews(context.Background(), &serviceInterfaces.GetScheduledTripsPreviewsInput{
			DepartureLocationName: "A", ArrivalLocationName: "B", SortBy: "banana",
		})
		assert.ErrorIs(t, err, tripErrors.ErrorInvalidInput)
	})

	t.Run("erreur - MaxPrice négatif → InvalidInput", func(t *testing.T) {
		_, _, _, _, svc := newTestService()
		_, err := svc.GetScheduledTripsPreviews(context.Background(), &serviceInterfaces.GetScheduledTripsPreviewsInput{
			DepartureLocationName: "A", ArrivalLocationName: "B", MaxPrice: -1,
		})
		assert.ErrorIs(t, err, tripErrors.ErrorInvalidInput)
	})

	t.Run("défauts - SortBy vide → relevance, MinSeats 0 → 1 transmis au repo", func(t *testing.T) {
		readRepo, _, _, _, svc := newTestService()
		ctx := context.Background()
		readRepo.On("SearchScheduledTripSegments", ctx, mock.MatchedBy(func(p *repoInterfaces.SearchTripsParams) bool {
			return p.SortBy == "relevance" && p.MinSeats == 1
		})).Return(&repoInterfaces.SearchTripsResult{Previews: nil, TotalCount: 0}, nil)

		_, err := svc.GetScheduledTripsPreviews(ctx, &serviceInterfaces.GetScheduledTripsPreviewsInput{
			DepartureLocationName: "A", ArrivalLocationName: "B",
		})
		require.NoError(t, err)
		readRepo.AssertExpectations(t)
	})
}
