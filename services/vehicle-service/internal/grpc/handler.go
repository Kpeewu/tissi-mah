package grpc

import (
	"context"
	"errors"
	"time"

	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/vehicle-service/internal/service/interfaces"
	vehicleErrors "github.com/Kpeewu/tissi-mah/services/vehicle-service/pkg/errors"
	vehiclepb "github.com/Kpeewu/tissi-mah/services/vehicle-service/proto/gen"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const serviceVersion = "1.0.0"

// VehicleHandler implémente vehiclepb.VehicleServiceServer.
// Il traduit les requêtes proto en appels de service et mappe les erreurs domaine
// vers les codes gRPC appropriés.
type VehicleHandler struct {
	vehiclepb.UnimplementedVehicleServiceServer
	service serviceInterfaces.VehicleService
	logger  *zap.Logger
}

// NewVehicleHandler crée un nouveau handler gRPC pour vehicle-service.
func NewVehicleHandler(service serviceInterfaces.VehicleService, logger *zap.Logger) *VehicleHandler {
	return &VehicleHandler{service: service, logger: logger}
}

// AddVehicle crée un nouveau véhicule.
func (h *VehicleHandler) AddVehicle(ctx context.Context, req *vehiclepb.AddVehicleRequest) (*vehiclepb.AddVehicleResponse, error) {
	h.logger.Debug("handler: AddVehicle called",
		zap.String("userID", req.UserId),
		zap.String("licencePlate", req.LicencePlate),
	)

	vehicleID, err := h.service.AddVehicle(ctx, serviceInterfaces.AddVehicleInput{
		UserID:        req.UserId,
		Brand:         req.Brand,
		NumberOfSeats: int16(req.NumberOfSeats),
		BrandModel:    req.BrandModel,
		Color:         req.Color,
		LicencePlate:  req.LicencePlate,
	})
	if err != nil {
		h.logger.Error("handler: AddVehicle failed", zap.Error(err))
		return nil, toGRPCError(err)
	}

	h.logger.Info("handler: AddVehicle success", zap.String("vehicleID", vehicleID))
	return &vehiclepb.AddVehicleResponse{VehicleId: vehicleID}, nil
}

// UpdateVehicle met à jour un véhicule existant.
func (h *VehicleHandler) UpdateVehicle(ctx context.Context, req *vehiclepb.UpdateVehicleRequest) (*vehiclepb.UpdateVehicleResponse, error) {
	h.logger.Debug("handler: UpdateVehicle called", zap.String("vehicleID", req.VehicleId))

	err := h.service.UpdateVehicle(ctx, serviceInterfaces.UpdateVehicleInput{
		VehicleID:    req.VehicleId,
		UserID:       req.UserId,
		Color:        req.Color,
		LicencePlate: req.LicencePlate,
	})
	if err != nil {
		h.logger.Error("handler: UpdateVehicle failed", zap.Error(err))
		return nil, toGRPCError(err)
	}

	h.logger.Info("handler: UpdateVehicle success", zap.String("vehicleID", req.VehicleId))
	return &vehiclepb.UpdateVehicleResponse{Success: true}, nil
}

// DeleteVehicle supprime un véhicule.
func (h *VehicleHandler) DeleteVehicle(ctx context.Context, req *vehiclepb.DeleteVehicleRequest) (*vehiclepb.DeleteVehicleResponse, error) {
	h.logger.Debug("handler: DeleteVehicle called", zap.String("vehicleID", req.VehicleId))

	if err := h.service.DeleteVehicle(ctx, req.UserId, req.VehicleId); err != nil {
		h.logger.Error("handler: DeleteVehicle failed", zap.Error(err))
		return nil, toGRPCError(err)
	}

	h.logger.Info("handler: DeleteVehicle success", zap.String("vehicleID", req.VehicleId))
	return &vehiclepb.DeleteVehicleResponse{Success: true}, nil
}

// GetVehicleDetails retourne les détails complets d'un véhicule.
func (h *VehicleHandler) GetVehicleDetails(ctx context.Context, req *vehiclepb.GetVehicleDetailsRequest) (*vehiclepb.GetVehicleDetailsResponse, error) {
	h.logger.Debug("handler: GetVehicleDetails called", zap.String("vehicleID", req.VehicleId))

	details, err := h.service.GetVehicleDetails(ctx, req.UserId, req.VehicleId)
	if err != nil {
		h.logger.Error("handler: GetVehicleDetails failed", zap.Error(err))
		return nil, toGRPCError(err)
	}

	return &vehiclepb.GetVehicleDetailsResponse{
		Vehicle: &vehiclepb.VehicleDetail{
			VehicleId:     details.Vehicle.VehicleID,
			UserId:        details.Vehicle.UserID,
			Brand:         details.Vehicle.Brand,
			NumberOfSeats: int32(details.Vehicle.NumberOfSeats),
			BrandModel:    details.Vehicle.BrandModel,
			Color:         details.Vehicle.Color,
			LicencePlate:  details.Vehicle.LicencePlate,
			IsVerified:    details.Vehicle.IsVerified,
			Documents: &vehiclepb.VehicleDocuments{
				AssuranceUrl:              details.Documents.AssuranceURL,
				VehicleRegistrationUrl:    details.Documents.VehicleRegistrationURL,
				DriverLicenceUrl:          details.Documents.DriverLicenceURL,
				AssuranceStatus:           details.Documents.AssuranceStatus,
				VehicleRegistrationStatus: details.Documents.VehicleRegistrationStatus,
			},
		},
	}, nil
}

// GetUserVehicles retourne la liste des véhicules d'un utilisateur.
func (h *VehicleHandler) GetUserVehicles(ctx context.Context, req *vehiclepb.GetUserVehiclesRequest) (*vehiclepb.GetUserVehiclesResponse, error) {
	h.logger.Debug("handler: GetUserVehicles called", zap.String("userID", req.UserId))

	previews, err := h.service.GetUserVehicles(ctx, req.UserId)
	if err != nil {
		h.logger.Error("handler: GetUserVehicles failed", zap.Error(err))
		return nil, toGRPCError(err)
	}

	protoVehicles := make([]*vehiclepb.VehiclePreview, 0, len(previews))
	for _, p := range previews {
		protoVehicles = append(protoVehicles, &vehiclepb.VehiclePreview{
			VehicleId:                 p.VehicleID,
			Brand:                     p.Brand,
			BrandModel:                p.BrandModel,
			LicencePlate:              p.LicencePlate,
			IsVerified:                p.IsVerified,
			NumberOfSeats:             int32(p.NumberOfSeats),
			AssuranceStatus:           p.AssuranceStatus,
			VehicleRegistrationStatus: p.VehicleRegistrationStatus,
			DriverLicenceStatus:       p.DriverLicenceStatus,
			TripCount:                 p.TripCount,
		})
	}

	return &vehiclepb.GetUserVehiclesResponse{Vehicles: protoVehicles}, nil
}

// GetVehicleInfo retourne les infos essentielles d'un véhicule (inter-service, sans auth).
func (h *VehicleHandler) GetVehicleInfo(ctx context.Context, req *vehiclepb.GetVehicleInfoRequest) (*vehiclepb.GetVehicleInfoResponse, error) {
	h.logger.Debug("handler: GetVehicleInfo called", zap.String("vehicleID", req.VehicleId))

	vehicle, err := h.service.GetVehicleInfo(ctx, req.VehicleId)
	if err != nil {
		// Véhicule introuvable = cas attendu (appelant inter-service dégrade gracieusement) :
		// on log en Debug pour éviter une fausse alerte ERROR avec stack trace.
		if errors.Is(err, vehicleErrors.ErrorVehicleNotFound) {
			h.logger.Debug("handler: GetVehicleInfo — vehicle not found", zap.String("vehicleID", req.VehicleId))
		} else {
			h.logger.Error("handler: GetVehicleInfo failed", zap.Error(err))
		}
		return nil, toGRPCError(err)
	}

	return &vehiclepb.GetVehicleInfoResponse{
		Vehicle: &vehiclepb.VehicleInfo{
			VehicleId:     vehicle.VehicleID,
			Brand:         vehicle.Brand,
			BrandModel:    vehicle.BrandModel,
			Color:         vehicle.Color,
			LicencePlate:  vehicle.LicencePlate,
			NumberOfSeats: int32(vehicle.NumberOfSeats),
			IsVerified:    vehicle.IsVerified,
		},
	}, nil
}

// Health retourne l'état de santé du service (route publique, sans auth).
func (h *VehicleHandler) Health(_ context.Context, _ *vehiclepb.HealthRequest) (*vehiclepb.HealthResponse, error) {
	return &vehiclepb.HealthResponse{
		Status:    "healthy",
		Version:   serviceVersion,
		Timestamp: time.Now().Unix(),
	}, nil
}

// toGRPCError traduit les erreurs domaine en codes de statut gRPC.
func toGRPCError(err error) error {
	switch {
	case errors.Is(err, vehicleErrors.ErrorVehicleNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, vehicleErrors.ErrorLicencePlateConflict):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, vehicleErrors.ErrorUnauthorized):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, vehicleErrors.ErrorInvalidInput):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, vehicleErrors.ErrorDataRetrievalFailed),
		errors.Is(err, vehicleErrors.ErrorInternalServer):
		return status.Error(codes.Internal, err.Error())
	default:
		return status.Error(codes.Internal, "internal server error")
	}
}
