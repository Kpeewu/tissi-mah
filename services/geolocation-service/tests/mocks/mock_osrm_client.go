// Package mocks fournit des doubles pour les tests unitaires.
package mocks

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/geolocation-service/internal/client/osrm"
	"github.com/stretchr/testify/mock"
)

// MockOSRMClient est un mock testify de osrm.Client pour les tests unitaires
// du service (sans avoir besoin d'un httptest.Server).
type MockOSRMClient struct {
	mock.Mock
}

func (m *MockOSRMClient) GetRoute(ctx context.Context, waypoints []osrm.Coordinate, profile string) (*osrm.RouteResult, error) {
	args := m.Called(ctx, waypoints, profile)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*osrm.RouteResult), args.Error(1)
}
