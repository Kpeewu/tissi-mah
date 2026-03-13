package service

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/trips-service/internal/domain"
	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/trips-service/internal/service/interfaces"
	tripErrors "github.com/Kpeewu/tissi-mah/services/trips-service/pkg/errors"
	"go.uber.org/zap"
)

// GetTripsPreviews retourne la liste paginée des trajets du conducteur
// enrichie avec le nom du conducteur et les infos du véhicule.
// Utilise un cache-aside à 3 niveaux : previews DB, driver name, vehicle info.
func (s *tripServiceImpl) GetTripsPreviews(ctx context.Context, input *serviceInterfaces.GetTripsPreviewsInput) ([]*serviceInterfaces.TripPreviewResult, error) {
	if input.DriverID == "" {
		return nil, tripErrors.ErrorInvalidInput
	}

	// --- Niveau 1 : cache des previews DB bruts ---
	previews, err := s.getCachedOrFetchPreviews(ctx, input.DriverID, input.PageIndex)
	if err != nil {
		return nil, err
	}

	if len(previews) == 0 {
		return []*serviceInterfaces.TripPreviewResult{}, nil
	}

	// --- Niveau 2 : cache du nom du conducteur ---
	driverName := s.getCachedOrFetchDriverName(ctx, input.DriverID)

	// --- Niveau 3 : cache des infos véhicule ---
	type vehicleInfo struct{ brand, plate string }
	vehicleMap := make(map[string]vehicleInfo)
	for _, p := range previews {
		if _, ok := vehicleMap[p.VehicleID]; ok {
			continue
		}
		brand, plate := s.getCachedOrFetchVehicleInfo(ctx, input.DriverID, p.VehicleID)
		vehicleMap[p.VehicleID] = vehicleInfo{brand: brand, plate: plate}
	}

	// --- Assemblage du résultat enrichi ---
	results := make([]*serviceInterfaces.TripPreviewResult, 0, len(previews))
	for _, p := range previews {
		v := vehicleMap[p.VehicleID]
		results = append(results, &serviceInterfaces.TripPreviewResult{
			TripID:                p.TripID,
			DriverID:              p.DriverID,
			DriverName:            driverName,
			VehicleID:             p.VehicleID,
			VehicleBrand:          v.brand,
			VehiclePlate:          v.plate,
			DepartureDatetime:     p.DepartureDatetime,
			TotalSeats:            p.TotalSeats,
			AvailableSeats:        p.AvailableSeats,
			DepartureLocationName: p.DepartureLocationName,
			ArrivalLocationName:   p.ArrivalLocationName,
		})
	}

	return results, nil
}

// GetCompletedTripsPreviews retourne la liste paginée des trajets complétés du conducteur
// enrichie avec le nom du conducteur et les infos du véhicule.
func (s *tripServiceImpl) GetCompletedTripsPreviews(ctx context.Context, input *serviceInterfaces.GetTripsPreviewsInput) ([]*serviceInterfaces.CompletedTripPreviewResult, error) {
	if input.DriverID == "" {
		return nil, tripErrors.ErrorInvalidInput
	}

	// --- Niveau 1 : cache des previews complétés DB bruts ---
	previews, err := s.getCachedOrFetchCompletedPreviews(ctx, input.DriverID, input.PageIndex)
	if err != nil {
		return nil, err
	}

	if len(previews) == 0 {
		return []*serviceInterfaces.CompletedTripPreviewResult{}, nil
	}

	// --- Niveau 2 : cache du nom du conducteur ---
	driverName := s.getCachedOrFetchDriverName(ctx, input.DriverID)

	// --- Niveau 3 : cache des infos véhicule ---
	type vehicleInfo struct{ brand, plate string }
	vehicleMap := make(map[string]vehicleInfo)
	for _, p := range previews {
		if _, ok := vehicleMap[p.VehicleID]; ok {
			continue
		}
		brand, plate := s.getCachedOrFetchVehicleInfo(ctx, input.DriverID, p.VehicleID)
		vehicleMap[p.VehicleID] = vehicleInfo{brand: brand, plate: plate}
	}

	// --- Assemblage du résultat enrichi ---
	results := make([]*serviceInterfaces.CompletedTripPreviewResult, 0, len(previews))
	for _, p := range previews {
		v := vehicleMap[p.VehicleID]
		results = append(results, &serviceInterfaces.CompletedTripPreviewResult{
			TripID:                p.TripID,
			DriverID:              p.DriverID,
			DriverName:            driverName,
			VehicleID:             p.VehicleID,
			VehicleBrand:          v.brand,
			VehiclePlateNumber:    v.plate,
			DepartureDatetime:     p.DepartureDatetime,
			TotalSeats:            p.TotalSeats,
			AvailableSeats:        p.AvailableSeats,
			DepartureLocationName: p.DepartureLocationName,
			ArrivalLocationName:   p.ArrivalLocationName,
		})
	}

	return results, nil
}

// getCachedOrFetchCompletedPreviews tente le cache Redis, puis fallback sur la DB.
func (s *tripServiceImpl) getCachedOrFetchCompletedPreviews(ctx context.Context, driverID string, pageIndex int) ([]*domain.TripPreview, error) {
	// Essai cache
	if s.cache != nil {
		cached, err := s.cache.GetCompletedTripsPreviews(ctx, driverID, pageIndex)
		if err == nil && cached != nil {
			return cached, nil
		}
	}

	// Fallback DB
	previews, err := s.readRepo.GetDriverCompletedTripsPreviews(ctx, driverID, pageIndex)
	if err != nil {
		s.logger.Error("GetDriverCompletedTripsPreviews failed", zap.Error(err), zap.String("driverID", driverID))
		return nil, err
	}

	// Populate cache
	if s.cache != nil {
		_ = s.cache.SetCompletedTripsPreviews(ctx, driverID, pageIndex, previews)
	}

	return previews, nil
}

// getCachedOrFetchPreviews tente le cache Redis, puis fallback sur la DB.
func (s *tripServiceImpl) getCachedOrFetchPreviews(ctx context.Context, driverID string, pageIndex int) ([]*domain.TripPreview, error) {
	// Essai cache
	if s.cache != nil {
		cached, err := s.cache.GetTripsPreviews(ctx, driverID, pageIndex)
		if err == nil && cached != nil {
			return cached, nil
		}
	}

	// Fallback DB
	previews, err := s.readRepo.GetDriverTripsPreviews(ctx, driverID, pageIndex)
	if err != nil {
		s.logger.Error("GetDriverTripsPreviews failed", zap.Error(err), zap.String("driverID", driverID))
		return nil, err
	}

	// Populate cache (y compris pour les résultats vides — évite des requêtes DB répétées)
	if s.cache != nil {
		_ = s.cache.SetTripsPreviews(ctx, driverID, pageIndex, previews)
	}

	return previews, nil
}

// getCachedOrFetchDriverName tente le cache Redis, puis fallback sur user-service.
func (s *tripServiceImpl) getCachedOrFetchDriverName(ctx context.Context, driverID string) string {
	// Essai cache
	if s.cache != nil {
		name, found, err := s.cache.GetDriverName(ctx, driverID)
		if err == nil && found {
			return name
		}
	}

	// Fallback gRPC
	name, err := s.userClient.GetDriverName(ctx, driverID)
	if err != nil {
		s.logger.Warn("GetDriverName failed, using empty name", zap.Error(err), zap.String("driverID", driverID))
		return ""
	}

	// Populate cache
	if s.cache != nil {
		_ = s.cache.SetDriverName(ctx, driverID, name)
	}

	return name
}

// getCachedOrFetchVehicleInfo tente le cache Redis, puis fallback sur vehicle-service.
func (s *tripServiceImpl) getCachedOrFetchVehicleInfo(ctx context.Context, driverID, vehicleID string) (string, string) {
	// Essai cache
	if s.cache != nil {
		brand, plate, found, err := s.cache.GetVehicleInfo(ctx, vehicleID)
		if err == nil && found {
			return brand, plate
		}
	}

	// Fallback gRPC
	brand, plate, err := s.vehicleClient.GetVehicleInfo(ctx, driverID, vehicleID)
	if err != nil {
		s.logger.Warn("GetVehicleInfo failed, using empty values",
			zap.Error(err),
			zap.String("vehicleID", vehicleID),
		)
		return "", ""
	}

	// Populate cache
	if s.cache != nil {
		_ = s.cache.SetVehicleInfo(ctx, vehicleID, brand, plate)
	}

	return brand, plate
}
