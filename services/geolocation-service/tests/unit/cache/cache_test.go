package cache_test

import (
	"context"
	"testing"

	"github.com/Kpeewu/tissi-mah/services/geolocation-service/internal/cache"
	"github.com/Kpeewu/tissi-mah/services/geolocation-service/internal/client/osrm"
	"github.com/Kpeewu/tissi-mah/services/geolocation-service/internal/service/interfaces"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// TestCache_NilClient_NoOps — un cache sans client Redis doit rester safe :
// Get → miss, Set → no-op, jamais de panic.
func TestCache_NilClient_NoOps(t *testing.T) {
	c := cache.New(nil, zap.NewNop())
	ctx := context.Background()

	wp := []osrm.Coordinate{{Lat: 6, Lng: 1}, {Lat: 7, Lng: 2}}

	r, hit := c.GetRoute(ctx, wp, "driving")
	assert.Nil(t, r)
	assert.False(t, hit)

	// Set ne doit pas paniquer même si client est nil.
	c.SetRoute(ctx, wp, "driving", &osrm.RouteResult{DistanceMeters: 100})

	g, hit := c.GetGeocode(ctx, "Lomé", "tg", 5)
	assert.Nil(t, g)
	assert.False(t, hit)

	c.SetGeocode(ctx, "Lomé", "tg", 5, []*interfaces.GeocodeResult{
		{DisplayName: "Lomé, Togo", Lat: 6.13, Lng: 1.22},
	})

	rev, hit := c.GetReverseGeocode(ctx, 6.13, 1.22)
	assert.Nil(t, rev)
	assert.False(t, hit)

	c.SetReverseGeocode(ctx, 6.13, 1.22, &interfaces.GeocodeResult{DisplayName: "x"})
}

// TestCache_NilCachePointer_DoesNotPanic — utiliser cache=*nil sur un service
// est aussi safe (couvre la voie où le service.go reçoit un nil pointer).
func TestCache_NilCachePointer_DoesNotPanic(t *testing.T) {
	var c *cache.Cache // nil
	ctx := context.Background()

	r, hit := c.GetRoute(ctx, []osrm.Coordinate{{Lat: 0, Lng: 0}, {Lat: 1, Lng: 1}}, "driving")
	assert.Nil(t, r)
	assert.False(t, hit)

	c.SetRoute(ctx, []osrm.Coordinate{{Lat: 0, Lng: 0}, {Lat: 1, Lng: 1}}, "driving", &osrm.RouteResult{})
}
