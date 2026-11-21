package service

import (
	"context"
	"strings"
	"time"

	"github.com/Kpeewu/tissi-mah/services/support-service/internal/config"
	"github.com/Kpeewu/tissi-mah/services/support-service/internal/domain"
	"github.com/Kpeewu/tissi-mah/services/support-service/internal/otp"
	"github.com/Kpeewu/tissi-mah/services/support-service/internal/password"
	repoIfaces "github.com/Kpeewu/tissi-mah/services/support-service/internal/repository/interfaces"
	svcIfaces "github.com/Kpeewu/tissi-mah/services/support-service/internal/service/interfaces"
	"github.com/Kpeewu/tissi-mah/services/support-service/internal/token"
	supportErrors "github.com/Kpeewu/tissi-mah/services/support-service/pkg/errors"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// EmailSender abstrait l'envoi d'email transactionnel (satisfait par *client.EmailClient).
// Permet d'injecter un mock en tests sans dépendre d'un gRPC client.
type EmailSender interface {
	SendEmail(ctx context.Context, to, subject, bodyText, bodyHTML string) error
}

type supportServiceImpl struct {
	cfg          *config.Config
	readRepo     repoIfaces.SupportUserReadRepository
	writeRepo    repoIfaces.SupportUserWriteRepository
	otpStore     *otp.Store
	jwtSigner    *token.JWTSigner
	refreshStore *token.RefreshStore
	resetStore   *token.ResetStore
	emailClient  EmailSender
	logger       *zap.Logger
}

// NewSupportService instancie l'implémentation du service.
func NewSupportService(
	cfg *config.Config,
	readRepo repoIfaces.SupportUserReadRepository,
	writeRepo repoIfaces.SupportUserWriteRepository,
	otpStore *otp.Store,
	jwtSigner *token.JWTSigner,
	refreshStore *token.RefreshStore,
	resetStore *token.ResetStore,
	emailClient EmailSender,
	logger *zap.Logger,
) svcIfaces.SupportService {
	return &supportServiceImpl{
		cfg:          cfg,
		readRepo:     readRepo,
		writeRepo:    writeRepo,
		otpStore:     otpStore,
		jwtSigner:    jwtSigner,
		refreshStore: refreshStore,
		resetStore:   resetStore,
		emailClient:  emailClient,
		logger:       logger,
	}
}

func normalizeEmail(e string) string {
	return strings.ToLower(strings.TrimSpace(e))
}

// ─── Login ───────────────────────────────────────────────────────────────────

func (s *supportServiceImpl) Login(ctx context.Context, email, plain string) (*svcIfaces.LoginResult, error) {
	email = normalizeEmail(email)
	if email == "" || plain == "" {
		return nil, supportErrors.ErrInvalidInput
	}

	locked, err := s.otpStore.IsLocked(ctx, email, s.cfg.RateLimit.FailThreshold)
	if err != nil {
		s.logger.Error("login: redis lock check failed", zap.String("email", email), zap.Error(err))
		return nil, supportErrors.ErrInternal
	}
	if locked {
		s.logger.Warn("login: account locked", zap.String("email", email))
		return nil, supportErrors.ErrAccountLocked
	}

	user, err := s.readRepo.GetByEmail(ctx, email)
	if err != nil {
		// Anti-énumération : on incrémente fail puis on renvoie ErrInvalidCredentials.
		_, _ = s.otpStore.IncrFail(ctx, email)
		return nil, supportErrors.ErrInvalidCredentials
	}
	if !user.IsActive {
		_, _ = s.otpStore.IncrFail(ctx, email)
		return nil, supportErrors.ErrInvalidCredentials
	}

	ok, err := password.Verify(user.PasswordHash, plain)
	if err != nil || !ok {
		s.logger.Warn("login: invalid credentials", zap.String("email", email))
		_, _ = s.otpStore.IncrFail(ctx, email)
		return nil, supportErrors.ErrInvalidCredentials
	}

	if err := s.otpStore.MarkResendCooldown(ctx, email); err != nil {
		return nil, err
	}

	return s.startOTPSession(ctx, user)
}

func (s *supportServiceImpl) startOTPSession(ctx context.Context, user *domain.SupportUser) (*svcIfaces.LoginResult, error) {
	code, err := otp.Generate()
	if err != nil {
		s.logger.Error("startOTPSession: OTP generation failed", zap.String("userID", user.UserID), zap.Error(err))
		return nil, supportErrors.ErrInternal
	}
	codeHash, err := password.Hash(code)
	if err != nil {
		s.logger.Error("startOTPSession: OTP hash failed", zap.String("userID", user.UserID), zap.Error(err))
		return nil, supportErrors.ErrInternal
	}
	sessionID := uuid.NewString()
	sess := &otp.Session{
		Email:     user.Email,
		UserID:    user.UserID,
		Role:      user.Role,
		CodeHash:  codeHash,
		Attempts:  0,
		CreatedAt: time.Now().Unix(),
	}
	if err := s.otpStore.SaveSession(ctx, sessionID, sess); err != nil {
		s.logger.Error("startOTPSession: save session failed", zap.String("userID", user.UserID), zap.Error(err))
		return nil, supportErrors.ErrInternal
	}

	go func(to, code string) {
		bgCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := s.emailClient.SendEmail(bgCtx, to,
			"Votre code de connexion TissiMah Support",
			"Votre code de connexion est : "+code+"\nIl expire dans 5 minutes.",
			"<p>Votre code de connexion est : <strong>"+code+"</strong></p><p>Il expire dans 5 minutes.</p>",
		); err != nil {
			s.logger.Error("OTP email send failed", zap.Error(err))
		}
	}(user.Email, code)

	s.logger.Info("OTP session started", zap.String("userID", user.UserID))
	return &svcIfaces.LoginResult{
		OTPSessionID:     sessionID,
		ExpiresInSeconds: s.cfg.OTP.TTLSeconds,
	}, nil
}

// ─── VerifyOTP ───────────────────────────────────────────────────────────────

func (s *supportServiceImpl) VerifyOTP(ctx context.Context, sessionID, code string) (*svcIfaces.VerifyOTPResult, error) {
	if sessionID == "" || code == "" {
		return nil, supportErrors.ErrInvalidInput
	}
	sess, err := s.otpStore.GetSession(ctx, sessionID)
	if err != nil {
		return nil, supportErrors.ErrOTPExpired
	}

	locked, _ := s.otpStore.IsLocked(ctx, sess.Email, s.cfg.RateLimit.FailThreshold)
	if locked {
		s.logger.Warn("verifyOTP: account locked", zap.String("email", sess.Email))
		return nil, supportErrors.ErrAccountLocked
	}

	if sess.Attempts >= s.cfg.OTP.MaxAttempts {
		s.logger.Warn("verifyOTP: max attempts reached", zap.String("email", sess.Email), zap.Int("attempts", sess.Attempts))
		_ = s.otpStore.DeleteSession(ctx, sessionID)
		_, _ = s.otpStore.IncrFail(ctx, sess.Email)
		return nil, supportErrors.ErrOTPTooManyAttempts
	}

	ok, err := password.Verify(sess.CodeHash, code)
	if err != nil || !ok {
		s.logger.Warn("verifyOTP: invalid code", zap.String("email", sess.Email), zap.Int("attempt", sess.Attempts+1))
		_ = s.otpStore.IncrSessionAttempts(ctx, sessionID, sess)
		_, _ = s.otpStore.IncrFail(ctx, sess.Email)
		return nil, supportErrors.ErrOTPInvalid
	}

	user, err := s.readRepo.GetByID(ctx, sess.UserID)
	if err != nil || !user.IsActive {
		return nil, supportErrors.ErrAccountInactive
	}

	_ = s.otpStore.DeleteSession(ctx, sessionID)
	_ = s.otpStore.ResetFail(ctx, sess.Email)
	_ = s.otpStore.ClearResendCooldown(ctx, sess.Email)

	access, accessExp, err := s.jwtSigner.Sign(user.UserID, user.Role, user.MustChangePassword)
	if err != nil {
		s.logger.Error("verifyOTP: JWT signing failed", zap.String("userID", user.UserID), zap.Error(err))
		return nil, supportErrors.ErrInternal
	}
	refresh, _, refreshExp, err := s.refreshStore.Issue(ctx, user.UserID, user.Role, "")
	if err != nil {
		s.logger.Error("verifyOTP: refresh token issue failed", zap.String("userID", user.UserID), zap.Error(err))
		return nil, supportErrors.ErrInternal
	}

	s.logger.Info("verifyOTP: authentication success", zap.String("userID", user.UserID))
	return &svcIfaces.VerifyOTPResult{
		AccessToken:        access,
		AccessExpiresAt:    accessExp,
		RefreshToken:       refresh,
		RefreshExpiresAt:   refreshExp,
		Role:               user.Role,
		MustChangePassword: user.MustChangePassword,
	}, nil
}

// ─── ResendOTP ───────────────────────────────────────────────────────────────

func (s *supportServiceImpl) ResendOTP(ctx context.Context, sessionID string) (*svcIfaces.LoginResult, error) {
	if sessionID == "" {
		return nil, supportErrors.ErrInvalidInput
	}
	sess, err := s.otpStore.GetSession(ctx, sessionID)
	if err != nil {
		return nil, supportErrors.ErrOTPSessionNotFound
	}
	locked, _ := s.otpStore.IsLocked(ctx, sess.Email, s.cfg.RateLimit.FailThreshold)
	if locked {
		s.logger.Warn("resendOTP: account locked", zap.String("email", sess.Email))
		return nil, supportErrors.ErrAccountLocked
	}
	if err := s.otpStore.MarkResendCooldown(ctx, sess.Email); err != nil {
		return nil, err
	}

	user, err := s.readRepo.GetByID(ctx, sess.UserID)
	if err != nil || !user.IsActive {
		return nil, supportErrors.ErrAccountInactive
	}

	code, err := otp.Generate()
	if err != nil {
		s.logger.Error("resendOTP: OTP generation failed", zap.String("userID", sess.UserID), zap.Error(err))
		return nil, supportErrors.ErrInternal
	}
	codeHash, err := password.Hash(code)
	if err != nil {
		s.logger.Error("resendOTP: OTP hash failed", zap.String("userID", sess.UserID), zap.Error(err))
		return nil, supportErrors.ErrInternal
	}
	newSess := &otp.Session{
		Email:     sess.Email,
		UserID:    sess.UserID,
		Role:      sess.Role,
		CodeHash:  codeHash,
		Attempts:  0,
		CreatedAt: time.Now().Unix(),
	}
	if err := s.otpStore.SaveSession(ctx, sessionID, newSess); err != nil {
		s.logger.Error("resendOTP: save session failed", zap.String("userID", sess.UserID), zap.Error(err))
		return nil, supportErrors.ErrInternal
	}

	go func(to, code string) {
		bgCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := s.emailClient.SendEmail(bgCtx, to,
			"Votre nouveau code de connexion TissiMah Support",
			"Votre nouveau code est : "+code+"\nIl expire dans 5 minutes.",
			"<p>Votre nouveau code est : <strong>"+code+"</strong></p>",
		); err != nil {
			s.logger.Error("resendOTP: email send failed", zap.Error(err))
		}
	}(sess.Email, code)

	s.logger.Debug("resendOTP: new OTP session saved", zap.String("userID", sess.UserID))
	return &svcIfaces.LoginResult{
		OTPSessionID:     sessionID,
		ExpiresInSeconds: s.cfg.OTP.TTLSeconds,
	}, nil
}

// ─── RefreshToken ────────────────────────────────────────────────────────────

func (s *supportServiceImpl) RefreshToken(ctx context.Context, raw string) (*svcIfaces.RefreshResult, error) {
	if raw == "" {
		return nil, supportErrors.ErrInvalidInput
	}
	rec, err := s.refreshStore.Verify(ctx, raw)
	if err != nil {
		return nil, err
	}
	user, err := s.readRepo.GetByID(ctx, rec.UserID)
	if err != nil || !user.IsActive {
		_ = s.refreshStore.RevokeFamily(ctx, rec.FamilyID)
		s.logger.Warn("refreshToken: account inactive or not found, family revoked", zap.String("userID", rec.UserID))
		return nil, supportErrors.ErrAccountInactive
	}
	newRefresh, refreshExp, err := s.refreshStore.Rotate(ctx, raw, rec)
	if err != nil {
		s.logger.Error("refreshToken: token rotation failed", zap.String("userID", rec.UserID), zap.Error(err))
		return nil, supportErrors.ErrInternal
	}
	access, accessExp, err := s.jwtSigner.Sign(user.UserID, user.Role, user.MustChangePassword)
	if err != nil {
		s.logger.Error("refreshToken: JWT signing failed", zap.String("userID", user.UserID), zap.Error(err))
		return nil, supportErrors.ErrInternal
	}
	return &svcIfaces.RefreshResult{
		AccessToken:      access,
		AccessExpiresAt:  accessExp,
		RefreshToken:     newRefresh,
		RefreshExpiresAt: refreshExp,
	}, nil
}

// ─── Logout ──────────────────────────────────────────────────────────────────

func (s *supportServiceImpl) Logout(ctx context.Context, raw string) error {
	if raw == "" {
		return nil
	}
	return s.refreshStore.Revoke(ctx, raw)
}

// ─── Me ──────────────────────────────────────────────────────────────────────

func (s *supportServiceImpl) Me(ctx context.Context, userID string) (*domain.SupportUser, error) {
	if userID == "" {
		return nil, supportErrors.ErrUnauthenticated
	}
	return s.readRepo.GetByID(ctx, userID)
}

// ─── ChangeMyPassword ────────────────────────────────────────────────────────

func (s *supportServiceImpl) ChangeMyPassword(ctx context.Context, userID, current, newPwd string) error {
	user, err := s.readRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	ok, err := password.Verify(user.PasswordHash, current)
	if err != nil || !ok {
		return supportErrors.ErrInvalidCredentials
	}
	if err := password.ValidateStrength(newPwd); err != nil {
		return supportErrors.ErrWeakPassword
	}
	hash, err := password.Hash(newPwd)
	if err != nil {
		s.logger.Error("changeMyPassword: hash failed", zap.String("userID", userID), zap.Error(err))
		return supportErrors.ErrInternal
	}
	if err := s.writeRepo.UpdatePassword(ctx, userID, hash, false); err != nil {
		return err
	}
	s.logger.Info("changeMyPassword: success", zap.String("userID", userID))
	return nil
}

// ─── UpdateMyProfile ─────────────────────────────────────────────────────────

// UpdateMyProfile modifie le nom et le prénom de l'utilisateur courant.
func (s *supportServiceImpl) UpdateMyProfile(ctx context.Context, userID, firstName, lastName string) error {
	firstName = strings.TrimSpace(firstName)
	lastName = strings.TrimSpace(lastName)
	if firstName == "" || lastName == "" {
		return supportErrors.ErrInvalidInput
	}
	if err := s.writeRepo.UpdateName(ctx, userID, firstName, lastName); err != nil {
		return err
	}
	s.logger.Info("updateMyProfile: success", zap.String("userID", userID))
	return nil
}

// ─── CreateSupportAgent ──────────────────────────────────────────────────────

func (s *supportServiceImpl) CreateSupportAgent(ctx context.Context, email, firstName, lastName, role string) (string, error) {
	email = normalizeEmail(email)
	if email == "" || firstName == "" || lastName == "" {
		return "", supportErrors.ErrInvalidInput
	}
	// Le rôle est obligatoire et doit être un rôle valide (admin/support).
	if !domain.IsValidRole(role) {
		return "", supportErrors.ErrInvalidInput
	}
	exists, err := s.readRepo.ExistsByEmail(ctx, email)
	if err != nil {
		return "", err
	}
	if exists {
		return "", supportErrors.ErrEmailAlreadyExists
	}
	temp, err := password.GenerateTemporary()
	if err != nil {
		s.logger.Error("createSupportAgent: temp password generation failed", zap.String("email", email), zap.Error(err))
		return "", supportErrors.ErrInternal
	}
	hash, err := password.Hash(temp)
	if err != nil {
		s.logger.Error("createSupportAgent: password hash failed", zap.String("email", email), zap.Error(err))
		return "", supportErrors.ErrInternal
	}
	user := &domain.SupportUser{
		UserID:             uuid.NewString(),
		Email:              email,
		PasswordHash:       hash,
		FirstName:          firstName,
		LastName:           lastName,
		Role:               role,
		IsActive:           true,
		MustChangePassword: true,
	}
	if err := s.writeRepo.Create(ctx, user); err != nil {
		s.logger.Error("createSupportAgent: DB create failed", zap.String("email", email), zap.Error(err))
		return "", err
	}

	go func(to, temp string) {
		bgCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := s.emailClient.SendEmail(bgCtx, to,
			"Votre compte support TissiMah",
			"Votre mot de passe provisoire : "+temp+"\nVous devrez le changer à la première connexion.",
			"<p>Votre mot de passe provisoire : <strong>"+temp+"</strong></p><p>Vous devrez le changer à la première connexion.</p>",
		); err != nil {
			s.logger.Error("createSupportAgent: welcome email send failed", zap.String("email", to), zap.Error(err))
		}
	}(email, temp)

	s.logger.Info("createSupportAgent: support agent created",
		zap.String("userID", user.UserID), zap.String("email", email), zap.String("role", role))
	return user.UserID, nil
}

// ─── ListSupportAgents ───────────────────────────────────────────────────────

func (s *supportServiceImpl) ListSupportAgents(ctx context.Context, limit, offset int) ([]*domain.SupportUser, int, error) {
	return s.readRepo.List(ctx, limit, offset)
}

// ─── DeactivateSupportAgent ──────────────────────────────────────────────────

func (s *supportServiceImpl) DeactivateSupportAgent(ctx context.Context, userID string) error {
	if userID == "" {
		return supportErrors.ErrInvalidInput
	}
	if err := s.writeRepo.Deactivate(ctx, userID); err != nil {
		return err
	}
	s.logger.Info("deactivateSupportAgent: success", zap.String("userID", userID))
	return nil
}

// ─── ActivateSupportAgent ────────────────────────────────────────────────────

func (s *supportServiceImpl) ActivateSupportAgent(ctx context.Context, userID string) error {
	if userID == "" {
		return supportErrors.ErrInvalidInput
	}
	if err := s.writeRepo.Activate(ctx, userID); err != nil {
		return err
	}
	s.logger.Info("activateSupportAgent: success", zap.String("userID", userID))
	return nil
}

// ─── DeleteSupportAgent ──────────────────────────────────────────────────────

// DeleteSupportAgent supprime (soft-delete) un agent. Un compte encore actif ne
// peut pas être supprimé : il doit d'abord être désactivé.
func (s *supportServiceImpl) DeleteSupportAgent(ctx context.Context, userID string) error {
	if userID == "" {
		return supportErrors.ErrInvalidInput
	}
	user, err := s.readRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	if user.IsActive {
		s.logger.Warn("deleteSupportAgent: account still active", zap.String("userID", userID))
		return supportErrors.ErrCannotDeleteActiveAccount
	}
	if err := s.writeRepo.SoftDelete(ctx, userID); err != nil {
		return err
	}
	s.logger.Info("deleteSupportAgent: success", zap.String("userID", userID))
	return nil
}

// ─── UpdateSupportAgent (admin) ──────────────────────────────────────────────

// UpdateSupportAgent modifie l'email et/ou le rôle d'un agent. Un champ vide
// signifie « inchangé ». Au moins un champ doit être fourni.
func (s *supportServiceImpl) UpdateSupportAgent(ctx context.Context, userID, newEmail, newRole string) error {
	if userID == "" {
		return supportErrors.ErrInvalidInput
	}
	newEmail = normalizeEmail(newEmail)
	if newEmail == "" && newRole == "" {
		return supportErrors.ErrInvalidInput
	}
	if newRole != "" && !domain.IsValidRole(newRole) {
		return supportErrors.ErrInvalidInput
	}

	// Vérifie l'existence du compte avant toute modification.
	user, err := s.readRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}

	// On ne traite l'email que s'il diffère réellement de l'email courant : le
	// front pré-remplit souvent l'email actuel lors d'une modif de rôle, il ne
	// faut pas le considérer comme un conflit avec lui-même.
	if newEmail != "" && newEmail != user.Email {
		exists, err := s.readRepo.ExistsByEmail(ctx, newEmail)
		if err != nil {
			return err
		}
		if exists {
			return supportErrors.ErrEmailAlreadyExists
		}
		if err := s.writeRepo.UpdateEmail(ctx, userID, newEmail); err != nil {
			return err
		}
	}

	if newRole != "" {
		if err := s.writeRepo.UpdateRole(ctx, userID, newRole); err != nil {
			return err
		}
	}

	s.logger.Info("updateSupportAgent: success",
		zap.String("userID", userID), zap.Bool("emailChanged", newEmail != ""), zap.Bool("roleChanged", newRole != ""))
	return nil
}

// ─── ForgotPassword (public) ─────────────────────────────────────────────────

// ForgotPassword déclenche le flux de réinitialisation. Réponse anti-énumération :
// l'appelant reçoit toujours un succès, quel que soit l'état du compte.
//   - compte admin  → lien de reset envoyé directement à sa propre adresse.
//   - compte support → demande enregistrée + email de notification aux admins.
func (s *supportServiceImpl) ForgotPassword(ctx context.Context, email string) error {
	email = normalizeEmail(email)
	if email == "" {
		return nil
	}
	user, err := s.readRepo.GetByEmail(ctx, email)
	if err != nil || !user.IsActive {
		// Anti-énumération : on ne révèle pas l'existence du compte.
		return nil
	}

	if user.Role == domain.RoleAdmin {
		if err := s.sendResetLink(ctx, user); err != nil {
			s.logger.Error("forgotPassword: admin self-service reset failed",
				zap.String("userID", user.UserID), zap.Error(err))
		}
		return nil
	}

	// Compte support : enregistre la demande puis notifie les admins.
	if err := s.writeRepo.SetPasswordResetRequested(ctx, user.UserID); err != nil {
		s.logger.Error("forgotPassword: set request flag failed",
			zap.String("userID", user.UserID), zap.Error(err))
		return nil
	}
	s.notifyAdminsOfResetRequest(ctx, user.Email)
	s.logger.Info("forgotPassword: support reset request recorded", zap.String("userID", user.UserID))
	return nil
}

// ─── ListPasswordResetRequests (admin) ───────────────────────────────────────

func (s *supportServiceImpl) ListPasswordResetRequests(ctx context.Context) ([]*domain.SupportUser, error) {
	return s.readRepo.ListPendingPasswordResets(ctx)
}

// ─── TriggerPasswordReset (admin) ────────────────────────────────────────────

// TriggerPasswordReset envoie un lien de réinitialisation à l'agent ciblé.
// La demande est « réclamée » de façon atomique : si un autre admin l'a déjà
// traitée, ErrResetAlreadyProcessed est renvoyé et aucun email n'est envoyé.
func (s *supportServiceImpl) TriggerPasswordReset(ctx context.Context, userID string) error {
	if userID == "" {
		return supportErrors.ErrInvalidInput
	}
	user, err := s.readRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	// Réclame la demande AVANT l'envoi : seul le premier admin réussit.
	if err := s.writeRepo.ClaimPasswordResetRequest(ctx, userID); err != nil {
		return err
	}
	if err := s.sendResetLink(ctx, user); err != nil {
		// L'envoi (génération du token) a échoué : on restaure la demande pour
		// qu'un admin puisse réessayer.
		if restoreErr := s.writeRepo.SetPasswordResetRequested(ctx, userID); restoreErr != nil {
			s.logger.Error("triggerPasswordReset: restore request flag failed",
				zap.String("userID", userID), zap.Error(restoreErr))
		}
		return err
	}
	s.logger.Info("triggerPasswordReset: reset link sent", zap.String("userID", userID))
	return nil
}

// ─── ResetPassword (public) ──────────────────────────────────────────────────

// ResetPassword applique un nouveau mot de passe à partir d'un token de reset valide.
func (s *supportServiceImpl) ResetPassword(ctx context.Context, rawToken, newPassword string) error {
	// Valider la force AVANT de consommer le token : un mot de passe faible ne doit
	// pas invalider le token (l'utilisateur peut réessayer avec le même lien).
	if err := password.ValidateStrength(newPassword); err != nil {
		return supportErrors.ErrWeakPassword
	}
	userID, err := s.resetStore.Consume(ctx, rawToken)
	if err != nil {
		return err
	}
	hash, err := password.Hash(newPassword)
	if err != nil {
		s.logger.Error("resetPassword: hash failed", zap.String("userID", userID), zap.Error(err))
		return supportErrors.ErrInternal
	}
	if err := s.writeRepo.UpdatePassword(ctx, userID, hash, false); err != nil {
		return err
	}
	// Efface une éventuelle demande en attente (cas support).
	if err := s.writeRepo.ClearPasswordResetRequested(ctx, userID); err != nil {
		s.logger.Warn("resetPassword: clear request flag failed",
			zap.String("userID", userID), zap.Error(err))
	}
	s.logger.Info("resetPassword: success", zap.String("userID", userID))
	return nil
}

// sendResetLink génère un token de reset et envoie le lien par email à l'utilisateur.
func (s *supportServiceImpl) sendResetLink(ctx context.Context, user *domain.SupportUser) error {
	raw, err := s.resetStore.Issue(ctx, user.UserID)
	if err != nil {
		return err
	}
	link := strings.TrimRight(s.cfg.FrontendURL, "/") + "/reset-password?token=" + raw

	go func(to, link string) {
		bgCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := s.emailClient.SendEmail(bgCtx, to,
			"Réinitialisation de votre mot de passe TissiMah Support",
			"Pour réinitialiser votre mot de passe, ouvrez ce lien : "+link+"\nCe lien expire bientôt. Si vous n'êtes pas à l'origine de cette demande, ignorez cet email.",
			"<p>Pour réinitialiser votre mot de passe, cliquez sur ce lien : <a href=\""+link+"\">Réinitialiser mon mot de passe</a></p><p>Ce lien expire bientôt. Si vous n'êtes pas à l'origine de cette demande, ignorez cet email.</p>",
		); err != nil {
			s.logger.Error("sendResetLink: email send failed", zap.String("to", to), zap.Error(err))
		}
	}(user.Email, link)

	return nil
}

// notifyAdminsOfResetRequest envoie un email à tous les admins actifs pour signaler
// qu'un agent support a demandé une réinitialisation de mot de passe.
func (s *supportServiceImpl) notifyAdminsOfResetRequest(ctx context.Context, agentEmail string) {
	admins, err := s.readRepo.ListAdminEmails(ctx)
	if err != nil {
		s.logger.Error("notifyAdminsOfResetRequest: list admins failed", zap.Error(err))
		return
	}
	if len(admins) == 0 {
		return
	}
	go func(recipients []string, agent string) {
		bgCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		subject := "Demande de réinitialisation de mot de passe — agent support"
		text := "L'agent support " + agent + " a demandé une réinitialisation de mot de passe.\n" +
			"Connectez-vous au back-office pour traiter la demande."
		html := "<p>L'agent support <strong>" + agent + "</strong> a demandé une réinitialisation de mot de passe.</p>" +
			"<p>Connectez-vous au back-office pour traiter la demande.</p>"
		for _, to := range recipients {
			if err := s.emailClient.SendEmail(bgCtx, to, subject, text, html); err != nil {
				s.logger.Error("notifyAdminsOfResetRequest: email send failed",
					zap.String("to", to), zap.Error(err))
			}
		}
	}(admins, agentEmail)
}
