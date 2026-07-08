package service

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/vehicle-service/internal/cache"
	clientInterfaces "github.com/Kpeewu/tissi-mah/services/vehicle-service/internal/client"
	"github.com/Kpeewu/tissi-mah/services/vehicle-service/internal/domain"
	repoInterfaces "github.com/Kpeewu/tissi-mah/services/vehicle-service/internal/repository/interfaces"
	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/vehicle-service/internal/service/interfaces"
	vehicleErrors "github.com/Kpeewu/tissi-mah/services/vehicle-service/pkg/errors"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type vehicleServiceImpl struct {
	readRepo    repoInterfaces.VehicleRepositoryRead
	writeRepo   repoInterfaces.VehicleRepositoryWrite
	fileClient  clientInterfaces.FileServiceClient
	tripsClient clientInterfaces.TripsServiceClient
	cache       *cache.VehicleCache
	logger      *zap.Logger
}

// NewVehicleService crée une nouvelle instance du service véhicule.
func NewVehicleService(
	readRepo repoInterfaces.VehicleRepositoryRead,
	writeRepo repoInterfaces.VehicleRepositoryWrite,
	fileClient clientInterfaces.FileServiceClient,
	tripsClient clientInterfaces.TripsServiceClient,
	vehicleCache *cache.VehicleCache,
	logger *zap.Logger,
) serviceInterfaces.VehicleService {
	return &vehicleServiceImpl{
		readRepo:    readRepo,
		writeRepo:   writeRepo,
		fileClient:  fileClient,
		tripsClient: tripsClient,
		cache:       vehicleCache,
		logger:      logger,
	}
}

// AddVehicle crée un nouveau véhicule après vérification de la plaque d'immatriculation.
func (s *vehicleServiceImpl) AddVehicle(ctx context.Context, input serviceInterfaces.AddVehicleInput) (string, error) {
	if input.UserID == "" || input.Brand == "" || input.LicencePlate == "" {
		s.logger.Error("champs obligatoires manquants pour la création du véhicule")
		return "", vehicleErrors.ErrorInvalidInput
	}

	s.logger.Debug("add vehicle",
		zap.String("userID", input.UserID),
		zap.String("licencePlate", input.LicencePlate),
	)

	exists, err := s.readRepo.ExistsByLicencePlate(ctx, input.LicencePlate)
	if err != nil {
		return "", err
	}
	if exists {
		s.logger.Warn("licence plate already in use", zap.String("licencePlate", input.LicencePlate))
		return "", vehicleErrors.ErrorLicencePlateConflict
	}

	vehicle := &domain.Vehicle{
		VehicleID:     uuid.New().String(),
		UserID:        input.UserID,
		Brand:         input.Brand,
		NumberOfSeats: input.NumberOfSeats,
		BrandModel:    input.BrandModel,
		Color:         input.Color,
		LicencePlate:  input.LicencePlate,
	}

	vehicleID, err := s.writeRepo.Create(ctx, vehicle)
	if err != nil {
		s.logger.Error("add vehicle failed", zap.Error(err))
		return "", err
	}

	s.logger.Info("vehicle added", zap.String("vehicleID", vehicleID))
	return vehicleID, nil
}

// GetVehicleDetails récupère les détails complets d'un véhicule, incluant ses
// documents depuis file-service. Seules les infos véhicule sont cachées (Redis) ;
// les documents sont toujours récupérés en temps réel pour éviter qu'un upload
// récent (via file-service) ne soit masqué par un cache périmé.
func (s *vehicleServiceImpl) GetVehicleDetails(ctx context.Context, userID string, vehicleID string) (*domain.VehicleDetails, error) {
	if vehicleID == "" {
		return nil, vehicleErrors.ErrorInvalidInput
	}

	// Infos véhicule : depuis le cache si présent, sinon DB (+ mise en cache).
	var vehicle *domain.Vehicle
	if s.cache != nil {
		if cached, err := s.cache.GetVehicle(ctx, vehicleID); err == nil {
			vehicle = cached
		}
	}
	if vehicle == nil {
		v, err := s.readRepo.GetByID(ctx, vehicleID)
		if err != nil {
			return nil, err
		}
		vehicle = v
		if s.cache != nil {
			if err := s.cache.SetVehicle(ctx, vehicleID, vehicle); err != nil {
				s.logger.Warn("failed to cache vehicle", zap.Error(err), zap.String("vehicleID", vehicleID))
			}
		}
	}

	// Vérification de la propriété du véhicule
	if vehicle.UserID != userID {
		s.logger.Warn("unauthorized vehicle details access",
			zap.String("userID", userID),
			zap.String("vehicleOwner", vehicle.UserID),
		)
		return nil, vehicleErrors.ErrorUnauthorized
	}

	// Documents véhicule : toujours récupérés en temps réel depuis file-service.
	// En cas d'erreur, on retourne le véhicule avec des documents vides (tolérance aux pannes).
	docs, err := s.fileClient.GetVehicleDocuments(ctx, vehicleID)
	if err != nil {
		s.logger.Warn("failed to fetch vehicle documents from file-service, returning empty documents",
			zap.Error(err),
			zap.String("vehicleID", vehicleID),
		)
		docs = domain.VehicleDocuments{}
	}

	// URL du permis du conducteur (document utilisateur).
	driverLicenceURL, _, _ := s.fileClient.GetCurrentUserDocument(ctx, userID, "driverLicence")
	docs.DriverLicenceURL = driverLicenceURL

	return &domain.VehicleDetails{
		Vehicle:   vehicle,
		Documents: docs,
	}, nil
}

// GetVehicleInfo récupère les infos essentielles d'un véhicule (sans documents ni
// contrôle de propriété). Destiné aux appels inter-service (ex. file-service qui
// enrichit les documents véhicule avec marque/modèle/plaque).
func (s *vehicleServiceImpl) GetVehicleInfo(ctx context.Context, vehicleID string) (*domain.Vehicle, error) {
	if vehicleID == "" {
		return nil, vehicleErrors.ErrorInvalidInput
	}
	return s.readRepo.GetByID(ctx, vehicleID)
}

// GetUserVehicles récupère les aperçus de tous les véhicules d'un utilisateur,
// enrichis avec les statuts des documents et le nombre de trajets par véhicule.
func (s *vehicleServiceImpl) GetUserVehicles(ctx context.Context, userID string) ([]*domain.VehiclePreview, error) {
	if userID == "" {
		return nil, vehicleErrors.ErrorInvalidInput
	}

	previews, err := s.readRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if len(previews) == 0 {
		return previews, nil
	}

	// Statut du permis : appel unique par userID (le permis est au niveau du conducteur).
	// Dégradation gracieuse si file-service est indisponible.
	_, driverLicenceStatus, _ := s.fileClient.GetCurrentUserDocument(ctx, userID, "driverLicence")
	if driverLicenceStatus == "" {
		driverLicenceStatus = "MISSING"
	}

	for _, p := range previews {
		// Statuts des docs véhicule (assurance + carte grise).
		docs, err := s.fileClient.GetVehicleDocuments(ctx, p.VehicleID)
		if err != nil {
			s.logger.Warn("failed to fetch vehicle documents for preview",
				zap.Error(err),
				zap.String("vehicleID", p.VehicleID),
			)
		}
		if docs.AssuranceStatus == "" {
			docs.AssuranceStatus = "MISSING"
		}
		if docs.VehicleRegistrationStatus == "" {
			docs.VehicleRegistrationStatus = "MISSING"
		}
		p.AssuranceStatus = docs.AssuranceStatus
		p.VehicleRegistrationStatus = docs.VehicleRegistrationStatus
		p.DriverLicenceStatus = driverLicenceStatus

		// Nombre de trajets complétés avec ce véhicule.
		count, err := s.tripsClient.GetVehicleCompletedTripCount(ctx, p.VehicleID)
		if err != nil {
			s.logger.Warn("failed to fetch trip count for vehicle",
				zap.Error(err),
				zap.String("vehicleID", p.VehicleID),
			)
		}
		p.TripCount = count
	}

	return previews, nil
}

// UpdateVehicle met à jour les champs modifiables d'un véhicule après vérification de la propriété.
func (s *vehicleServiceImpl) UpdateVehicle(ctx context.Context, input serviceInterfaces.UpdateVehicleInput) error {
	if input.VehicleID == "" || input.UserID == "" {
		return vehicleErrors.ErrorInvalidInput
	}

	s.logger.Debug("update vehicle",
		zap.String("vehicleID", input.VehicleID),
		zap.String("userID", input.UserID),
	)

	existing, err := s.readRepo.GetByID(ctx, input.VehicleID)
	if err != nil {
		return err
	}

	if existing.UserID != input.UserID {
		s.logger.Warn("unauthorized vehicle update attempt",
			zap.String("userID", input.UserID),
			zap.String("vehicleOwner", existing.UserID),
		)
		return vehicleErrors.ErrorUnauthorized
	}

	if input.Color != "" {
		existing.Color = input.Color
	}
	if input.LicencePlate != "" && input.LicencePlate != existing.LicencePlate {
		plateExists, err := s.readRepo.ExistsByLicencePlate(ctx, input.LicencePlate)
		if err != nil {
			return err
		}
		if plateExists {
			s.logger.Warn("licence plate already in use", zap.String("licencePlate", input.LicencePlate))
			return vehicleErrors.ErrorLicencePlateConflict
		}
		existing.LicencePlate = input.LicencePlate
	}

	_, err = s.writeRepo.Update(ctx, existing)
	if err != nil {
		s.logger.Error("update vehicle failed", zap.Error(err))
		return err
	}

	// Invalidation du cache après modification
	if s.cache != nil {
		s.cache.InvalidateVehicle(ctx, input.VehicleID)
	}

	s.logger.Info("vehicle updated", zap.String("vehicleID", input.VehicleID))
	return nil
}

// DeleteVehicle supprime un véhicule après vérification de la propriété.
func (s *vehicleServiceImpl) DeleteVehicle(ctx context.Context, userID string, vehicleID string) error {
	if vehicleID == "" || userID == "" {
		return vehicleErrors.ErrorInvalidInput
	}

	s.logger.Debug("delete vehicle", zap.String("vehicleID", vehicleID), zap.String("userID", userID))

	existing, err := s.readRepo.GetByID(ctx, vehicleID)
	if err != nil {
		return err
	}

	if existing.UserID != userID {
		s.logger.Warn("unauthorized vehicle delete attempt",
			zap.String("userID", userID),
			zap.String("vehicleOwner", existing.UserID),
		)
		return vehicleErrors.ErrorUnauthorized
	}

	if err := s.writeRepo.Delete(ctx, vehicleID); err != nil {
		return err
	}

	// Invalidation du cache après suppression
	if s.cache != nil {
		s.cache.InvalidateVehicle(ctx, vehicleID)
	}

	return nil
}

// VerifyVehicle met à jour le statut de vérification d'un véhicule.
func (s *vehicleServiceImpl) VerifyVehicle(ctx context.Context, vehicleID string, isVerified bool) error {
	if vehicleID == "" {
		return vehicleErrors.ErrorInvalidInput
	}

	if err := s.writeRepo.SetVerified(ctx, vehicleID, isVerified); err != nil {
		return err
	}

	// Invalidation du cache après changement de statut de vérification
	if s.cache != nil {
		s.cache.InvalidateVehicle(ctx, vehicleID)
	}

	return nil
}
