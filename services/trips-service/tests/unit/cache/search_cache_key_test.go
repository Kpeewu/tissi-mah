package cache_test

import (
	"testing"

	"github.com/Kpeewu/tissi-mah/services/trips-service/internal/cache"
	repoInterfaces "github.com/Kpeewu/tissi-mah/services/trips-service/internal/repository/interfaces"
	"github.com/stretchr/testify/assert"
)

// TestBuildSearchCacheKey vérifie que chaque paramètre influençant le résultat
// entre bien dans la clé (sinon deux recherches différentes partageraient un cache).
func TestBuildSearchCacheKey(t *testing.T) {
	base := func() *repoInterfaces.SearchTripsParams {
		return &repoInterfaces.SearchTripsParams{
			DepartureLocationName: "Agbalkpédo",
			ArrivalLocationName:   "Sokodé",
			SortBy:                "relevance",
			MinSeats:              1,
		}
	}
	baseKey := cache.BuildSearchCacheKey(base())

	t.Run("mêmes params → même clé (casse ignorée)", func(t *testing.T) {
		p := base()
		p.DepartureLocationName = "AGBALKPÉDO"
		assert.Equal(t, baseKey, cache.BuildSearchCacheKey(p))
	})

	t.Run("chaque param produit une clé distincte", func(t *testing.T) {
		lng, lat := 1.185, 6.175
		mutations := map[string]func(*repoInterfaces.SearchTripsParams){
			"SortBy":        func(p *repoInterfaces.SearchTripsParams) { p.SortBy = "price" },
			"MaxPrice":      func(p *repoInterfaces.SearchTripsParams) { p.MaxPrice = 5000 },
			"MinSeats":      func(p *repoInterfaces.SearchTripsParams) { p.MinSeats = 3 },
			"AllowLuggages": func(p *repoInterfaces.SearchTripsParams) { p.AllowLuggages = true },
			"AllowPets":     func(p *repoInterfaces.SearchTripsParams) { p.AllowPets = true },
			"AllowFood":     func(p *repoInterfaces.SearchTripsParams) { p.AllowFood = true },
			"AllowSmoking":  func(p *repoInterfaces.SearchTripsParams) { p.AllowSmoking = true },
			"PageIndex":     func(p *repoInterfaces.SearchTripsParams) { p.PageIndex = 1 },
			"Coordonnées": func(p *repoInterfaces.SearchTripsParams) {
				p.PassengerLng, p.PassengerLat, p.DistanceRangeMeters = &lng, &lat, 5000
			},
		}

		seen := map[string]string{"base": baseKey}
		for name, mutate := range mutations {
			p := base()
			mutate(p)
			key := cache.BuildSearchCacheKey(p)
			for other, otherKey := range seen {
				assert.NotEqual(t, otherKey, key, "clé identique entre %s et %s", name, other)
			}
			seen[name] = key
		}
	})
}
