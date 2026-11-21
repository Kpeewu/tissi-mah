package service

import (
	"context"
	"time"

	"github.com/Kpeewu/tissi-mah/pkg/notification"
	"github.com/Kpeewu/tissi-mah/services/auth-service/internal/client"
	"github.com/Kpeewu/tissi-mah/services/auth-service/internal/domain"
	"github.com/Kpeewu/tissi-mah/services/auth-service/internal/middleware"
	repoInterfaces "github.com/Kpeewu/tissi-mah/services/auth-service/internal/repository/interfaces"
	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/auth-service/internal/service/interfaces"
	authErrors "github.com/Kpeewu/tissi-mah/services/auth-service/pkg/errors"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type authServiceImpl struct {
	readRepo        repoInterfaces.AuthRepositoryRead
	writeRepo       repoInterfaces.AuthRepositoryWrite
	userClient      client.UserClient
	tripsClient     client.TripsClient
	bookingClient   client.BookingClient
	paymentClient   client.PaymentClient
	chatClient      client.ChatClient
	fileClient      client.FileClient
	redisClient     *redis.Client
	suspensionRedis *redis.Client
	logger          *zap.Logger
}

func NewAuthService(
	readRepo repoInterfaces.AuthRepositoryRead,
	writeRepo repoInterfaces.AuthRepositoryWrite,
	userClient client.UserClient,
	tripsClient client.TripsClient,
	bookingClient client.BookingClient,
	paymentClient client.PaymentClient,
	chatClient client.ChatClient,
	fileClient client.FileClient,
	redisClient *redis.Client,
	suspensionRedis *redis.Client,
	logger *zap.Logger) serviceInterfaces.AuthService {

	return &authServiceImpl{
		readRepo:        readRepo,
		writeRepo:       writeRepo,
		userClient:      userClient,
		tripsClient:     tripsClient,
		bookingClient:   bookingClient,
		paymentClient:   paymentClient,
		chatClient:      chatClient,
		fileClient:      fileClient,
		redisClient:     redisClient,
		suspensionRedis: suspensionRedis,
		logger:          logger,
	}
}

// GetUserByFirebaseID récupère les données d'authentification d'un utilisateur par son FirebaseID
func (s *authServiceImpl) GetUserByFirebaseID(ctx context.Context, firebaseID string) (*domain.Auth, error) {
	if firebaseID == "" {
		return nil, authErrors.ErrorUserNotFound
	}

	auth, err := s.readRepo.GetByFirebaseID(ctx, firebaseID)
	if err != nil {
		return nil, err
	}

	return auth, nil
}

// RegisterUser crée un nouveau compte auth et un profil utilisateur dans le user-service
func (s *authServiceImpl) RegisterUser(ctx context.Context, name string, firstName string, email string, phoneNumber string, profilePhotoURL string, birthDate string) (*domain.UserPreview, error) {
	firebaseID, ok := ctx.Value(middleware.FirebaseIDKey).(string)
	if !ok || firebaseID == "" {
		s.logger.Error("firebase ID missing from context")
		return nil, authErrors.ErrorInternalServer
	}

	// Normalisation des entrées
	email = domain.NormalizeEmail(email)
	phoneNumber = domain.NormalizePhone(phoneNumber)

	s.logger.Debug("register user", zap.String("firebaseID", firebaseID), zap.String("email", email))

	// Validation du format email
	if email != "" {
		if err := domain.ValidateEmail(email); err != nil {
			return nil, err
		}
	}

	// Validation du format téléphone
	if phoneNumber != "" {
		if err := domain.ValidatePhone(phoneNumber); err != nil {
			return nil, err
		}
	}

	// Vérification de la disponibilité de l'email
	if email != "" {
		exists, err := s.readRepo.EmailExists(ctx, email)
		if err != nil {
			s.logger.Error("email check failed", zap.String("email", email), zap.Error(err))
			return nil, err
		}
		if exists {
			return nil, authErrors.ErrorEmailNotAvailable
		}
	}

	// Vérification de la disponibilité du numéro de téléphone
	if phoneNumber != "" {
		exists, err := s.readRepo.PhoneNumberExists(ctx, phoneNumber)
		if err != nil {
			s.logger.Error("phone check failed", zap.String("phone", phoneNumber), zap.Error(err))
			return nil, err
		}
		if exists {
			return nil, authErrors.ErrorPhoneNumberNotAvailable
		}
	}

	var emailPtr *string
	if email != "" {
		emailPtr = &email
	}

	var phonePtr *string
	if phoneNumber != "" {
		phonePtr = &phoneNumber
	}

	auth := &domain.Auth{
		AuthID:      uuid.New().String(),
		FirebaseID:  firebaseID,
		Email:       emailPtr,
		PhoneNumber: phonePtr,
		IsActive:    true,
	}

	authID, err := s.writeRepo.Create(ctx, auth)
	if err != nil {
		s.logger.Error("create auth record failed", zap.Error(err))
		return nil, err
	}

	s.logger.Debug("auth record created", zap.String("authID", authID))

	// Création du profil utilisateur dans le user-service
	// Email et PhoneNumber sont stockés dans auth-service, pas dans user-service
	userPreview, err := s.userClient.CreateUser(ctx, authID, firebaseID, name, firstName, profilePhotoURL, birthDate)
	if err != nil {
		s.logger.Error("user-service CreateUser failed, rolling back auth record",
			zap.Error(err), zap.String("authID", authID))

		// Compensation : suppression de l'auth record orphelin
		if deleteErr := s.writeRepo.Delete(ctx, auth); deleteErr != nil {
			s.logger.Error("failed to rollback auth record after user-service failure",
				zap.Error(deleteErr), zap.String("authID", authID))
		}

		return nil, authErrors.ErrorInternalServer
	}

	// Enrichissement avec les données auth (email/phone)
	userPreview.Email = auth.Email
	userPreview.PhoneNumber = auth.PhoneNumber

	s.logger.Info("user registered", zap.String("authID", authID), zap.String("firebaseID", firebaseID))

	// Publication de l'event WELCOME (non bloquant)
	welcomeEvent := notification.Event{
		EventType:     notification.Welcome,
		UserID:        userPreview.UserID,
		ReferenceID:   authID,
		ReferenceType: notification.RefUser,
		Payload: map[string]string{
			"first_name": firstName,
			"user_name":  name,
			"email":      email,
		},
	}
	if s.redisClient != nil {
		if err := notification.Publish(ctx, s.redisClient, welcomeEvent); err != nil {
			s.logger.Error("failed to publish welcome notification", zap.Error(err))
		}
	}

	return userPreview, nil
}

// CheckEmail vérifie si un email est disponible (non utilisé)
func (s *authServiceImpl) CheckEmail(ctx context.Context, email string) (bool, error) {
	email = domain.NormalizeEmail(email)
	if err := domain.ValidateEmail(email); err != nil {
		return false, err
	}

	exists, err := s.readRepo.EmailExists(ctx, email)
	if err != nil {
		return false, err
	}

	return !exists, nil
}

// CheckPhoneNumber vérifie si un numéro de téléphone est disponible (non utilisé)
func (s *authServiceImpl) CheckPhoneNumber(ctx context.Context, phoneNumber string) (bool, error) {
	phoneNumber = domain.NormalizePhone(phoneNumber)
	if err := domain.ValidatePhone(phoneNumber); err != nil {
		return false, err
	}

	exists, err := s.readRepo.PhoneNumberExists(ctx, phoneNumber)
	if err != nil {
		return false, err
	}

	return !exists, nil
}

// SuspendAccount suspend ou bannit définitivement un compte.
// Écrit aussi une clé Redis "suspended:{firebaseUID}" consommée par l'api-gateway.
func (s *authServiceImpl) SuspendAccount(ctx context.Context, authID string, suspendedUntil *time.Time, isBanned bool) error {
	if authID == "" {
		return authErrors.ErrorUserNotFound
	}

	auth, err := s.readRepo.GetByAuthID(ctx, authID)
	if err != nil {
		return err
	}

	if err := s.writeRepo.Suspend(ctx, authID, suspendedUntil, isBanned); err != nil {
		return err
	}

	if s.suspensionRedis != nil {
		key := "suspended:" + auth.FirebaseID
		if isBanned {
			// Ban permanent : clé sans TTL
			if err := s.suspensionRedis.Set(ctx, key, "banned", 0).Err(); err != nil {
				s.logger.Error("failed to write permanent ban to Redis", zap.Error(err), zap.String("authID", authID))
			}
		} else if suspendedUntil != nil {
			ttl := time.Until(*suspendedUntil)
			if ttl > 0 {
				if err := s.suspensionRedis.Set(ctx, key, "suspended", ttl).Err(); err != nil {
					s.logger.Error("failed to write suspension to Redis", zap.Error(err), zap.String("authID", authID))
				}
			}
		}
	}

	s.logger.Info("account suspended", zap.String("authID", authID), zap.Bool("isBanned", isBanned))
	return nil
}

// GetAuthInfo récupère les données d'authentification par AuthID (inter-service)
func (s *authServiceImpl) GetAuthInfo(ctx context.Context, authID string) (*domain.Auth, error) {
	if authID == "" {
		return nil, authErrors.ErrorUserNotFound
	}

	auth, err := s.readRepo.GetByAuthID(ctx, authID)
	if err != nil {
		return nil, err
	}

	return auth, nil
}

// DeleteUserAccount vérifie les conditions bloquantes, anonymise les données dans tous les services,
// puis supprime le compte d'authentification.
func (s *authServiceImpl) DeleteUserAccount(ctx context.Context, firebaseID string) error {
	if firebaseID == "" {
		return authErrors.ErrorUserNotFound
	}

	auth, err := s.readRepo.GetByFirebaseID(ctx, firebaseID)
	if err != nil {
		return authErrors.ErrorUserNotFound
	}

	// Résoudre l'AuthID → UserID interne (user-service)
	userPreview, err := s.userClient.GetUserByAuthID(ctx, auth.AuthID)
	if err != nil {
		s.logger.Error("DeleteUserAccount: cannot resolve userID", zap.Error(err), zap.String("authID", auth.AuthID))
		return authErrors.ErrorInternalServer
	}
	userID := userPreview.UserID

	// =========================================================================
	// PHASE 1 — Vérifications bloquantes
	// =========================================================================

	tripsOK, tripsReason, err := s.tripsClient.CheckDeletionEligibility(ctx, userID)
	if err != nil {
		s.logger.Error("DeleteUserAccount: trips eligibility check failed", zap.Error(err))
		return authErrors.ErrorInternalServer
	}
	if !tripsOK {
		s.logger.Info("DeleteUserAccount blocked by trips-service", zap.String("reason", tripsReason))
		return authErrors.ErrorCantDeleteAccount
	}

	bookingOK, bookingReason, err := s.bookingClient.CheckDeletionEligibility(ctx, userID)
	if err != nil {
		s.logger.Error("DeleteUserAccount: booking eligibility check failed", zap.Error(err))
		return authErrors.ErrorInternalServer
	}
	if !bookingOK {
		s.logger.Info("DeleteUserAccount blocked by booking-service", zap.String("reason", bookingReason))
		return authErrors.ErrorCantDeleteAccount
	}

	paymentOK, paymentReason, err := s.paymentClient.CheckDeletionEligibility(ctx, userID)
	if err != nil {
		s.logger.Error("DeleteUserAccount: payment eligibility check failed", zap.Error(err))
		return authErrors.ErrorInternalServer
	}
	if !paymentOK {
		s.logger.Info("DeleteUserAccount blocked by payment-service", zap.String("reason", paymentReason))
		return authErrors.ErrorCantDeleteAccount
	}

	// =========================================================================
	// PHASE 2 — Anonymisation cross-services
	// =========================================================================

	// Récupérer les IDs de réservation du passager pour anonymiser passenger_phone_number
	bookingIDs, err := s.bookingClient.GetPassengerBookingIDs(ctx, userID)
	if err != nil {
		s.logger.Error("DeleteUserAccount: cannot fetch passenger booking IDs", zap.Error(err))
		return authErrors.ErrorInternalServer
	}

	if err := s.chatClient.AnonymizeUserData(ctx, userID); err != nil {
		s.logger.Error("CRITICAL: chat anonymization failed", zap.Error(err), zap.String("userID", userID))
		return authErrors.ErrorCantDeleteAccount
	}

	if err := s.fileClient.DeleteAllUserFiles(ctx, userID); err != nil {
		s.logger.Error("CRITICAL: file deletion failed", zap.Error(err), zap.String("userID", userID))
		return authErrors.ErrorCantDeleteAccount
	}

	if err := s.bookingClient.AnonymizeUserData(ctx, userID); err != nil {
		s.logger.Error("CRITICAL: booking anonymization failed", zap.Error(err), zap.String("userID", userID))
		return authErrors.ErrorCantDeleteAccount
	}

	if err := s.paymentClient.AnonymizeUserData(ctx, userID, bookingIDs); err != nil {
		s.logger.Error("CRITICAL: payment anonymization failed", zap.Error(err), zap.String("userID", userID))
		return authErrors.ErrorCantDeleteAccount
	}

	if err := s.tripsClient.AnonymizeUserData(ctx, userID); err != nil {
		s.logger.Error("CRITICAL: trips anonymization failed", zap.Error(err), zap.String("userID", userID))
		return authErrors.ErrorCantDeleteAccount
	}

	// Anonymiser et soft-delete le profil dans user-service
	if err := s.userClient.SoftDeleteUser(ctx, auth.AuthID); err != nil {
		s.logger.Error("CRITICAL: user-service soft-delete failed", zap.Error(err), zap.String("authID", auth.AuthID))
		return authErrors.ErrorCantDeleteAccount
	}

	// Anonymiser et soft-delete l'entrée auth (avec retry)
	if err := s.writeRepo.Delete(ctx, auth); err != nil {
		s.logger.Warn("first attempt to delete auth failed, retrying",
			zap.Error(err), zap.String("authID", auth.AuthID))

		if retryErr := s.writeRepo.Delete(ctx, auth); retryErr != nil {
			s.logger.Error("CRITICAL: auth record not deleted after full anonymization — manual cleanup required",
				zap.Error(retryErr),
				zap.String("authID", auth.AuthID),
				zap.String("userID", userID),
			)
			return authErrors.ErrorCantDeleteAccount
		}
	}

	return nil
}
