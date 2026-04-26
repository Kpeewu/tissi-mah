package client_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Kpeewu/tissi-mah/services/geolocation-service/internal/client/nominatim"
	geoErrors "github.com/Kpeewu/tissi-mah/services/geolocation-service/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

const (
	searchOk = `[
      {"lat":"6.13","lon":"1.22","display_name":"Marché de Lomé, Togo","type":"market","class":"amenity",
       "address":{"city":"Lomé","country":"Togo","country_code":"tg"}}
    ]`

	searchEmpty = `[]`

	reverseOk = `{"lat":"6.13","lon":"1.22","display_name":"Lomé, Togo","type":"city","class":"place",
                  "address":{"city":"Lomé","country":"Togo","country_code":"tg"}}`

	reverseError = `{"error":"Unable to geocode"}`
)

func newNominatim(t *testing.T, baseURL string) nominatim.Client {
	t.Helper()
	cfg := nominatim.DefaultConfig(baseURL)
	cfg.RequestTimeout = 2 * time.Second
	return nominatim.New(cfg, zap.NewNop())
}

func TestNominatim_NewWithEmptyURL_ReturnsNil(t *testing.T) {
	c := nominatim.New(nominatim.DefaultConfig(""), zap.NewNop())
	assert.Nil(t, c, "URL vide → client nil (graceful degradation)")
}

func TestNominatim_Search_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/search")
		assert.Contains(t, r.URL.RawQuery, "format=json")
		assert.Contains(t, r.URL.RawQuery, "addressdetails=1")
		assert.Contains(t, r.URL.RawQuery, "countrycodes=tg")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(searchOk))
	}))
	defer srv.Close()

	c := newNominatim(t, srv.URL)
	results, err := c.Search(context.Background(), "Marché de Lomé", "tg", 5)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, 6.13, results[0].Lat)
	assert.Equal(t, 1.22, results[0].Lng)
	assert.Equal(t, "Marché de Lomé, Togo", results[0].DisplayName)
	assert.Equal(t, "Lomé", results[0].City)
	assert.Equal(t, "tg", results[0].Country)
}

func TestNominatim_Search_EmptyQuery_Rejected(t *testing.T) {
	c := newNominatim(t, "http://unused")
	_, err := c.Search(context.Background(), "", "tg", 5)
	require.Error(t, err)
	assert.ErrorIs(t, err, geoErrors.ErrorEmptyQuery)
}

func TestNominatim_Search_NoResults_ReturnsAddressNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(searchEmpty))
	}))
	defer srv.Close()

	c := newNominatim(t, srv.URL)
	_, err := c.Search(context.Background(), "Atlantis", "tg", 5)
	require.Error(t, err)
	assert.ErrorIs(t, err, geoErrors.ErrorAddressNotFound)
}

func TestNominatim_Search_5xx_ReturnsUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := newNominatim(t, srv.URL)
	_, err := c.Search(context.Background(), "Lomé", "tg", 5)
	require.Error(t, err)
	assert.ErrorIs(t, err, geoErrors.ErrorGeocodingUnavailable)
}

func TestNominatim_Reverse_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/reverse")
		assert.Contains(t, r.URL.RawQuery, "lat=6.13")
		assert.Contains(t, r.URL.RawQuery, "lon=1.22")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(reverseOk))
	}))
	defer srv.Close()

	c := newNominatim(t, srv.URL)
	r, err := c.Reverse(context.Background(), 6.13, 1.22)
	require.NoError(t, err)
	require.NotNil(t, r)
	assert.Equal(t, "Lomé, Togo", r.DisplayName)
	assert.Equal(t, "Lomé", r.City)
	assert.Equal(t, "tg", r.Country)
}

func TestNominatim_Reverse_Error_ReturnsAddressNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(reverseError))
	}))
	defer srv.Close()

	c := newNominatim(t, srv.URL)
	_, err := c.Reverse(context.Background(), 0, 0)
	require.Error(t, err)
	assert.ErrorIs(t, err, geoErrors.ErrorAddressNotFound)
}

func TestNominatim_Reverse_OutOfRange(t *testing.T) {
	c := newNominatim(t, "http://unused")
	_, err := c.Reverse(context.Background(), 95, 0)
	require.Error(t, err)
	assert.ErrorIs(t, err, geoErrors.ErrorInvalidWaypoints)
}
