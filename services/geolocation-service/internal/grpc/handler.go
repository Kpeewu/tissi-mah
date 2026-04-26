package grpc

import (
	"context"
	"time"

	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/geolocation-service/internal/service/interfaces"
	geolocationpb "github.com/Kpeewu/tissi-mah/services/geolocation-service/proto/gen"
	"go.uber.org/zap"
)

// GeolocationHandler implémente l'interface gRPC GeolocationServiceServer.
// La logique métier est déléguée à serviceInterfaces.GeolocationService —
// le handler ne fait que mapper proto ↔ domain et journaliser.
type GeolocationHandler struct {
	geolocationpb.UnimplementedGeolocationServiceServer

	service serviceInterfaces.GeolocationService
	logger  *zap.Logger
}

func NewGeolocationHandler(service serviceInterfaces.GeolocationService, logger *zap.Logger) *GeolocationHandler {
	return &GeolocationHandler{
		service: service,
		logger:  logger,
	}
}

// =============================================================================
// ComputeRoute
// =============================================================================

func (h *GeolocationHandler) ComputeRoute(ctx context.Context, req *geolocationpb.ComputeRouteRequest) (*geolocationpb.ComputeRouteResponse, error) {
	input := serviceInterfaces.ComputeRouteInput{
		Waypoints:     toCoords(req.Waypoints),
		Profile:       req.Profile,
		DepartureTime: req.DepartureTime,
	}

	result, err := h.service.ComputeRoute(ctx, input)
	if err != nil {
		h.logger.Warn("ComputeRoute failed", zap.Error(err))
		return &geolocationpb.ComputeRouteResponse{
			ErrorMessage: err.Error(),
		}, nil
	}

	return &geolocationpb.ComputeRouteResponse{
		DistanceMeters:  result.DistanceMeters,
		DurationSeconds: result.DurationSeconds,
		PolylineEncoded: result.PolylineEncoded,
		Legs:            toProtoLegs(result.Legs),
	}, nil
}

// =============================================================================
// Geocode
// =============================================================================

func (h *GeolocationHandler) Geocode(ctx context.Context, req *geolocationpb.GeocodeRequest) (*geolocationpb.GeocodeResponse, error) {
	results, err := h.service.Geocode(ctx, serviceInterfaces.GeocodeInput{
		Query:         req.Query,
		CountryFilter: req.CountryFilter,
		Limit:         req.Limit,
	})
	if err != nil {
		h.logger.Warn("Geocode failed", zap.Error(err))
		return &geolocationpb.GeocodeResponse{ErrorMessage: err.Error()}, nil
	}

	return &geolocationpb.GeocodeResponse{
		Results: toProtoGeocodeResults(results),
	}, nil
}

// =============================================================================
// ReverseGeocode
// =============================================================================

func (h *GeolocationHandler) ReverseGeocode(ctx context.Context, req *geolocationpb.ReverseGeocodeRequest) (*geolocationpb.ReverseGeocodeResponse, error) {
	result, err := h.service.ReverseGeocode(ctx, serviceInterfaces.ReverseGeocodeInput{
		Lat: req.Lat,
		Lng: req.Lng,
	})
	if err != nil {
		h.logger.Warn("ReverseGeocode failed", zap.Error(err))
		return &geolocationpb.ReverseGeocodeResponse{ErrorMessage: err.Error()}, nil
	}

	return &geolocationpb.ReverseGeocodeResponse{
		Result: toProtoGeocodeResult(result),
	}, nil
}

// =============================================================================
// Health
// =============================================================================

func (h *GeolocationHandler) Health(_ context.Context, _ *geolocationpb.HealthRequest) (*geolocationpb.HealthResponse, error) {
	return &geolocationpb.HealthResponse{
		Status:    "SERVING",
		Version:   "1.0.0",
		Timestamp: time.Now().Unix(),
	}, nil
}

// =============================================================================
// Mappers
// =============================================================================

func toCoords(in []*geolocationpb.Coordinate) []serviceInterfaces.Coordinate {
	out := make([]serviceInterfaces.Coordinate, 0, len(in))
	for _, c := range in {
		if c == nil {
			continue
		}
		out = append(out, serviceInterfaces.Coordinate{Lat: c.Lat, Lng: c.Lng})
	}
	return out
}

func toProtoLegs(in []serviceInterfaces.RouteLeg) []*geolocationpb.RouteLeg {
	out := make([]*geolocationpb.RouteLeg, 0, len(in))
	for _, leg := range in {
		out = append(out, &geolocationpb.RouteLeg{
			DistanceMeters:       leg.DistanceMeters,
			DurationSeconds:      leg.DurationSeconds,
			MinutesFromDeparture: leg.MinutesFromDeparture,
			EstimatedArrivalTime: leg.EstimatedArrivalTime,
		})
	}
	return out
}

func toProtoGeocodeResults(in []*serviceInterfaces.GeocodeResult) []*geolocationpb.GeocodeResult {
	out := make([]*geolocationpb.GeocodeResult, 0, len(in))
	for _, r := range in {
		if r == nil {
			continue
		}
		out = append(out, toProtoGeocodeResult(r))
	}
	return out
}

func toProtoGeocodeResult(r *serviceInterfaces.GeocodeResult) *geolocationpb.GeocodeResult {
	if r == nil {
		return nil
	}
	return &geolocationpb.GeocodeResult{
		DisplayName: r.DisplayName,
		Lat:         r.Lat,
		Lng:         r.Lng,
		Country:     r.Country,
		City:        r.City,
		Type:        r.Type,
	}
}
