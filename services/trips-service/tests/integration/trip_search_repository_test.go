package integration

import (
	"context"
	"testing"
	"time"

	"github.com/Kpeewu/tissi-mah/services/trips-service/fixtures"
	"github.com/Kpeewu/tissi-mah/services/trips-service/internal/domain"
	i "github.com/Kpeewu/tissi-mah/services/trips-service/internal/repository/interfaces"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// TestSearchScheduledTripSegments — moteur de recherche passager
// =============================================================================

// Coordonnées de référence (Togo)
const (
	agbalkpedoLng = 1.1850
	agbalkpedoLat = 6.1750
	adidogomeLng  = 1.1670
	adidogomeLat  = 6.1660
	sokodeLng     = 1.1394
	sokodeLat     = 8.9833
	karaLng       = 1.1900
	karaLat       = 9.5500
)

// searchTripSpec décrit un trajet 2 waypoints (départ → arrivée Sokodé) à seeder.
type searchTripSpec struct {
	depName  string
	depCity  string
	depLng   float64
	depLat   float64
	arrPrice int // prix du segment, défaut 500
	tripOpts []fixtures.TripOption
}

// seedSearchTrip insère un trajet scheduled avec départ paramétrable et arrivée à Sokodé.
func seedSearchTrip(t *testing.T, ctx context.Context, spec searchTripSpec) *domain.Trip {
	t.Helper()
	writeRepo := newTestWriteRepository()

	trip := fixtures.NewTestTrip(spec.tripOpts...)
	dep := fixtures.NewTestWaypoint(trip.TripID,
		fixtures.WithWaypointType(domain.WaypointTypeDeparture),
		fixtures.WithSequencerOrder(1),
		fixtures.WithLocationName(spec.depName),
		fixtures.WithCity(spec.depCity),
		fixtures.WithWaypointCoordinates(spec.depLng, spec.depLat),
	)
	arrPrice := spec.arrPrice
	if arrPrice == 0 {
		arrPrice = 500
	}
	arr := fixtures.NewTestWaypoint(trip.TripID,
		fixtures.WithWaypointType(domain.WaypointTypeArrival),
		fixtures.WithSequencerOrder(3),
		fixtures.WithLocationName("Gare routière de Sokodé"),
		fixtures.WithCity("Sokodé"),
		fixtures.WithWaypointCoordinates(sokodeLng, sokodeLat),
		fixtures.WithPriceFromPrevious(arrPrice),
	)
	_, err := writeRepo.Create(ctx, trip, []*domain.Waypoint{dep, arr})
	require.NoError(t, err)
	return trip
}

func TestSearchScheduledTripSegments_Fuzzy(t *testing.T) {
	ctx := context.Background()

	t.Run("fuzzy - Abalpedo → Sokode trouve « Gare d'Agbalkpédo » → « Sokodé »", func(t *testing.T) {
		cleanupTripsTable(t, ctx)
		readRepo := newTestReadRepository()
		trip := seedSearchTrip(t, ctx, searchTripSpec{
			depName: "Gare d'Agbalkpédo, Lomé", depCity: "Agbalkpédo",
			depLng: agbalkpedoLng, depLat: agbalkpedoLat,
		})

		res, err := readRepo.SearchScheduledTripSegments(ctx, &i.SearchTripsParams{
			DepartureLocationName: "Abalpedo",
			ArrivalLocationName:   "Sokode",
		})

		require.NoError(t, err)
		require.Len(t, res.Previews, 1)
		assert.Equal(t, trip.TripID, res.Previews[0].TripID)
		assert.Greater(t, res.Previews[0].RelevanceScore, 0.0)
		assert.Equal(t, 1, res.TotalCount)
	})

	t.Run("fuzzy - matche sur city quand location_name est sans rapport", func(t *testing.T) {
		cleanupTripsTable(t, ctx)
		readRepo := newTestReadRepository()
		trip := seedSearchTrip(t, ctx, searchTripSpec{
			depName: "Station Shell", depCity: "Agbalkpédo",
			depLng: agbalkpedoLng, depLat: agbalkpedoLat,
		})

		res, err := readRepo.SearchScheduledTripSegments(ctx, &i.SearchTripsParams{
			DepartureLocationName: "Agbalkpedo",
			ArrivalLocationName:   "Sokode",
		})

		require.NoError(t, err)
		require.Len(t, res.Previews, 1)
		assert.Equal(t, trip.TripID, res.Previews[0].TripID)
	})

	t.Run("aucun match nom → vide", func(t *testing.T) {
		cleanupTripsTable(t, ctx)
		readRepo := newTestReadRepository()
		seedSearchTrip(t, ctx, searchTripSpec{
			depName: "Gare d'Agbalkpédo, Lomé", depCity: "Agbalkpédo",
			depLng: agbalkpedoLng, depLat: agbalkpedoLat,
		})

		res, err := readRepo.SearchScheduledTripSegments(ctx, &i.SearchTripsParams{
			DepartureLocationName: "Ouagadougou",
			ArrivalLocationName:   "Sokode",
		})

		require.NoError(t, err)
		assert.Empty(t, res.Previews)
		assert.Equal(t, 0, res.TotalCount)
	})
}

func TestSearchScheduledTripSegments_DepartureZone(t *testing.T) {
	ctx := context.Background()

	// Le nom recherché « Agbalkpedo » ne matche ni « Marché d'Adidogomé » ni la city « Lomé »
	newZoneParams := func(lng, lat float64, radiusMeters int) *i.SearchTripsParams {
		return &i.SearchTripsParams{
			DepartureLocationName: "Agbalkpedo",
			ArrivalLocationName:   "Sokode",
			PassengerLng:          &lng,
			PassengerLat:          &lat,
			DistanceRangeMeters:   radiusMeters,
		}
	}

	t.Run("départ dans le rayon sans match nom → trouvé (sémantique OU)", func(t *testing.T) {
		cleanupTripsTable(t, ctx)
		readRepo := newTestReadRepository()
		trip := seedSearchTrip(t, ctx, searchTripSpec{
			depName: "Marché d'Adidogomé", depCity: "Lomé",
			depLng: adidogomeLng, depLat: adidogomeLat,
		})

		res, err := readRepo.SearchScheduledTripSegments(ctx, newZoneParams(adidogomeLng, adidogomeLat, 10000))

		require.NoError(t, err)
		require.Len(t, res.Previews, 1)
		assert.Equal(t, trip.TripID, res.Previews[0].TripID)
	})

	t.Run("départ hors rayon et sans match nom → absent", func(t *testing.T) {
		cleanupTripsTable(t, ctx)
		readRepo := newTestReadRepository()
		seedSearchTrip(t, ctx, searchTripSpec{
			depName: "Marché d'Adidogomé", depCity: "Lomé",
			depLng: adidogomeLng, depLat: adidogomeLat,
		})

		// Zone de départ à Kara, à ~370 km d'Adidogomé
		res, err := readRepo.SearchScheduledTripSegments(ctx, newZoneParams(karaLng, karaLat, 5000))

		require.NoError(t, err)
		assert.Empty(t, res.Previews)
	})

	t.Run("match nom hors rayon → trouvé quand même (OU, pas ET)", func(t *testing.T) {
		cleanupTripsTable(t, ctx)
		readRepo := newTestReadRepository()
		trip := seedSearchTrip(t, ctx, searchTripSpec{
			depName: "Gare d'Agbalkpédo, Lomé", depCity: "Agbalkpédo",
			depLng: agbalkpedoLng, depLat: agbalkpedoLat,
		})

		// Zone à Kara : le départ est hors rayon mais le nom matche
		res, err := readRepo.SearchScheduledTripSegments(ctx, newZoneParams(karaLng, karaLat, 5000))

		require.NoError(t, err)
		require.Len(t, res.Previews, 1)
		assert.Equal(t, trip.TripID, res.Previews[0].TripID)
	})
}

func TestSearchScheduledTripSegments_RelevanceAndSort(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()

	// tripName matche par nom (score ≈ 1), tripGeo uniquement par rayon (score nom ≈ 0)
	seedBoth := func(t *testing.T) (tripName, tripGeo *domain.Trip) {
		t.Helper()
		cleanupTripsTable(t, ctx)
		tripName = seedSearchTrip(t, ctx, searchTripSpec{
			depName: "Gare d'Agbalkpédo, Lomé", depCity: "Agbalkpédo",
			depLng: agbalkpedoLng, depLat: agbalkpedoLat,
			arrPrice: 3000,
			tripOpts: []fixtures.TripOption{fixtures.WithDepartureDatetime(now.Add(2 * time.Hour))},
		})
		tripGeo = seedSearchTrip(t, ctx, searchTripSpec{
			depName: "Marché d'Adidogomé", depCity: "Lomé",
			depLng: adidogomeLng, depLat: adidogomeLat,
			arrPrice: 1000,
			tripOpts: []fixtures.TripOption{fixtures.WithDepartureDatetime(now.Add(1 * time.Hour))},
		})
		return tripName, tripGeo
	}

	newParams := func(sortBy string) *i.SearchTripsParams {
		lng, lat := adidogomeLng, adidogomeLat
		return &i.SearchTripsParams{
			DepartureLocationName: "Agbalkpedo",
			ArrivalLocationName:   "Sokode",
			PassengerLng:          &lng,
			PassengerLat:          &lat,
			DistanceRangeMeters:   10000,
			SortBy:                sortBy,
		}
	}

	t.Run("tri pertinence (défaut) - match nom devant match rayon-seulement", func(t *testing.T) {
		tripName, tripGeo := seedBoth(t)
		readRepo := newTestReadRepository()

		res, err := readRepo.SearchScheduledTripSegments(ctx, newParams(""))

		require.NoError(t, err)
		require.Len(t, res.Previews, 2)
		assert.Equal(t, tripName.TripID, res.Previews[0].TripID)
		assert.Equal(t, tripGeo.TripID, res.Previews[1].TripID)
		assert.Greater(t, res.Previews[0].RelevanceScore, res.Previews[1].RelevanceScore)
	})

	t.Run("tri departure_time - le plus tôt en premier", func(t *testing.T) {
		tripName, tripGeo := seedBoth(t)
		readRepo := newTestReadRepository()

		res, err := readRepo.SearchScheduledTripSegments(ctx, newParams("departure_time"))

		require.NoError(t, err)
		require.Len(t, res.Previews, 2)
		assert.Equal(t, tripGeo.TripID, res.Previews[0].TripID)
		assert.Equal(t, tripName.TripID, res.Previews[1].TripID)
	})

	t.Run("tri price - le moins cher en premier", func(t *testing.T) {
		tripName, tripGeo := seedBoth(t)
		readRepo := newTestReadRepository()

		res, err := readRepo.SearchScheduledTripSegments(ctx, newParams("price"))

		require.NoError(t, err)
		require.Len(t, res.Previews, 2)
		assert.Equal(t, tripGeo.TripID, res.Previews[0].TripID)
		assert.Equal(t, 1000, res.Previews[0].SegmentPrice)
		assert.Equal(t, tripName.TripID, res.Previews[1].TripID)
		assert.Equal(t, 3000, res.Previews[1].SegmentPrice)
	})
}

func TestSearchScheduledTripSegments_Filters(t *testing.T) {
	ctx := context.Background()

	baseParams := func() *i.SearchTripsParams {
		return &i.SearchTripsParams{
			DepartureLocationName: "Agbalkpedo",
			ArrivalLocationName:   "Sokode",
		}
	}

	t.Run("MaxPrice - exclut les segments trop chers, TotalCount cohérent", func(t *testing.T) {
		cleanupTripsTable(t, ctx)
		readRepo := newTestReadRepository()
		seedSearchTrip(t, ctx, searchTripSpec{
			depName: "Agbalkpédo", depCity: "Agbalkpédo",
			depLng: agbalkpedoLng, depLat: agbalkpedoLat, arrPrice: 3000,
		})
		cheap := seedSearchTrip(t, ctx, searchTripSpec{
			depName: "Agbalkpédo", depCity: "Agbalkpédo",
			depLng: agbalkpedoLng, depLat: agbalkpedoLat, arrPrice: 1000,
		})

		params := baseParams()
		params.MaxPrice = 1500
		res, err := readRepo.SearchScheduledTripSegments(ctx, params)

		require.NoError(t, err)
		require.Len(t, res.Previews, 1)
		assert.Equal(t, cheap.TripID, res.Previews[0].TripID)
		assert.Equal(t, 1, res.TotalCount)
	})

	t.Run("MinSeats - sièges disponibles du segment (booked_seats)", func(t *testing.T) {
		cleanupTripsTable(t, ctx)
		readRepo := newTestReadRepository()
		trip := seedSearchTrip(t, ctx, searchTripSpec{
			depName: "Agbalkpédo", depCity: "Agbalkpédo",
			depLng: agbalkpedoLng, depLat: agbalkpedoLat,
		})

		// 3 places réservées sur le leg de départ → 1 place restante sur le segment
		_, err := testPool.Exec(ctx,
			"UPDATE trips_waypoints SET booked_seats = 3 WHERE trip_id = $1 AND sequencer_order = 1",
			trip.TripID)
		require.NoError(t, err)

		params := baseParams()
		params.MinSeats = 2
		res, err := readRepo.SearchScheduledTripSegments(ctx, params)
		require.NoError(t, err)
		assert.Empty(t, res.Previews)

		params.MinSeats = 1
		res, err = readRepo.SearchScheduledTripSegments(ctx, params)
		require.NoError(t, err)
		require.Len(t, res.Previews, 1)
		assert.Equal(t, int16(1), res.Previews[0].AvailableSeats)
	})

	t.Run("options - AllowPets ne retourne que les trajets qui l'autorisent", func(t *testing.T) {
		cleanupTripsTable(t, ctx)
		readRepo := newTestReadRepository()
		seedSearchTrip(t, ctx, searchTripSpec{
			depName: "Agbalkpédo", depCity: "Agbalkpédo",
			depLng: agbalkpedoLng, depLat: agbalkpedoLat,
		})
		withPets := seedSearchTrip(t, ctx, searchTripSpec{
			depName: "Agbalkpédo", depCity: "Agbalkpédo",
			depLng: agbalkpedoLng, depLat: agbalkpedoLat,
			tripOpts: []fixtures.TripOption{fixtures.WithAllowOptions(false, true, false, false)},
		})

		params := baseParams()
		params.AllowPets = true
		res, err := readRepo.SearchScheduledTripSegments(ctx, params)

		require.NoError(t, err)
		require.Len(t, res.Previews, 1)
		assert.Equal(t, withPets.TripID, res.Previews[0].TripID)
	})
}
