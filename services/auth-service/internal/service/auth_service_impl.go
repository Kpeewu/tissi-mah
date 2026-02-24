package service

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/auth-service/internal/client"
	"github.com/Kpeewu/tissi-mah/services/auth-service/internal/domain"
	"github.com/Kpeewu/tissi-mah/services/auth-service/internal/middleware"
	repoInterfaces "github.com/Kpeewu/tissi-mah/services/auth-service/internal/repository/interfaces"
	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/auth-service/internal/service/interfaces"
	authErrors "github.com/Kpeewu/tissi-mah/services/auth-service/pkg/errors"
	"github.com/google/uuid"
)

type authServiceImpl struct {
	readRepo   repoInterfaces.AuthRepositoryRead
	writeRepo  repoInterfaces.AuthRepositoryWrite
	userClient *client.UserServiceClient
}

func NewAuthService(
	readRepo repoInterfaces.AuthRepositoryRead,
	writeRepo repoInterfaces.AuthRepositoryWrite,
	userClient *client.UserServiceClient) serviceInterfaces.AuthService {

	return &authServiceImpl{
		readRepo:   readRepo,
		writeRepo:  writeRepo,
		userClient: userClient,
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
func (s *authServiceImpl) RegisterUser(ctx context.Context, name string, firstName string, email string, phoneNumber string, profilePhotoURL string) (*domain.UserPreview, error) {
	firebaseID, ok := ctx.Value(middleware.FirebaseIDKey).(string)
	if !ok || firebaseID == "" {
		return nil, authErrors.ErrorInternalServer
	}

	// Vérification de la disponibilité de l'email
	if email != "" {
		_, err := s.readRepo.EmailExists(ctx, email)
		if err != nil {
			return nil, err
		}
	}

	// Vérification de la disponibilité du numéro de téléphone
	if phoneNumber != "" {
		_, err := s.readRepo.PhoneNumberExists(ctx, phoneNumber)
		if err != nil {
			return nil, err
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
		return nil, err
	}

	// Création du profil utilisateur dans le user-service
	// Email et PhoneNumber sont stockés dans auth-service, pas dans user-service
	userPreview, err := s.userClient.CreateUser(ctx, authID, firebaseID, name, firstName, profilePhotoURL)
	if err != nil {
		return nil, authErrors.ErrorInternalServer
	}

	// Enrichissement avec les données auth (email/phone)
	userPreview.Email = auth.Email
	userPreview.PhoneNumber = auth.PhoneNumber

	return userPreview, nil
}

// LoginUser vérifie que l'utilisateur authentifié possède un compte et peut se connecter
func (s *authServiceImpl) LoginUser(ctx context.Context) (*domain.UserPreview, error) {
	firebaseID, ok := ctx.Value(middleware.FirebaseIDKey).(string)
	if !ok || firebaseID == "" {
		return nil, authErrors.ErrorInternalServer
	}

	auth, err := s.readRepo.GetByFirebaseID(ctx, firebaseID)
	if err != nil {
		return nil, authErrors.ErrorUserNotFound
	}

	if !auth.CanLogin() {
		return nil, authErrors.ErrorInternalServer
	}

	// Récupération du profil depuis le user-service
	userPreview, err := s.userClient.GetUserByAuthID(ctx, auth.AuthID)
	if err != nil {
		return nil, authErrors.ErrorDataRetrievalFailed
	}

	// Enrichissement avec les données auth (email/phone appartiennent à auth-service)
	userPreview.Email = auth.Email
	userPreview.PhoneNumber = auth.PhoneNumber

	return userPreview, nil
}

// CheckEmail vérifie si un email est disponible (non utilisé)
func (s *authServiceImpl) CheckEmail(ctx context.Context, email string) (bool, error) {
	if email == "" {
		return false, authErrors.ErrorInternalServer
	}

	exists, err := s.readRepo.EmailExists(ctx, email)
	if err != nil {
		return false, err
	}

	return !exists, nil
}

// CheckPhoneNumber vérifie si un numéro de téléphone est disponible (non utilisé)
func (s *authServiceImpl) CheckPhoneNumber(ctx context.Context, phoneNumber string) (bool, error) {
	if phoneNumber == "" {
		return false, authErrors.ErrorInternalServer
	}

	exists, err := s.readRepo.PhoneNumberExists(ctx, phoneNumber)
	if err != nil {
		return false, err
	}

	return !exists, nil
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

// DeleteUserAccount anonymise et supprime le compte d'authentification d'un utilisateur
func (s *authServiceImpl) DeleteUserAccount(ctx context.Context, firebaseID string) error {
	if firebaseID == "" {
		return authErrors.ErrorUserNotFound
	}

	auth, err := s.readRepo.GetByFirebaseID(ctx, firebaseID)
	if err != nil {
		return authErrors.ErrorUserNotFound
	}

	return s.writeRepo.Delete(ctx, auth)
}
