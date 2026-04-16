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
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const serviceVersion = "1.0.0"

// Timeouts pour les handlers gRPC
const (
	defaultTimeout      = 5 * time.Second  // Opérations simples (lecture DB)
	crossServiceTimeout = 10 * time.Second // Opérations impliquant un appel inter-service
)

// AuthHandler implémente authpb.AuthServiceServer.
// Il traduit les requêtes proto en appels de service et mappe les erreurs domaine
// vers les codes gRPC appropriés.
type AuthHandler struct {
	authpb.UnimplementedAuthServiceServer
	service serviceInterfaces.AuthService
	logger  *zap.Logger
}

func NewAuthHandler(service serviceInterfaces.AuthService, logger *zap.Logger) *AuthHandler {
	return &AuthHandler{service: service, logger: logger}
}

// CreateAccount crée un nouveau compte utilisateur.
func (h *AuthHandler) CreateAccount(ctx context.Context, req *authpb.CreateAccountRequest) (*authpb.CreateAccountResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, crossServiceTimeout)
	defer cancel()

	h.logger.Debug("handler: CreateAccount called",
		zap.String("name", req.Name),
		zap.String("firstName", req.FirstName),
		zap.String("email", req.Email),
		zap.String("phone", req.PhoneNumber),
	)
	user, err := h.service.RegisterUser(ctx, req.Name, req.FirstName, req.Email, req.PhoneNumber, req.ProfileImageURL)
	if err != nil {
		h.logger.Error("handler: CreateAccount failed", zap.Error(err))
		return nil, toGRPCError(err)
	}
	h.logger.Info("handler: CreateAccount success", zap.String("authID", user.AuthID))
	return &authpb.CreateAccountResponse{User: toProtoUserPreview(user)}, nil
}

// CheckEmail vérifie si une adresse email est disponible (non utilisée).
func (h *AuthHandler) CheckEmail(ctx context.Context, req *authpb.CheckEmailRequest) (*authpb.CheckPhoneOrEmailResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	h.logger.Debug("handler: CheckEmail called", zap.String("email", req.Email))
	available, err := h.service.CheckEmail(ctx, req.Email)
	if err != nil {
		h.logger.Error("handler: CheckEmail failed", zap.Error(err))
		return nil, toGRPCError(err)
	}
	h.logger.Debug("handler: CheckEmail result", zap.Bool("available", available))
	return &authpb.CheckPhoneOrEmailResponse{IsAvailable: available}, nil
}

// CheckPhoneNumber vérifie si un numéro de téléphone est disponible.
func (h *AuthHandler) CheckPhoneNumber(ctx context.Context, req *authpb.CheckPhoneNumberRequest) (*authpb.CheckPhoneOrEmailResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	h.logger.Debug("handler: CheckPhoneNumber called", zap.String("phone", req.PhoneNumber))
	available, err := h.service.CheckPhoneNumber(ctx, req.PhoneNumber)
	if err != nil {
		h.logger.Error("handler: CheckPhoneNumber failed", zap.Error(err))
		return nil, toGRPCError(err)
	}
	h.logger.Debug("handler: CheckPhoneNumber result", zap.Bool("available", available))
	return &authpb.CheckPhoneOrEmailResponse{IsAvailable: available}, nil
}

// DeleteAccount supprime le compte de l'utilisateur authentifié.
// Le Firebase UID est extrait du contexte (injecté par l'intercepteur JWT).
func (h *AuthHandler) DeleteAccount(ctx context.Context, _ *authpb.DeleteAccountRequest) (*authpb.AuthServerResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, crossServiceTimeout)
	defer cancel()

	h.logger.Debug("handler: DeleteAccount called")
	firebaseID, ok := ctx.Value(middleware.FirebaseIDKey).(string)
	if !ok || firebaseID == "" {
		h.logger.Error("handler: DeleteAccount - missing firebase ID in context")
		return nil, status.Error(codes.Unauthenticated, "missing firebase ID in context")
	}
	if err := h.service.DeleteUserAccount(ctx, firebaseID); err != nil {
		h.logger.Error("handler: DeleteAccount failed", zap.Error(err))
		return nil, toGRPCError(err)
	}
	h.logger.Info("handler: DeleteAccount success", zap.String("firebaseID", firebaseID))
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

// GetAuthInfo retourne les données d'authentification d'un utilisateur (inter-service, pas de JWT).
func (h *AuthHandler) GetAuthInfo(ctx context.Context, req *authpb.GetAuthInfoRequest) (*authpb.GetAuthInfoResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	h.logger.Debug("handler: GetAuthInfo called", zap.String("authID", req.AuthID))
	auth, err := h.service.GetAuthInfo(ctx, req.AuthID)
	if err != nil {
		h.logger.Error("handler: GetAuthInfo failed", zap.Error(err), zap.String("authID", req.AuthID))
		return nil, toGRPCError(err)
	}

	resp := &authpb.GetAuthInfoResponse{
		AuthID:      auth.AuthID,
		IsActive:    auth.IsActive,
		IsSuspended: auth.IsSuspended,
	}

	if auth.Email != nil {
		resp.Email = *auth.Email
	}
	if auth.PhoneNumber != nil {
		resp.PhoneNumber = *auth.PhoneNumber
	}
	if auth.SuspensionEndDate != nil {
		resp.SuspensionEndDate = auth.SuspensionEndDate.Format(time.RFC3339)
	}

	h.logger.Debug("handler: GetAuthInfo success", zap.String("authID", req.AuthID))
	return resp, nil
}

// toGRPCError traduit les erreurs domaine en codes de statut gRPC.
func toGRPCError(err error) error {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "request timeout")
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, "request canceled")
	case errors.Is(err, domain.ErrEmailInvalidFormat),
		errors.Is(err, domain.ErrEmailTooLong),
		errors.Is(err, domain.ErrPhoneInvalidFormat):
		return status.Error(codes.InvalidArgument, err.Error())
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
