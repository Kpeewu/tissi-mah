// Package service contient l'implémentation du GeolocationService.
package service

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/geolocation-service/internal/service/interfaces"
	geoErrors "github.com/Kpeewu/tissi-mah/services/geolocation-service/pkg/errors"
	"go.uber.org/zap"
)

// geolocationServiceImpl orchestre OSRM, Nominatim et Redis cache.
// Les dépendances concrètes (clients OSRM, Nominatim, cache) seront injectées
// dans les phases suivantes (1.4 client OSRM, 1.5 cache, 1.6 wiring complet).
type geolocationServiceImpl struct {
	// osrmClient    osrm.Client     // Phase 1.4
	// nominatimClient nominatim.Client // Phase 1.4 / 4
	// cache cache.GeolocationCache  // Phase 1.5
	logger *zap.Logger
}

// NewGeolocationService construit l'implémentation par défaut.
// Les dépendances seront ajoutées au fur et à mesure des phases.
func NewGeolocationService(logger *zap.Logger) interfaces.GeolocationService {
	return &geolocationServiceImpl{logger: logger}
}

// ComputeRoute — squelette. Implémentation réelle en Phase 1.6.
func (s *geolocationServiceImpl) ComputeRoute(_ context.Context, _ interfaces.ComputeRouteInput) (*interfaces.ComputeRouteResult, error) {
	return nil, geoErrors.ErrorRoutingUnavailable
}

// Geocode — squelette. Implémentation réelle en Phase 4.
func (s *geolocationServiceImpl) Geocode(_ context.Context, _ interfaces.GeocodeInput) ([]*interfaces.GeocodeResult, error) {
	return nil, geoErrors.ErrorGeocodingUnavailable
}

// ReverseGeocode — squelette. Implémentation réelle en Phase 4.
func (s *geolocationServiceImpl) ReverseGeocode(_ context.Context, _ interfaces.ReverseGeocodeInput) (*interfaces.GeocodeResult, error) {
	return nil, geoErrors.ErrorGeocodingUnavailable
}
