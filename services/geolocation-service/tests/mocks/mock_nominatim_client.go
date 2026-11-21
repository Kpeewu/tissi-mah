package mocks

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/geolocation-service/internal/service/interfaces"
	"github.com/stretchr/testify/mock"
)

// MockNominatimClient est un mock testify de nominatim.Client.
type MockNominatimClient struct {
	mock.Mock
}

func (m *MockNominatimClient) Search(ctx context.Context, query, countryFilter string, limit int32) ([]*interfaces.GeocodeResult, error) {
	args := m.Called(ctx, query, countryFilter, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*interfaces.GeocodeResult), args.Error(1)
}

func (m *MockNominatimClient) Reverse(ctx context.Context, lat, lng float64) (*interfaces.GeocodeResult, error) {
	args := m.Called(ctx, lat, lng)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*interfaces.GeocodeResult), args.Error(1)
}
