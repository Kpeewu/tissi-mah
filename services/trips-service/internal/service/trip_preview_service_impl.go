package service

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/trips-service/internal/cache"
	"github.com/Kpeewu/tissi-mah/services/trips-service/internal/domain"
	repoInterfaces "github.com/Kpeewu/tissi-mah/services/trips-service/internal/repository/interfaces"
	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/trips-service/internal/service/interfaces"
	tripErrors "github.com/Kpeewu/tissi-mah/services/trips-service/pkg/errors"
	"go.uber.org/zap"
)

// GetTripsPreviews retourne la liste paginée des trajets du conducteur
// enrichie avec le nom du conducteur et les infos du véhicule.
// Utilise un cache-aside à 3 niveaux : previews DB, driver name, vehicle info.
func (s *tripServiceImpl) GetTripsPreviews(ctx context.Context, input *serviceInterfaces.GetTripsPreviewsInput) ([]*serviceInterfaces.TripPreviewResult, error) {
	driverUserID, err := s.resolveDriverUserID(ctx, input.DriverID)
	if err != nil {
		return nil, err
	}

	// --- Niveau 1 : cache des previews DB bruts ---
	previews, err := s.getCachedOrFetchPreviews(ctx, driverUserID, input.PageIndex)
	if err != nil {
		return nil, err
	}

	if len(previews) == 0 {
		return []*serviceInterfaces.TripPreviewResult{}, nil
	}

	// --- Niveau 2 : cache du nom du conducteur ---
	driverName := s.getCachedOrFetchDriverName(ctx, driverUserID)

	// --- Niveau 3 : cache des infos véhicule ---
	type vehicleInfo struct{ brand, plate string }
	vehicleMap := make(map[string]vehicleInfo)
	for _, p := range previews {
		if _, ok := vehicleMap[p.VehicleID]; ok {
			continue
		}
		brand, plate := s.getCachedOrFetchVehicleInfo(ctx, driverUserID, p.VehicleID)
		vehicleMap[p.VehicleID] = vehicleInfo{brand: brand, plate: plate}
	}

	// --- Assemblage du résultat enrichi ---
	results := make([]*serviceInterfaces.TripPreviewResult, 0, len(previews))
	for _, p := range previews {
		v := vehicleMap[p.VehicleID]

		// Overlay available_seats depuis le cache Redis (temps réel post-réconciliation)
		availableSeats := p.AvailableSeats
		if s.cache != nil {
			if seats, found, err := s.cache.GetSeatCounter(ctx, p.TripID); found && err == nil {
				availableSeats = int16(seats)
			} else if err != nil {
				s.logger.Warn("GetTripsPreviews — seat cache read failed, using DB value",
					zap.String("tripID", p.TripID),
					zap.Error(err),
				)
			}
		}

		results = append(results, &serviceInterfaces.TripPreviewResult{
			TripID:                p.TripID,
			DriverID:              p.DriverID,
			DriverName:            driverName,
			VehicleID:             p.VehicleID,
			VehicleBrand:          v.brand,
			VehiclePlate:          v.plate,
			DepartureDatetime:     p.DepartureDatetime,
			TotalSeats:            p.TotalSeats,
			AvailableSeats:        availableSeats,
			DepartureLocationName: p.DepartureLocationName,
			ArrivalLocationName:   p.ArrivalLocationName,
		})
	}

	return results, nil
}

// GetCompletedTripsPreviews retourne la liste paginée des trajets complétés du conducteur
// enrichie avec le nom du conducteur et les infos du véhicule.
func (s *tripServiceImpl) GetCompletedTripsPreviews(ctx context.Context, input *serviceInterfaces.GetTripsPreviewsInput) ([]*serviceInterfaces.CompletedTripPreviewResult, error) {
	driverUserID, err := s.resolveDriverUserID(ctx, input.DriverID)
	if err != nil {
		return nil, err
	}

	// --- Niveau 1 : cache des previews complétés DB bruts ---
	previews, err := s.getCachedOrFetchCompletedPreviews(ctx, driverUserID, input.PageIndex)
	if err != nil {
		return nil, err
	}

	if len(previews) == 0 {
		return []*serviceInterfaces.CompletedTripPreviewResult{}, nil
	}

	// --- Niveau 2 : cache du nom du conducteur ---
	driverName := s.getCachedOrFetchDriverName(ctx, driverUserID)

	// --- Niveau 3 : cache des infos véhicule ---
	type vehicleInfo struct{ brand, plate string }
	vehicleMap := make(map[string]vehicleInfo)
	for _, p := range previews {
		if _, ok := vehicleMap[p.VehicleID]; ok {
			continue
		}
		brand, plate := s.getCachedOrFetchVehicleInfo(ctx, driverUserID, p.VehicleID)
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

// validSearchSortBy liste les valeurs SortBy acceptées pour la recherche passager.
var validSearchSortBy = map[string]bool{
	"relevance":      true,
	"departure_time": true,
	"price":          true,
}

// GetScheduledTripsPreviews recherche les trajets/segments disponibles pour un passager.
// Utilise le cache Redis pour les résultats de recherche (TTL 60s) et l'enrichissement standard.
func (s *tripServiceImpl) GetScheduledTripsPreviews(ctx context.Context, input *serviceInterfaces.GetScheduledTripsPreviewsInput) (*serviceInterfaces.ScheduledTripsPreviewsResult, error) {
	if input.DepartureLocationName == "" || input.ArrivalLocationName == "" {
		return nil, tripErrors.ErrorInvalidInput
	}

	// Tri : défaut pertinence, valeur inconnue rejetée
	sortBy := input.SortBy
	if sortBy == "" {
		sortBy = "relevance"
	}
	if !validSearchSortBy[sortBy] {
		return nil, tripErrors.ErrorInvalidInput
	}

	if input.MaxPrice < 0 || input.MinSeats < 0 {
		return nil, tripErrors.ErrorInvalidInput
	}

	pageSize := 10

	// Construire les paramètres de recherche
	distanceMeters := 5000
	if input.DistanceRange != nil && *input.DistanceRange > 0 {
		distanceMeters = *input.DistanceRange * 1000
	}

	minSeats := input.MinSeats
	if minSeats <= 0 {
		minSeats = 1
	}

	params := &repoInterfaces.SearchTripsParams{
		PassengerLng:          input.PassengerPositionLng,
		PassengerLat:          input.PassengerPositionLat,
		DistanceRangeMeters:   distanceMeters,
		DepartureLocationName: input.DepartureLocationName,
		ArrivalLocationName:   input.ArrivalLocationName,
		TripStartDate:         input.TripStartDate,
		TripStartHour:         input.TripStartHour,
		TripArrivalHour:       input.TripArrivalHour,
		SortBy:                sortBy,
		MaxPrice:              input.MaxPrice,
		MinSeats:              minSeats,
		AllowLuggages:         input.AllowLuggages,
		AllowPets:             input.AllowPets,
		AllowFood:             input.AllowFood,
		AllowSmoking:          input.AllowSmoking,
		PageIndex:             input.PageIndex,
		PageSize:              pageSize,
	}

	// Générer la clé de cache normalisée
	cacheKey := cache.BuildSearchCacheKey(params)

	// Essayer le cache Redis
	var previews []*domain.TripPreview
	var totalCount int

	if s.cache != nil {
		cached, count, err := s.cache.GetSearchResults(ctx, cacheKey)
		if err == nil && cached != nil {
			previews = cached
			totalCount = count
		}
	}

	// Cache miss : requête DB
	if previews == nil {
		result, err := s.readRepo.SearchScheduledTripSegments(ctx, params)
		if err != nil {
			s.logger.Error("SearchScheduledTripSegments failed", zap.Error(err))
			return nil, err
		}
		previews = result.Previews
		totalCount = result.TotalCount

		// Stocker en cache
		if s.cache != nil {
			_ = s.cache.SetSearchResults(ctx, cacheKey, previews, totalCount)
		}
	}

	if len(previews) == 0 {
		return &serviceInterfaces.ScheduledTripsPreviewsResult{
			Previews:   []*serviceInterfaces.TripPreviewResult{},
			NextIndex:  -1,
			TotalCount: totalCount,
		}, nil
	}

	// Enrichissement multi-driver : collecter les driverIDs et vehicleIDs uniques
	type vehicleInfo struct{ brand, plate string }
	type driverEnrichment struct {
		name, profileImageURL string
		ratingAverage         float64
	}
	driverMap := make(map[string]driverEnrichment)
	vehicleMap := make(map[string]vehicleInfo)

	for _, p := range previews {
		if _, ok := driverMap[p.DriverID]; !ok {
			name, photo := s.getCachedOrFetchDriverInfo(ctx, p.DriverID)
			rating := s.getCachedOrFetchDriverRating(ctx, p.DriverID)
			driverMap[p.DriverID] = driverEnrichment{name: name, profileImageURL: photo, ratingAverage: rating}
		}
		if _, ok := vehicleMap[p.VehicleID]; !ok {
			brand, plate := s.getCachedOrFetchVehicleInfo(ctx, p.DriverID, p.VehicleID)
			vehicleMap[p.VehicleID] = vehicleInfo{brand: brand, plate: plate}
		}
	}

	// Assemblage du résultat enrichi
	results := make([]*serviceInterfaces.TripPreviewResult, 0, len(previews))
	for _, p := range previews {
		v := vehicleMap[p.VehicleID]
		d := driverMap[p.DriverID]

		results = append(results, &serviceInterfaces.TripPreviewResult{
			TripID:                 p.TripID,
			DriverID:               p.DriverID,
			DriverName:             d.name,
			VehicleID:              p.VehicleID,
			VehicleBrand:           v.brand,
			VehiclePlate:           v.plate,
			DepartureDatetime:      p.DepartureDatetime,
			TotalSeats:             p.TotalSeats,
			AvailableSeats:         p.AvailableSeats,
			DepartureLocationName:  p.DepartureLocationName,
			ArrivalLocationName:    p.ArrivalLocationName,
			DepartureWaypointID:    p.DepartureWaypointID,
			ArrivalWaypointID:      p.ArrivalWaypointID,
			SegmentPrice:           p.SegmentPrice,
			SegmentDurationMinutes: p.SegmentDurationMinutes,
			DriverProfileImageURL:  d.profileImageURL,
			DriverRatingAverage:    d.ratingAverage,
			RelevanceScore:         p.RelevanceScore,
		})
	}

	// Calculer NextIndex
	nextIndex := -1
	if input.PageIndex*pageSize+len(results) < totalCount {
		nextIndex = input.PageIndex + 1
	}

	return &serviceInterfaces.ScheduledTripsPreviewsResult{
		Previews:   results,
		NextIndex:  nextIndex,
		TotalCount: totalCount,
	}, nil
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
	brand, plate, _, _, err := s.vehicleClient.GetVehicleInfo(ctx, driverID, vehicleID)
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

// getCachedOrFetchDriverInfo tente le cache Redis, puis fallback sur user-service.
// Retourne (name, profileImageURL).
func (s *tripServiceImpl) getCachedOrFetchDriverInfo(ctx context.Context, driverID string) (string, string) {
	// Essai cache
	if s.cache != nil {
		name, photo, found, err := s.cache.GetDriverInfo(ctx, driverID)
		if err == nil && found {
			return name, photo
		}
	}

	// Fallback gRPC
	name, photo, err := s.userClient.GetDriverInfo(ctx, driverID)
	if err != nil {
		s.logger.Warn("GetDriverInfo failed, using empty values", zap.Error(err), zap.String("driverID", driverID))
		return "", ""
	}

	// Populate cache
	if s.cache != nil {
		_ = s.cache.SetDriverInfo(ctx, driverID, name, photo)
	}

	return name, photo
}

// getCachedOrFetchDriverRating tente le cache Redis, puis fallback sur rating-service.
func (s *tripServiceImpl) getCachedOrFetchDriverRating(ctx context.Context, driverID string) float64 {
	// Essai cache
	if s.cache != nil {
		rating, found, err := s.cache.GetDriverRating(ctx, driverID)
		if err == nil && found {
			return rating
		}
	}

	// Fallback gRPC
	if s.ratingClient == nil {
		return 0
	}
	rating, err := s.ratingClient.GetDriverRatingAverage(ctx, driverID)
	if err != nil {
		s.logger.Warn("GetDriverRatingAverage failed, using 0", zap.Error(err), zap.String("driverID", driverID))
		return 0
	}

	// Populate cache
	if s.cache != nil {
		_ = s.cache.SetDriverRating(ctx, driverID, rating)
	}

	return rating
}
