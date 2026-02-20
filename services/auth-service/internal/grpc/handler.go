package grpc

import (
	"context"
	"errors"
	"time"

	"github.com/Kpeewu/tissi-mah/services/auth-service/internal/domain"
	"github.com/Kpeewu/tissi-mah/services/auth-service/internal/middleware"
	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/auth-service/internal/service/interfaces"
	authErrors "github.com/Kpeewu/tissi-mah/services/auth-service/pkg/errors"
	authpb "github.com/Kpeewu/tissi-mah/services/auth-service/proto/gen"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const serviceVersion = "1.0.0"

// AuthHandler implémente authpb.AuthServiceServer.
// Il traduit les requêtes proto en appels de service et mappe les erreurs domaine
// vers les codes gRPC appropriés.
type AuthHandler struct {
	authpb.UnimplementedAuthServiceServer
	service serviceInterfaces.AuthService
}

func NewAuthHandler(service serviceInterfaces.AuthService) *AuthHandler {
	return &AuthHandler{service: service}
}

// Login vérifie si l'utilisateur authentifié possède déjà un compte.
// Retourne {Exists: false} (sans erreur gRPC) si aucun compte n'est trouvé,
// afin que le client redirige vers l'inscription.
func (h *AuthHandler) Login(ctx context.Context, _ *authpb.LoginRequest) (*authpb.LoginResponse, error) {
	user, err := h.service.LoginUser(ctx)
	if err != nil {
		if errors.Is(err, authErrors.ErrorUserNotFound) {
			return &authpb.LoginResponse{Exists: false}, nil
		}
		return nil, toGRPCError(err)
	}
	return &authpb.LoginResponse{
		Exists: true,
		User:   toProtoUserPreview(user),
	}, nil
}

// CreateAccount crée un nouveau compte utilisateur.
func (h *AuthHandler) CreateAccount(ctx context.Context, req *authpb.CreateAccountRequest) (*authpb.CreateAccountResponse, error) {
	user, err := h.service.RegisterUser(ctx, req.Name, req.FirstName, req.Email, req.PhoneNumber, req.ProfileImageURL)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &authpb.CreateAccountResponse{User: toProtoUserPreview(user)}, nil
}

// CheckEmail vérifie si une adresse email est disponible (non utilisée).
func (h *AuthHandler) CheckEmail(ctx context.Context, req *authpb.CheckEmailRequest) (*authpb.CheckPhoneOrEmailResponse, error) {
	available, err := h.service.CheckEmail(ctx, req.Email)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &authpb.CheckPhoneOrEmailResponse{IsAvailable: available}, nil
}

// CheckPhoneNumber vérifie si un numéro de téléphone est disponible.
func (h *AuthHandler) CheckPhoneNumber(ctx context.Context, req *authpb.CheckPhoneNumberRequest) (*authpb.CheckPhoneOrEmailResponse, error) {
	available, err := h.service.CheckPhoneNumber(ctx, req.PhoneNumber)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &authpb.CheckPhoneOrEmailResponse{IsAvailable: available}, nil
}

// DeleteAccount supprime le compte de l'utilisateur authentifié.
// Le Firebase UID est extrait du contexte (injecté par l'intercepteur JWT).
func (h *AuthHandler) DeleteAccount(ctx context.Context, _ *authpb.DeleteAccountRequest) (*authpb.AuthServerResponse, error) {
	firebaseID, ok := ctx.Value(middleware.FirebaseIDKey).(string)
	if !ok || firebaseID == "" {
		return nil, status.Error(codes.Unauthenticated, "missing firebase ID in context")
	}
	if err := h.service.DeleteUserAccount(ctx, firebaseID); err != nil {
		return nil, toGRPCError(err)
	}
	return &authpb.AuthServerResponse{Success: true}, nil
}

// Health retourne l'état de santé du service (route publique, sans auth).
func (h *AuthHandler) Health(_ context.Context, _ *authpb.HealthRequest) (*authpb.HealthResponse, error) {
	return &authpb.HealthResponse{
		Status:    "healthy",
		Version:   serviceVersion,
		Timestamp: time.Now().Unix(),
	}, nil
}

// toGRPCError traduit les erreurs domaine en codes de statut gRPC.
func toGRPCError(err error) error {
	switch {
	case errors.Is(err, authErrors.ErrorUserNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, authErrors.ErrorEmailNotAvailable):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, authErrors.ErrorPhoneNumberNotAvailable):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, authErrors.ErrorCantDeleteAccount):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, authErrors.ErrorDataRetrievalFailed),
		errors.Is(err, authErrors.ErrorInternalServer):
		return status.Error(codes.Internal, err.Error())
	default:
		return status.Error(codes.Internal, "internal server error")
	}
}

// toProtoUserPreview convertit domain.UserPreview en message proto UserPreview.
func toProtoUserPreview(u *domain.UserPreview) *authpb.UserPreview {
	preview := &authpb.UserPreview{
		AuthID:    u.AuthID,
		UserID:    u.UserID,
		Name:      u.Name,
		FirstName: u.FirstName,
	}
	if u.Email != nil {
		preview.Email = *u.Email
	}
	if u.PhoneNumber != nil {
		preview.PhoneNumber = *u.PhoneNumber
	}
	if u.ProfilePhotoURL != nil {
		preview.ProfileImageURL = *u.ProfilePhotoURL
	}
	return preview
}
