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
	fileClient client.FileClient
	logger     *zap.Logger
}

func NewUserService(
	readRepo repoInterfaces.UserRepositoryRead,
	writeRepo repoInterfaces.UserRepositoryWrite,
	authClient client.AuthClient,
	fileClient client.FileClient,
	logger *zap.Logger,
) serviceInterfaces.UserService {
	return &userServiceImpl{
		readRepo:   readRepo,
		writeRepo:  writeRepo,
		authClient: authClient,
		fileClient: fileClient,
		logger:     logger.Named("service"),
	}
}

// CreateUser crée un nouveau profil utilisateur (appelé par auth-service)
func (s *userServiceImpl) CreateUser(ctx context.Context, authID string, firebaseID string, name string, firstName string, profilePhotoURL string, birthDate string) (*domain.User, error) {
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
		DateOfBirth:     birthDate,
		IsPassenger:     true,
		IsDriver:        false,
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

// GetUserByFirebaseID récupère le profil utilisateur par son Firebase UID (appelé par trips-service)
func (s *userServiceImpl) GetUserByFirebaseID(ctx context.Context, firebaseID string) (*domain.User, error) {
	s.logger.Debug("récupération profil par firebaseID", zap.String("firebase_id", firebaseID))

	if firebaseID == "" {
		return nil, userErrors.ErrorUserNotFound
	}

	user, err := s.readRepo.GetByFirebaseID(ctx, firebaseID)
	if err != nil {
		s.logger.Error("échec de la récupération du profil par firebaseID", zap.Error(err), zap.String("firebase_id", firebaseID))
		return nil, err
	}

	return user, nil
}

// GetUserByUserID récupère le profil utilisateur par son UserID interne (appelé par trips-service)
func (s *userServiceImpl) GetUserByUserID(ctx context.Context, userID string) (*domain.User, error) {
	s.logger.Debug("récupération profil par userID", zap.String("user_id", userID))

	if userID == "" {
		return nil, userErrors.ErrorUserNotFound
	}

	user, err := s.readRepo.GetByUserID(ctx, userID)
	if err != nil {
		s.logger.Error("échec de la récupération du profil par userID", zap.Error(err), zap.String("user_id", userID))
		return nil, err
	}

	return user, nil
}

// GetUsersByUserIDs récupère plusieurs profils par UserID (batch, inter-service).
// Léger : pas d'enrichissement email/phone (réservé à la page finale côté appelant).
func (s *userServiceImpl) GetUsersByUserIDs(ctx context.Context, userIDs []string) ([]*domain.User, error) {
	s.logger.Debug("récupération profils batch par userIDs", zap.Int("count", len(userIDs)))
	if len(userIDs) == 0 {
		return []*domain.User{}, nil
	}
	return s.readRepo.GetByUserIDs(ctx, userIDs)
}

// GetUserProfileByUserID récupère le profil utilisateur enrichi avec email/phone depuis auth-service.
// Utilisé par notification-service et le back-office support (photo de profil fraîche incluse).
func (s *userServiceImpl) GetUserProfileByUserID(ctx context.Context, userID string) (*domain.User, string, string, error) {
	user, err := s.GetUserByUserID(ctx, userID)
	if err != nil {
		return nil, "", "", err
	}

	s.refreshProfileImage(ctx, user)

	authInfo, err := s.authClient.GetAuthInfo(ctx, user.AuthID)
	if err != nil {
		s.logger.Warn("échec récupération auth info pour enrichissement", zap.Error(err), zap.String("auth_id", user.AuthID))
		// Retourner le user sans email/phone plutôt que de faire échouer
		return user, "", "", nil
	}

	return user, authInfo.Email, authInfo.PhoneNumber, nil
}

// SoftDeleteUser anonymise et soft-delete le profil utilisateur (appelé par auth-service)
func (s *userServiceImpl) SoftDeleteUser(ctx context.Context, authID string) error {
	s.logger.Debug("suppression profil utilisateur", zap.String("auth_id", authID))

	if authID == "" {
		return userErrors.ErrorInvalidUserID
	}

	user, err := s.readRepo.GetByAuthID(ctx, authID)
	if err != nil {
		s.logger.Error("échec de la récupération du profil pour suppression", zap.Error(err), zap.String("auth_id", authID))
		return err
	}

	user.AnonymizeAndDelete()

	if err := s.writeRepo.AnonymizeAndDelete(ctx, user); err != nil {
		s.logger.Error("échec de l'anonymisation du profil", zap.Error(err), zap.String("auth_id", authID))
		return err
	}

	s.logger.Info("profil utilisateur anonymisé et supprimé", zap.String("user_id", user.UserID), zap.String("auth_id", authID))
	return nil
}

// resolveContextUser résout l'utilisateur connecté depuis le Firebase UID stocké
// dans le contexte gRPC (middleware.FirebaseIDKey). Les endpoints client-facing
// ne font JAMAIS confiance à un UserID fourni dans le body.
func (s *userServiceImpl) resolveContextUser(ctx context.Context) (*domain.User, error) {
	firebaseID, ok := ctx.Value(middleware.FirebaseIDKey).(string)
	if !ok || firebaseID == "" {
		s.logger.Error("firebase ID manquant dans le contexte")
		return nil, userErrors.ErrorInternalServer
	}
	user, err := s.readRepo.GetByFirebaseID(ctx, firebaseID)
	if err != nil {
		s.logger.Error("échec de la récupération du profil par firebaseID", zap.Error(err), zap.String("firebase_id", firebaseID))
		return nil, err
	}
	return user, nil
}

// refreshProfileImage résout la photo de profil À LA LECTURE : URL présignée
// fraîche du selfie courant (fallback profilePicture historique). Les URLs
// présignées expirent — la valeur MongoDB ne sert que de repli (ex : photo
// externe importée à la création du compte).
func (s *userServiceImpl) refreshProfileImage(ctx context.Context, user *domain.User) {
	if url := s.fileClient.GetCurrentDocumentURL(ctx, user.UserID, "selfie"); url != "" {
		user.ProfileImageURL = url
		user.HasProfileImage = true
		return
	}
	if url := s.fileClient.GetCurrentDocumentURL(ctx, user.UserID, "profilePicture"); url != "" {
		user.ProfileImageURL = url
		user.HasProfileImage = true
	}
}

// GetMyProfile récupère le profil complet de l'utilisateur connecté avec enrichissement auth
func (s *userServiceImpl) GetMyProfile(ctx context.Context) (*serviceInterfaces.FullProfile, error) {
	s.logger.Debug("récupération du profil de l'utilisateur connecté")

	user, err := s.resolveContextUser(ctx)
	if err != nil {
		return nil, err
	}

	// Enrichissement avec les données auth
	authInfo, err := s.authClient.GetAuthInfo(ctx, user.AuthID)
	if err != nil {
		s.logger.Error("échec de la récupération des données auth", zap.Error(err), zap.String("auth_id", user.AuthID))
		return nil, userErrors.ErrorAuthServiceUnavailable
	}

	idExpiry := s.getDocExpiry(ctx, user.UserID, "idCardFront", "idCardBack")
	drExpiry := s.getDocExpiry(ctx, user.UserID, "driverLicenceFront", "driverLicenceBack")
	s.refreshProfileImage(ctx, user)

	s.logger.Info("profil complet récupéré avec succès", zap.String("user_id", user.UserID))
	return toFullProfile(user, authInfo, idExpiry, drExpiry), nil
}

// CreateDriverAccount active ou désactive le statut conducteur.
// L'utilisateur est résolu depuis le Firebase UID du contexte (jamais du body).
func (s *userServiceImpl) CreateDriverAccount(ctx context.Context, createDriver bool) error {
	s.logger.Debug("modification statut conducteur", zap.Bool("create_driver", createDriver))

	user, err := s.resolveContextUser(ctx)
	if err != nil {
		return err
	}

	if createDriver {
		user.EnableDriverAccount()
	} else {
		user.DisableDriverAccount()
	}

	_, err = s.writeRepo.Update(ctx, user)
	if err != nil {
		s.logger.Error("échec de la mise à jour du statut conducteur", zap.Error(err), zap.String("profile_id", user.UserID))
		return err
	}

	s.logger.Info("statut conducteur mis à jour avec succès", zap.String("profile_id", user.UserID), zap.Bool("is_driver", createDriver))
	return nil
}

// UpdateProfileVerification met à jour les flags de vérification KYC (appelé par
// kyc-service). Retourne les valeurs PRÉCÉDENTES des flags pour que kyc-service
// détecte les bascules false→true et publie les notifications correspondantes.
func (s *userServiceImpl) UpdateProfileVerification(ctx context.Context, userID string, driver, passenger *bool) (bool, bool, error) {
	s.logger.Debug("mise à jour vérification profil",
		zap.String("user_id", userID),
		zap.Any("driver", driver),
		zap.Any("passenger", passenger),
	)

	if userID == "" {
		return false, false, userErrors.ErrorInvalidUserID
	}

	user, err := s.readRepo.GetByUserID(ctx, userID)
	if err != nil {
		s.logger.Error("échec de la récupération du profil pour vérification", zap.Error(err), zap.String("user_id", userID))
		return false, false, err
	}

	prevDriver := user.IsDriverProfileVerified
	prevPassenger := user.IsPassengerProfileVerified

	user.SetProfileVerification(driver, passenger)

	_, err = s.writeRepo.Update(ctx, user)
	if err != nil {
		s.logger.Error("échec de la mise à jour de la vérification du profil", zap.Error(err), zap.String("user_id", userID))
		return false, false, err
	}

	s.logger.Info("vérification du profil mise à jour avec succès",
		zap.String("user_id", userID),
		zap.Bool("is_driver_verified", user.IsDriverProfileVerified),
		zap.Bool("is_passenger_verified", user.IsPassengerProfileVerified),
	)
	return prevDriver, prevPassenger, nil
}

// AddTripPreferences ajoute les préférences de trajet.
// L'utilisateur est résolu depuis le Firebase UID du contexte (jamais du body).
func (s *userServiceImpl) AddTripPreferences(ctx context.Context, preferences []domain.TripPreference) error {
	s.logger.Debug("ajout préférences de trajet", zap.Int("nb_preferences", len(preferences)))

	user, err := s.resolveContextUser(ctx)
	if err != nil {
		return err
	}

	user.SetTripPreferences(preferences)

	_, err = s.writeRepo.Update(ctx, user)
	if err != nil {
		s.logger.Error("échec de la mise à jour des préférences", zap.Error(err), zap.String("profile_id", user.UserID))
		return err
	}

	s.logger.Info("préférences de trajet mises à jour avec succès", zap.String("profile_id", user.UserID))
	return nil
}

// UpdateProfile met à jour les informations du profil.
// L'utilisateur est résolu depuis le Firebase UID du contexte (jamais du body).
// NB : la photo de profil n'est PAS modifiable ici — c'est le selfie d'identité,
// soumis via POST /api/v1/file/uploadSelfie et validé par le support.
func (s *userServiceImpl) UpdateProfile(ctx context.Context, req serviceInterfaces.UpdateProfileRequest) (*serviceInterfaces.FullProfile, error) {
	s.logger.Debug("mise à jour du profil")

	user, err := s.resolveContextUser(ctx)
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
	if req.WithdrawNumber != nil {
		user.WithdrawNumber = *req.WithdrawNumber
	}
	if req.Bio != nil {
		user.Bio = *req.Bio
	}
	updated, err := s.writeRepo.Update(ctx, user)
	if err != nil {
		s.logger.Error("échec de la mise à jour du profil", zap.Error(err), zap.String("profile_id", user.UserID))
		return nil, err
	}

	// Enrichissement avec les données auth
	authInfo, err := s.authClient.GetAuthInfo(ctx, updated.AuthID)
	if err != nil {
		s.logger.Error("échec de la récupération des données auth après mise à jour", zap.Error(err), zap.String("auth_id", updated.AuthID))
		return nil, userErrors.ErrorAuthServiceUnavailable
	}

	idExpiry := s.getDocExpiry(ctx, updated.UserID, "idCardFront", "idCardBack")
	drExpiry := s.getDocExpiry(ctx, updated.UserID, "driverLicenceFront", "driverLicenceBack")
	s.refreshProfileImage(ctx, updated)

	s.logger.Info("profil mis à jour avec succès", zap.String("profile_id", updated.UserID))
	return toFullProfile(updated, authInfo, idExpiry, drExpiry), nil
}

// getDocExpiry récupère expired_at du document courant, en essayant chaque type dans l'ordre.
// Retourne "" si aucun document n'est trouvé ou si file-service est indisponible.
func (s *userServiceImpl) getDocExpiry(ctx context.Context, userID string, types ...string) string {
	for _, docType := range types {
		if exp := s.fileClient.GetDocumentExpiry(ctx, userID, docType); exp != "" {
			return exp
		}
	}
	return ""
}

// toFullProfile convertit un User + AuthInfo + dates d'expiration en FullProfile
func toFullProfile(user *domain.User, authInfo *client.AuthInfo, idExpiry, drExpiry string) *serviceInterfaces.FullProfile {
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
		IDCardExpirationDate:       idExpiry,
		DriveLicenceExpirationDate: drExpiry,
		WithdrawNumber:             user.WithdrawNumber,
	}
}
