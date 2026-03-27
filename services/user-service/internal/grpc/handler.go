package grpc

import (
	"context"
	"errors"
	"time"

	"github.com/Kpeewu/tissi-mah/services/user-service/internal/domain"
	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/user-service/internal/service/interfaces"
	userErrors "github.com/Kpeewu/tissi-mah/services/user-service/pkg/errors"
	userpb "github.com/Kpeewu/tissi-mah/services/user-service/proto/gen"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const serviceVersion = "1.0.0"

// UserHandler implémente userpb.UserServiceServer.
// Il traduit les requêtes proto en appels de service et mappe les erreurs domaine
// vers les codes gRPC appropriés.
type UserHandler struct {
	userpb.UnimplementedUserServiceServer
	service serviceInterfaces.UserService
	logger  *zap.Logger
}

func NewUserHandler(service serviceInterfaces.UserService, logger *zap.Logger) *UserHandler {
	return &UserHandler{
		service: service,
		logger:  logger.Named("handler"),
	}
}

// --- Inter-service RPCs ---

// CreateUser crée un profil utilisateur (appelé par auth-service après création du compte)
func (h *UserHandler) CreateUser(ctx context.Context, req *userpb.CreateUserRequest) (*userpb.UserProfileResponse, error) {
	h.logger.Debug("CreateUser appelé", zap.String("auth_id", req.AuthID), zap.String("firebase_id", req.FirebaseID))

	user, err := h.service.CreateUser(ctx, req.AuthID, req.FirebaseID, req.Name, req.FirstName, req.ProfilePhotoURL)
	if err != nil {
		h.logger.Error("CreateUser échoué", zap.Error(err), zap.String("auth_id", req.AuthID))
		return nil, toGRPCError(err)
	}

	h.logger.Info("CreateUser réussi", zap.String("user_id", user.UserID), zap.String("auth_id", req.AuthID))
	return toProtoUserProfile(user), nil
}

// GetUserByAuthID récupère le profil utilisateur par AuthID (appelé par auth-service au login)
func (h *UserHandler) GetUserByAuthID(ctx context.Context, req *userpb.GetUserByAuthIDRequest) (*userpb.UserProfileResponse, error) {
	h.logger.Debug("GetUserByAuthID appelé", zap.String("auth_id", req.AuthID))

	user, err := h.service.GetUserByAuthID(ctx, req.AuthID)
	if err != nil {
		h.logger.Error("GetUserByAuthID échoué", zap.Error(err), zap.String("auth_id", req.AuthID))
		return nil, toGRPCError(err)
	}

	h.logger.Debug("GetUserByAuthID réussi", zap.String("auth_id", req.AuthID), zap.String("user_id", user.UserID))
	return toProtoUserProfile(user), nil
}

// GetUserByUserID récupère le profil utilisateur par son UserID interne (appelé par trips-service)
func (h *UserHandler) GetUserByUserID(ctx context.Context, req *userpb.GetUserByUserIDRequest) (*userpb.UserProfileResponse, error) {
	h.logger.Debug("GetUserByUserID appelé", zap.String("user_id", req.UserID))

	user, err := h.service.GetUserByUserID(ctx, req.UserID)
	if err != nil {
		h.logger.Error("GetUserByUserID échoué", zap.Error(err), zap.String("user_id", req.UserID))
		return nil, toGRPCError(err)
	}

	h.logger.Debug("GetUserByUserID réussi", zap.String("user_id", req.UserID))
	return toProtoUserProfile(user), nil
}

// --- Client-facing RPCs ---

// GetMyProfile récupère le profil complet de l'utilisateur connecté
func (h *UserHandler) GetMyProfile(ctx context.Context, _ *userpb.GetMyProfileRequest) (*userpb.GetMyProfileResponse, error) {
	h.logger.Debug("GetMyProfile appelé")

	profile, err := h.service.GetMyProfile(ctx)
	if err != nil {
		h.logger.Error("GetMyProfile échoué", zap.Error(err))
		return nil, toGRPCError(err)
	}

	h.logger.Info("GetMyProfile réussi", zap.String("profile_id", profile.UserID))
	return &userpb.GetMyProfileResponse{
		User: toProtoFullProfile(profile),
	}, nil
}

// CreateDriverAccount active ou désactive le statut conducteur
func (h *UserHandler) CreateDriverAccount(ctx context.Context, req *userpb.CreateDriverAccountRequest) (*userpb.OperationResponse, error) {
	h.logger.Debug("CreateDriverAccount appelé", zap.String("profile_id", req.UserID), zap.Bool("create_driver", req.CreateDriverAccount))

	err := h.service.CreateDriverAccount(ctx, req.UserID, req.CreateDriverAccount)
	if err != nil {
		h.logger.Error("CreateDriverAccount échoué", zap.Error(err), zap.String("profile_id", req.UserID))
		return nil, toGRPCError(err)
	}

	h.logger.Info("CreateDriverAccount réussi", zap.String("profile_id", req.UserID))
	return &userpb.OperationResponse{Success: true}, nil
}

// AddTripPreferences ajoute les préférences de trajet
func (h *UserHandler) AddTripPreferences(ctx context.Context, req *userpb.AddTripPreferencesRequest) (*userpb.OperationResponse, error) {
	h.logger.Debug("AddTripPreferences appelé", zap.String("profile_id", req.UserID), zap.Int("nb_preferences", len(req.Preferences)))

	prefs := make([]domain.TripPreference, len(req.Preferences))
	for i, p := range req.Preferences {
		prefs[i] = domain.TripPreference{
			Preference: p.Preference,
			IsAllowed:  p.IsAllowed,
		}
	}

	err := h.service.AddTripPreferences(ctx, req.UserID, prefs)
	if err != nil {
		h.logger.Error("AddTripPreferences échoué", zap.Error(err), zap.String("profile_id", req.UserID))
		return nil, toGRPCError(err)
	}

	h.logger.Info("AddTripPreferences réussi", zap.String("profile_id", req.UserID))
	return &userpb.OperationResponse{Success: true}, nil
}

// UpdateProfile met à jour les informations du profil
func (h *UserHandler) UpdateProfile(ctx context.Context, req *userpb.UpdateProfileRequest) (*userpb.UpdateProfileResponse, error) {
	h.logger.Debug("UpdateProfile appelé", zap.String("profile_id", req.UserID))

	updateReq := serviceInterfaces.UpdateProfileRequest{
		UserID: req.UserID,
	}

	if req.FirstName != nil {
		v := *req.FirstName
		updateReq.FirstName = &v
	}
	if req.LastName != nil {
		v := *req.LastName
		updateReq.LastName = &v
	}
	if req.BirthDate != nil {
		v := *req.BirthDate
		updateReq.BirthDate = &v
	}
	if req.Email != nil {
		v := *req.Email
		updateReq.Email = &v
	}
	if req.PhoneNumber != nil {
		v := *req.PhoneNumber
		updateReq.PhoneNumber = &v
	}
	if req.WithdrawNumber != nil {
		v := *req.WithdrawNumber
		updateReq.WithdrawNumber = &v
	}
	profile, err := h.service.UpdateProfile(ctx, updateReq)
	if err != nil {
		h.logger.Error("UpdateProfile échoué", zap.Error(err), zap.String("profile_id", req.UserID))
		return nil, toGRPCError(err)
	}

	h.logger.Info("UpdateProfile réussi", zap.String("profile_id", req.UserID))
	return &userpb.UpdateProfileResponse{
		User: toProtoFullProfile(profile),
	}, nil
}

// ChangeProfilePicture uploade la nouvelle photo de profil et met à jour le profil.
func (h *UserHandler) ChangeProfilePicture(ctx context.Context, req *userpb.ChangeProfilePictureRequest) (*userpb.ChangeProfilePictureResponse, error) {
	h.logger.Debug("ChangeProfilePicture appelé", zap.String("user_id", req.UserID))

	if req.UserID == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id est requis")
	}
	if len(req.NewProfilePicture) == 0 {
		return nil, status.Error(codes.InvalidArgument, "new_profile_picture est requis")
	}

	profile, err := h.service.ChangeProfilePicture(ctx, req.UserID, req.NewProfilePicture)
	if err != nil {
		h.logger.Error("ChangeProfilePicture échoué", zap.Error(err), zap.String("user_id", req.UserID))
		return nil, toGRPCError(err)
	}

	h.logger.Info("ChangeProfilePicture réussi", zap.String("user_id", req.UserID))
	return &userpb.ChangeProfilePictureResponse{
		User: toProtoFullProfile(profile),
	}, nil
}

// Health retourne l'état de santé du service (route publique, sans auth).
func (h *UserHandler) Health(_ context.Context, _ *userpb.HealthRequest) (*userpb.HealthResponse, error) {
	return &userpb.HealthResponse{
		Status:    "healthy",
		Version:   serviceVersion,
		Timestamp: time.Now().Unix(),
	}, nil
}

// toGRPCError traduit les erreurs domaine en codes de statut gRPC.
func toGRPCError(err error) error {
	switch {
	case errors.Is(err, userErrors.ErrorUserNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, userErrors.ErrorProfileAlreadyExists):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, userErrors.ErrorInvalidUserID):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, userErrors.ErrorAuthServiceUnavailable):
		return status.Error(codes.Unavailable, err.Error())
	case errors.Is(err, userErrors.ErrorDataRetrievalFailed),
		errors.Is(err, userErrors.ErrorInternalServer):
		return status.Error(codes.Internal, err.Error())
	default:
		return status.Error(codes.Internal, "internal server error")
	}
}

// toProtoTripPreferences convertit []domain.TripPreference en []*userpb.TripPreference
func toProtoTripPreferences(prefs []domain.TripPreference) []*userpb.TripPreference {
	result := make([]*userpb.TripPreference, len(prefs))
	for i, p := range prefs {
		result[i] = &userpb.TripPreference{
			Preference: p.Preference,
			IsAllowed:  p.IsAllowed,
		}
	}
	return result
}

// toProtoUserProfile convertit domain.User en message proto UserProfileResponse (inter-service)
func toProtoUserProfile(u *domain.User) *userpb.UserProfileResponse {
	return &userpb.UserProfileResponse{
		UserID:                     u.UserID,
		AuthID:                     u.AuthID,
		Name:                       u.Name,
		FirstName:                  u.FirstName,
		Gender:                     u.Gender,
		DateOfBirth:                u.DateOfBirth,
		Bio:                        u.Bio,
		HasProfileImage:            u.HasProfileImage,
		ProfileImageURL:            u.ProfileImageURL,
		IsDriver:                   u.IsDriver,
		IsPassenger:                u.IsPassenger,
		IsDriverProfileVerified:    u.IsDriverProfileVerified,
		IsPassengerProfileVerified: u.IsPassengerProfileVerified,
		TripPreferences:            toProtoTripPreferences(u.TripPreferences),
		WithdrawNumber:             u.WithdrawNumber,
	}
}

// toProtoFullProfile convertit FullProfile en message proto FullUserProfile (client-facing)
func toProtoFullProfile(p *serviceInterfaces.FullProfile) *userpb.FullUserProfile {
	return &userpb.FullUserProfile{
		AuthID:                     p.AuthID,
		UserID:                     p.UserID,
		Name:                       p.Name,
		FirstName:                  p.FirstName,
		Gender:                     p.Gender,
		DateOfBirth:                p.DateOfBirth,
		Bio:                        p.Bio,
		Email:                      p.Email,
		PhoneNumber:                p.PhoneNumber,
		ProfileImageURL:            p.ProfileImageURL,
		HasProfileImage:            p.HasProfileImage,
		IsDriver:                   p.IsDriver,
		IsPassenger:                p.IsPassenger,
		IsDriverProfileVerified:    p.IsDriverProfileVerified,
		IsPassengerProfileVerified: p.IsPassengerProfileVerified,
		IsActive:                   p.IsActive,
		IsSuspended:                p.IsSuspended,
		SuspensionEndDate:          p.SuspensionEndDate,
		TripPreferences:            toProtoTripPreferences(p.TripPreferences),
		IDCardExpirationDate:       p.IDCardExpirationDate,
		DriveLicenceExpirationDate: p.DriveLicenceExpirationDate,
		WithdrawNumber:             p.WithdrawNumber,
	}
}
