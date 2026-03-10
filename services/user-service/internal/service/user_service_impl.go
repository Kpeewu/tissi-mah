package service

import (
	"context"
	"time"

	"github.com/Kpeewu/tissi-mah/services/user-service/internal/client"
	"github.com/Kpeewu/tissi-mah/services/user-service/internal/domain"
	"github.com/Kpeewu/tissi-mah/services/user-service/internal/middleware"
	repoInterfaces "github.com/Kpeewu/tissi-mah/services/user-service/internal/repository/interfaces"
	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/user-service/internal/service/interfaces"
	userErrors "github.com/Kpeewu/tissi-mah/services/user-service/pkg/errors"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type userServiceImpl struct {
	readRepo   repoInterfaces.UserRepositoryRead
	writeRepo  repoInterfaces.UserRepositoryWrite
	authClient client.AuthClient
	logger     *zap.Logger
}

func NewUserService(
	readRepo repoInterfaces.UserRepositoryRead,
	writeRepo repoInterfaces.UserRepositoryWrite,
	authClient client.AuthClient,
	logger *zap.Logger,
) serviceInterfaces.UserService {
	return &userServiceImpl{
		readRepo:   readRepo,
		writeRepo:  writeRepo,
		authClient: authClient,
		logger:     logger.Named("service"),
	}
}

// CreateUser crée un nouveau profil utilisateur (appelé par auth-service)
func (s *userServiceImpl) CreateUser(ctx context.Context, authID string, firebaseID string, name string, firstName string, profilePhotoURL string) (*domain.User, error) {
	s.logger.Debug("création profil utilisateur",
		zap.String("auth_id", authID),
		zap.String("firebase_id", firebaseID),
		zap.String("name", name),
	)

	now := time.Now().UTC()
	user := &domain.User{
		UserID:          uuid.New().String(),
		AuthID:          authID,
		FirebaseID:      firebaseID,
		Name:            name,
		FirstName:       firstName,
		ProfileImageURL: profilePhotoURL,
		HasProfileImage: profilePhotoURL != "",
		IsPassenger:     true,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	_, err := s.writeRepo.Create(ctx, user)
	if err != nil {
		s.logger.Error("échec de la création du profil utilisateur", zap.Error(err), zap.String("auth_id", authID))
		return nil, err
	}

	s.logger.Info("profil utilisateur créé avec succès", zap.String("user_id", user.UserID), zap.String("auth_id", authID))
	return user, nil
}

// GetUserByAuthID récupère le profil utilisateur par AuthID (appelé par auth-service)
func (s *userServiceImpl) GetUserByAuthID(ctx context.Context, authID string) (*domain.User, error) {
	s.logger.Debug("récupération profil par authID", zap.String("auth_id", authID))

	if authID == "" {
		return nil, userErrors.ErrorUserNotFound
	}

	user, err := s.readRepo.GetByAuthID(ctx, authID)
	if err != nil {
		s.logger.Error("échec de la récupération du profil par authID", zap.Error(err), zap.String("auth_id", authID))
		return nil, err
	}

	return user, nil
}

// GetMyProfile récupère le profil complet de l'utilisateur connecté avec enrichissement auth
func (s *userServiceImpl) GetMyProfile(ctx context.Context) (*serviceInterfaces.FullProfile, error) {
	s.logger.Debug("récupération du profil de l'utilisateur connecté")

	firebaseID, ok := ctx.Value(middleware.FirebaseIDKey).(string)
	if !ok || firebaseID == "" {
		s.logger.Error("firebase ID manquant dans le contexte")
		return nil, userErrors.ErrorInternalServer
	}

	// Lookup par Firebase ID dans MongoDB
	user, err := s.readRepo.GetByFirebaseID(ctx, firebaseID)
	if err != nil {
		s.logger.Error("échec de la récupération du profil par firebaseID", zap.Error(err), zap.String("firebase_id", firebaseID))
		return nil, err
	}

	// Enrichissement avec les données auth
	authInfo, err := s.authClient.GetAuthInfo(ctx, user.AuthID)
	if err != nil {
		s.logger.Error("échec de la récupération des données auth", zap.Error(err), zap.String("auth_id", user.AuthID))
		return nil, userErrors.ErrorAuthServiceUnavailable
	}

	s.logger.Info("profil complet récupéré avec succès", zap.String("user_id", user.UserID))
	return toFullProfile(user, authInfo), nil
}

// CreateDriverAccount active ou désactive le statut conducteur
func (s *userServiceImpl) CreateDriverAccount(ctx context.Context, profileID string, createDriver bool) error {
	s.logger.Debug("modification statut conducteur", zap.String("profile_id", profileID), zap.Bool("create_driver", createDriver))

	if profileID == "" {
		return userErrors.ErrorInvalidUserID
	}

	user, err := s.readRepo.GetByUserID(ctx, profileID)
	if err != nil {
		s.logger.Error("échec de la récupération du profil pour statut conducteur", zap.Error(err), zap.String("profile_id", profileID))
		return err
	}

	if createDriver {
		user.EnableDriverAccount()
	} else {
		user.DisableDriverAccount()
	}

	_, err = s.writeRepo.Update(ctx, user)
	if err != nil {
		s.logger.Error("échec de la mise à jour du statut conducteur", zap.Error(err), zap.String("profile_id", profileID))
		return err
	}

	s.logger.Info("statut conducteur mis à jour avec succès", zap.String("profile_id", profileID), zap.Bool("is_driver", createDriver))
	return nil
}

// AddTripPreferences ajoute les préférences de trajet
func (s *userServiceImpl) AddTripPreferences(ctx context.Context, profileID string, preferences []domain.TripPreference) error {
	s.logger.Debug("ajout préférences de trajet", zap.String("profile_id", profileID), zap.Int("nb_preferences", len(preferences)))

	if profileID == "" {
		return userErrors.ErrorInvalidUserID
	}

	user, err := s.readRepo.GetByUserID(ctx, profileID)
	if err != nil {
		s.logger.Error("échec de la récupération du profil pour préférences", zap.Error(err), zap.String("profile_id", profileID))
		return err
	}

	user.SetTripPreferences(preferences)

	_, err = s.writeRepo.Update(ctx, user)
	if err != nil {
		s.logger.Error("échec de la mise à jour des préférences", zap.Error(err), zap.String("profile_id", profileID))
		return err
	}

	s.logger.Info("préférences de trajet mises à jour avec succès", zap.String("profile_id", profileID))
	return nil
}

// UpdateProfile met à jour les informations du profil
func (s *userServiceImpl) UpdateProfile(ctx context.Context, req serviceInterfaces.UpdateProfileRequest) (*serviceInterfaces.FullProfile, error) {
	s.logger.Debug("mise à jour du profil", zap.String("profile_id", req.UserID))

	if req.UserID == "" {
		return nil, userErrors.ErrorInvalidUserID
	}

	user, err := s.readRepo.GetByUserID(ctx, req.UserID)
	if err != nil {
		s.logger.Error("échec de la récupération du profil pour mise à jour", zap.Error(err), zap.String("profile_id", req.UserID))
		return nil, err
	}

	// Mise à jour des champs optionnels
	if req.FirstName != nil {
		user.FirstName = *req.FirstName
	}
	if req.LastName != nil {
		user.Name = *req.LastName
	}
	if req.BirthDate != nil {
		user.DateOfBirth = *req.BirthDate
	}
	if req.ProfilePictureURL != nil {
		user.ProfileImageURL = *req.ProfilePictureURL
		user.HasProfileImage = *req.ProfilePictureURL != ""
	}

	updated, err := s.writeRepo.Update(ctx, user)
	if err != nil {
		s.logger.Error("échec de la mise à jour du profil", zap.Error(err), zap.String("profile_id", req.UserID))
		return nil, err
	}

	// Enrichissement avec les données auth
	authInfo, err := s.authClient.GetAuthInfo(ctx, updated.AuthID)
	if err != nil {
		s.logger.Error("échec de la récupération des données auth après mise à jour", zap.Error(err), zap.String("auth_id", updated.AuthID))
		return nil, userErrors.ErrorAuthServiceUnavailable
	}

	s.logger.Info("profil mis à jour avec succès", zap.String("profile_id", req.UserID))
	return toFullProfile(updated, authInfo), nil
}

// toFullProfile convertit un User + AuthInfo en FullProfile
func toFullProfile(user *domain.User, authInfo *client.AuthInfo) *serviceInterfaces.FullProfile {
	return &serviceInterfaces.FullProfile{
		AuthID:                     user.AuthID,
		UserID:                     user.UserID,
		Name:                       user.Name,
		FirstName:                  user.FirstName,
		Gender:                     user.Gender,
		DateOfBirth:                user.DateOfBirth,
		Bio:                        user.Bio,
		Email:                      authInfo.Email,
		PhoneNumber:                authInfo.PhoneNumber,
		ProfileImageURL:            user.ProfileImageURL,
		HasProfileImage:            user.HasProfileImage,
		IsDriver:                   user.IsDriver,
		IsPassenger:                user.IsPassenger,
		IsDriverProfileVerified:    user.IsDriverProfileVerified,
		IsPassengerProfileVerified: user.IsPassengerProfileVerified,
		IsActive:                   authInfo.IsActive,
		IsSuspended:                authInfo.IsSuspended,
		SuspensionEndDate:          authInfo.SuspensionEndDate,
		TripPreferences:            user.TripPreferences,
		IDCardExpirationDate:       user.IDCardExpirationDate,
		DriveLicenceExpirationDate: user.DriveLicenceExpirationDate,
		UserFiles:                  []domain.UserFile{},
	}
}
