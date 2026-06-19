package grpcsrv

import (
	"context"
	"errors"

	"github.com/Kpeewu/tissi-mah/services/support-service/internal/domain"
	"github.com/Kpeewu/tissi-mah/services/support-service/internal/middleware"
	svcIfaces "github.com/Kpeewu/tissi-mah/services/support-service/internal/service/interfaces"
	supportErrors "github.com/Kpeewu/tissi-mah/services/support-service/pkg/errors"
	supportpb "github.com/Kpeewu/tissi-mah/services/support-service/proto/gen"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// SupportHandler implémente le serveur gRPC support.SupportService.
type SupportHandler struct {
	supportpb.UnimplementedSupportServiceServer
	svc    svcIfaces.SupportService
	logger *zap.Logger
}

func NewSupportHandler(svc svcIfaces.SupportService, logger *zap.Logger) *SupportHandler {
	return &SupportHandler{svc: svc, logger: logger}
}

func toGRPCError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, supportErrors.ErrInvalidInput):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, supportErrors.ErrUnauthenticated):
		return status.Error(codes.Unauthenticated, err.Error())
	case errors.Is(err, supportErrors.ErrInvalidCredentials):
		return status.Error(codes.Unauthenticated, err.Error())
	case errors.Is(err, supportErrors.ErrAccountLocked):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, supportErrors.ErrAccountInactive):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, supportErrors.ErrForbidden):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, supportErrors.ErrUserNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, supportErrors.ErrEmailAlreadyExists):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, supportErrors.ErrOTPExpired),
		errors.Is(err, supportErrors.ErrOTPSessionNotFound),
		errors.Is(err, supportErrors.ErrOTPInvalid),
		errors.Is(err, supportErrors.ErrOTPTooManyAttempts):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, supportErrors.ErrResendCooldown):
		return status.Error(codes.ResourceExhausted, err.Error())
	case errors.Is(err, supportErrors.ErrRefreshInvalid),
		errors.Is(err, supportErrors.ErrRefreshRevoked):
		return status.Error(codes.Unauthenticated, err.Error())
	case errors.Is(err, supportErrors.ErrWeakPassword):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, supportErrors.ErrEmailChangeCooldown):
		return status.Error(codes.FailedPrecondition, err.Error())
	default:
		return status.Error(codes.Internal, "internal server error")
	}
}

// requireAdmin garde les routes /admin/* aux comptes de rôle admin.
func requireAdmin(ctx context.Context) error {
	role, ok := middleware.RoleFromContext(ctx)
	if !ok || role != domain.RoleAdmin {
		return supportErrors.ErrForbidden
	}
	return nil
}

// gateMustChange empêche tout appel autre que ChangeMyPassword/Logout tant que mustChangePassword=true.
// (Implémenté en interrogeant le repo via le service Me.)
func (h *SupportHandler) gateMustChange(ctx context.Context, allowed bool) error {
	if allowed {
		return nil
	}
	uid, ok := middleware.UIDFromContext(ctx)
	if !ok {
		return supportErrors.ErrUnauthenticated
	}
	user, err := h.svc.Me(ctx, uid)
	if err != nil {
		return err
	}
	if user.MustChangePassword {
		return supportErrors.ErrMustChangePassword
	}
	return nil
}

func (h *SupportHandler) Login(ctx context.Context, req *supportpb.LoginRequest) (*supportpb.LoginResponse, error) {
	h.logger.Debug("handler: Login called", zap.String("email", req.GetEmail()))
	res, err := h.svc.Login(ctx, req.GetEmail(), req.GetPassword())
	if err != nil {
		h.logger.Error("handler: Login failed", zap.String("email", req.GetEmail()), zap.Error(err))
		return nil, toGRPCError(err)
	}
	h.logger.Info("handler: Login success - OTP session started", zap.String("email", req.GetEmail()))
	return &supportpb.LoginResponse{
		OtpSessionId:     res.OTPSessionID,
		ExpiresInSeconds: int32(res.ExpiresInSeconds),
	}, nil
}

func (h *SupportHandler) VerifyOTP(ctx context.Context, req *supportpb.VerifyOTPRequest) (*supportpb.VerifyOTPResponse, error) {
	h.logger.Debug("handler: VerifyOTP called", zap.String("sessionID", req.GetOtpSessionId()))
	res, err := h.svc.VerifyOTP(ctx, req.GetOtpSessionId(), req.GetCode())
	if err != nil {
		h.logger.Error("handler: VerifyOTP failed", zap.String("sessionID", req.GetOtpSessionId()), zap.Error(err))
		return nil, toGRPCError(err)
	}
	h.logger.Info("handler: VerifyOTP success", zap.String("sessionID", req.GetOtpSessionId()))
	return &supportpb.VerifyOTPResponse{
		AccessToken:        res.AccessToken,
		AccessExpiresAt:    res.AccessExpiresAt,
		RefreshToken:       res.RefreshToken,
		RefreshExpiresAt:   res.RefreshExpiresAt,
		Role:               res.Role,
		MustChangePassword: res.MustChangePassword,
	}, nil
}

func (h *SupportHandler) ResendOTP(ctx context.Context, req *supportpb.ResendOTPRequest) (*supportpb.ResendOTPResponse, error) {
	h.logger.Debug("handler: ResendOTP called", zap.String("sessionID", req.GetOtpSessionId()))
	res, err := h.svc.ResendOTP(ctx, req.GetOtpSessionId())
	if err != nil {
		h.logger.Error("handler: ResendOTP failed", zap.String("sessionID", req.GetOtpSessionId()), zap.Error(err))
		return nil, toGRPCError(err)
	}
	h.logger.Debug("handler: ResendOTP success", zap.String("sessionID", req.GetOtpSessionId()))
	return &supportpb.ResendOTPResponse{
		OtpSessionId:     res.OTPSessionID,
		ExpiresInSeconds: int32(res.ExpiresInSeconds),
	}, nil
}

func (h *SupportHandler) RefreshToken(ctx context.Context, req *supportpb.RefreshTokenRequest) (*supportpb.RefreshTokenResponse, error) {
	h.logger.Debug("handler: RefreshToken called")
	res, err := h.svc.RefreshToken(ctx, req.GetRefreshToken())
	if err != nil {
		h.logger.Error("handler: RefreshToken failed", zap.Error(err))
		return nil, toGRPCError(err)
	}
	h.logger.Debug("handler: RefreshToken success")
	return &supportpb.RefreshTokenResponse{
		AccessToken:      res.AccessToken,
		AccessExpiresAt:  res.AccessExpiresAt,
		RefreshToken:     res.RefreshToken,
		RefreshExpiresAt: res.RefreshExpiresAt,
	}, nil
}

func (h *SupportHandler) Logout(ctx context.Context, req *supportpb.LogoutRequest) (*supportpb.LogoutResponse, error) {
	h.logger.Debug("handler: Logout called")
	if err := h.svc.Logout(ctx, req.GetRefreshToken()); err != nil {
		h.logger.Error("handler: Logout failed", zap.Error(err))
		return nil, toGRPCError(err)
	}
	h.logger.Debug("handler: Logout success")
	return &supportpb.LogoutResponse{}, nil
}

func (h *SupportHandler) Me(ctx context.Context, _ *supportpb.MeRequest) (*supportpb.MeResponse, error) {
	uid, ok := middleware.UIDFromContext(ctx)
	if !ok {
		h.logger.Error("handler: Me - missing support UID in context")
		return nil, toGRPCError(supportErrors.ErrUnauthenticated)
	}
	h.logger.Debug("handler: Me called", zap.String("uid", uid))
	u, err := h.svc.Me(ctx, uid)
	if err != nil {
		h.logger.Error("handler: Me failed", zap.String("uid", uid), zap.Error(err))
		return nil, toGRPCError(err)
	}
	return &supportpb.MeResponse{
		UserId:             u.UserID,
		Email:              u.Email,
		FirstName:          u.FirstName,
		LastName:           u.LastName,
		Role:               u.Role,
		MustChangePassword: u.MustChangePassword,
		CreatedAt:          u.CreatedAt.Unix(),
	}, nil
}

func (h *SupportHandler) ChangeMyPassword(ctx context.Context, req *supportpb.ChangeMyPasswordRequest) (*supportpb.ChangeMyPasswordResponse, error) {
	uid, ok := middleware.UIDFromContext(ctx)
	if !ok {
		h.logger.Error("handler: ChangeMyPassword - missing support UID in context")
		return nil, toGRPCError(supportErrors.ErrUnauthenticated)
	}
	h.logger.Debug("handler: ChangeMyPassword called", zap.String("uid", uid))
	if err := h.svc.ChangeMyPassword(ctx, uid, req.GetCurrentPassword(), req.GetNewPassword()); err != nil {
		h.logger.Error("handler: ChangeMyPassword failed", zap.String("uid", uid), zap.Error(err))
		return nil, toGRPCError(err)
	}
	h.logger.Info("handler: ChangeMyPassword success", zap.String("uid", uid))
	return &supportpb.ChangeMyPasswordResponse{}, nil
}

func (h *SupportHandler) ChangeMyEmail(ctx context.Context, req *supportpb.ChangeMyEmailRequest) (*supportpb.ChangeMyEmailResponse, error) {
	uid, ok := middleware.UIDFromContext(ctx)
	if !ok {
		h.logger.Error("handler: ChangeMyEmail - missing support UID in context")
		return nil, toGRPCError(supportErrors.ErrUnauthenticated)
	}
	h.logger.Debug("handler: ChangeMyEmail called", zap.String("uid", uid))
	if err := h.gateMustChange(ctx, false); err != nil {
		return nil, toGRPCError(err)
	}
	if err := h.svc.ChangeMyEmail(ctx, uid, req.GetNewEmail(), req.GetCurrentPassword()); err != nil {
		h.logger.Error("handler: ChangeMyEmail failed", zap.String("uid", uid), zap.Error(err))
		return nil, toGRPCError(err)
	}
	h.logger.Info("handler: ChangeMyEmail success", zap.String("uid", uid))
	return &supportpb.ChangeMyEmailResponse{}, nil
}

func (h *SupportHandler) CreateSupportAgent(ctx context.Context, req *supportpb.CreateSupportAgentRequest) (*supportpb.CreateSupportAgentResponse, error) {
	if err := requireAdmin(ctx); err != nil {
		h.logger.Warn("handler: CreateSupportAgent - insufficient role", zap.Error(err))
		return nil, toGRPCError(err)
	}
	if err := h.gateMustChange(ctx, false); err != nil {
		return nil, toGRPCError(err)
	}
	h.logger.Debug("handler: CreateSupportAgent called", zap.String("email", req.GetEmail()))
	id, err := h.svc.CreateSupportAgent(ctx, req.GetEmail(), req.GetFirstName(), req.GetLastName())
	if err != nil {
		h.logger.Error("handler: CreateSupportAgent failed", zap.String("email", req.GetEmail()), zap.Error(err))
		return nil, toGRPCError(err)
	}
	h.logger.Info("handler: CreateSupportAgent success", zap.String("email", req.GetEmail()), zap.String("userID", id))
	return &supportpb.CreateSupportAgentResponse{UserId: id}, nil
}

func (h *SupportHandler) ListSupportAgents(ctx context.Context, req *supportpb.ListSupportAgentsRequest) (*supportpb.ListSupportAgentsResponse, error) {
	if err := requireAdmin(ctx); err != nil {
		h.logger.Warn("handler: ListSupportAgents - insufficient role", zap.Error(err))
		return nil, toGRPCError(err)
	}
	if err := h.gateMustChange(ctx, false); err != nil {
		return nil, toGRPCError(err)
	}
	h.logger.Debug("handler: ListSupportAgents called", zap.Int32("limit", req.GetLimit()), zap.Int32("offset", req.GetOffset()))
	users, total, err := h.svc.ListSupportAgents(ctx, int(req.GetLimit()), int(req.GetOffset()))
	if err != nil {
		h.logger.Error("handler: ListSupportAgents failed", zap.Error(err))
		return nil, toGRPCError(err)
	}
	out := make([]*supportpb.SupportAgent, 0, len(users))
	for _, u := range users {
		out = append(out, &supportpb.SupportAgent{
			UserId:    u.UserID,
			Email:     u.Email,
			FirstName: u.FirstName,
			LastName:  u.LastName,
			Role:      u.Role,
			IsActive:  u.IsActive,
			CreatedAt: u.CreatedAt.Unix(),
		})
	}
	h.logger.Debug("handler: ListSupportAgents success", zap.Int("count", len(out)), zap.Int32("total", int32(total)))
	return &supportpb.ListSupportAgentsResponse{Agents: out, Total: int32(total)}, nil
}

func (h *SupportHandler) DeactivateSupportAgent(ctx context.Context, req *supportpb.DeactivateSupportAgentRequest) (*supportpb.DeactivateSupportAgentResponse, error) {
	if err := requireAdmin(ctx); err != nil {
		h.logger.Warn("handler: DeactivateSupportAgent - insufficient role", zap.Error(err))
		return nil, toGRPCError(err)
	}
	if err := h.gateMustChange(ctx, false); err != nil {
		return nil, toGRPCError(err)
	}
	h.logger.Debug("handler: DeactivateSupportAgent called", zap.String("userID", req.GetUserId()))
	if err := h.svc.DeactivateSupportAgent(ctx, req.GetUserId()); err != nil {
		h.logger.Error("handler: DeactivateSupportAgent failed", zap.String("userID", req.GetUserId()), zap.Error(err))
		return nil, toGRPCError(err)
	}
	h.logger.Info("handler: DeactivateSupportAgent success", zap.String("userID", req.GetUserId()))
	return &supportpb.DeactivateSupportAgentResponse{}, nil
}

func (h *SupportHandler) Health(_ context.Context, _ *supportpb.HealthRequest) (*supportpb.HealthResponse, error) {
	return &supportpb.HealthResponse{
		Status:  "ok",
		Version: "1.0.0",
		Service: "support-service",
	}, nil
}

// GetSupportUserByID est un RPC inter-service appelé par payment-service pour vérifier
// l'identité et l'habilitation d'un agent support avant d'exécuter un payout manuel.
func (h *SupportHandler) GetSupportUserByID(ctx context.Context, req *supportpb.GetSupportUserByIDRequest) (*supportpb.GetSupportUserByIDResponse, error) {
	h.logger.Debug("handler: GetSupportUserByID called", zap.String("userID", req.GetUserId()))
	u, err := h.svc.Me(ctx, req.GetUserId())
	if err != nil {
		h.logger.Error("handler: GetSupportUserByID failed", zap.String("userID", req.GetUserId()), zap.Error(err))
		return nil, toGRPCError(err)
	}
	h.logger.Debug("handler: GetSupportUserByID success", zap.String("userID", req.GetUserId()))
	return &supportpb.GetSupportUserByIDResponse{
		UserId:    u.UserID,
		FirstName: u.FirstName,
		LastName:  u.LastName,
		Role:      u.Role,
		IsActive:  u.IsActive,
	}, nil
}
