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
)

type userServiceImpl struct {
	readRepo   repoInterfaces.UserRepositoryRead
	writeRepo  repoInterfaces.UserRepositoryWrite
	authClient client.AuthClient
}

func NewUserService(
	readRepo repoInterfaces.UserRepositoryRead,
	writeRepo repoInterfaces.UserRepositoryWrite,
	authClient client.AuthClient,
) serviceInterfaces.UserService {
	return &userServiceImpl{
		readRepo:   readRepo,
		writeRepo:  writeRepo,
		authClient: authClient,
	}
}

// CreateUser crée un nouveau profil utilisateur (appelé par auth-service)
func (s *userServiceImpl) CreateUser(ctx context.Context, authID string, firebaseID string, name string, firstName string, profilePhotoURL string) (*domain.User, error) {
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
		return nil, err
	}

	return user, nil
}

// GetUserByAuthID récupère le profil utilisateur par AuthID (appelé par auth-service)
func (s *userServiceImpl) GetUserByAuthID(ctx context.Context, authID string) (*domain.User, error) {
	if authID == "" {
		return nil, userErrors.ErrorUserNotFound
	}

	return s.readRepo.GetByAuthID(ctx, authID)
}

// GetMyProfile récupère le profil complet de l'utilisateur connecté avec enrichissement auth
func (s *userServiceImpl) GetMyProfile(ctx context.Context) (*serviceInterfaces.FullProfile, error) {
	firebaseID, ok := ctx.Value(middleware.FirebaseIDKey).(string)
	if !ok || firebaseID == "" {
		return nil, userErrors.ErrorInternalServer
	}

	// Lookup par Firebase ID dans MongoDB
	user, err := s.readRepo.GetByFirebaseID(ctx, firebaseID)
	if err != nil {
		return nil, err
	}

	// Enrichissement avec les données auth
	authInfo, err := s.authClient.GetAuthInfo(ctx, user.AuthID)
	if err != nil {
		return nil, userErrors.ErrorAuthServiceUnavailable
	}

	return toFullProfile(user, authInfo), nil
}

// CreateDriverAccount active ou désactive le statut conducteur
func (s *userServiceImpl) CreateDriverAccount(ctx context.Context, profileID string, createDriver bool) error {
	if profileID == "" {
		return userErrors.ErrorInvalidProfileID
	}

	user, err := s.readRepo.GetByUserID(ctx, profileID)
	if err != nil {
		return err
	}

	if createDriver {
		user.EnableDriverAccount()
	} else {
		user.DisableDriverAccount()
	}

	_, err = s.writeRepo.Update(ctx, user)
	return err
}

// AddTripPreferences ajoute les préférences de trajet
func (s *userServiceImpl) AddTripPreferences(ctx context.Context, profileID string, preferences []domain.TripPreference) error {
	if profileID == "" {
		return userErrors.ErrorInvalidProfileID
	}

	user, err := s.readRepo.GetByUserID(ctx, profileID)
	if err != nil {
		return err
	}

	user.SetTripPreferences(preferences)

	_, err = s.writeRepo.Update(ctx, user)
	return err
}

// UpdateProfile met à jour les informations du profil
func (s *userServiceImpl) UpdateProfile(ctx context.Context, req serviceInterfaces.UpdateProfileRequest) (*serviceInterfaces.FullProfile, error) {
	if req.ProfileID == "" {
		return nil, userErrors.ErrorInvalidProfileID
	}

	user, err := s.readRepo.GetByUserID(ctx, req.ProfileID)
	if err != nil {
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
		return nil, err
	}

	// Enrichissement avec les données auth
	authInfo, err := s.authClient.GetAuthInfo(ctx, updated.AuthID)
	if err != nil {
		return nil, userErrors.ErrorAuthServiceUnavailable
	}

	return toFullProfile(updated, authInfo), nil
}

// toFullProfile convertit un User + AuthInfo en FullProfile
func toFullProfile(user *domain.User, authInfo *client.AuthInfo) *serviceInterfaces.FullProfile {
	return &serviceInterfaces.FullProfile{
		AuthID:                     user.AuthID,
		ProfileID:                  user.UserID,
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
