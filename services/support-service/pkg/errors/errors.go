package errors

import "errors"

var (
	ErrInternal            = errors.New("ErrInternal")
	ErrInvalidInput        = errors.New("ErrInvalidInput")
	ErrInvalidCredentials  = errors.New("ErrInvalidCredentials")
	ErrAccountLocked       = errors.New("ErrAccountLocked")
	ErrAccountInactive     = errors.New("ErrAccountInactive")
	ErrUserNotFound        = errors.New("ErrUserNotFound")
	ErrEmailAlreadyExists  = errors.New("ErrEmailAlreadyExists")
	ErrOTPExpired          = errors.New("ErrOTPExpired")
	ErrOTPSessionNotFound  = errors.New("ErrOTPSessionNotFound")
	ErrOTPInvalid          = errors.New("ErrOTPInvalid")
	ErrOTPTooManyAttempts  = errors.New("ErrOTPTooManyAttempts")
	ErrResendCooldown      = errors.New("ErrResendCooldown")
	ErrRefreshInvalid      = errors.New("ErrRefreshInvalid")
	ErrRefreshRevoked      = errors.New("ErrRefreshRevoked")
	ErrForbidden           = errors.New("ErrForbidden")
	ErrWeakPassword        = errors.New("ErrWeakPassword")
	ErrEmailChangeCooldown = errors.New("ErrEmailChangeCooldown")
	ErrMustChangePassword  = errors.New("ErrMustChangePassword")
	ErrUnauthenticated     = errors.New("ErrUnauthenticated")
	// ErrCannotDeleteActiveAccount est renvoyé quand on tente de supprimer un
	// compte encore actif : il doit d'abord être désactivé.
	ErrCannotDeleteActiveAccount = errors.New("ErrCannotDeleteActiveAccount")
	// ErrResetTokenInvalid : token de réinitialisation absent, expiré ou déjà utilisé.
	ErrResetTokenInvalid = errors.New("ErrResetTokenInvalid")
	// ErrResetAlreadyProcessed : la demande de réinitialisation a déjà été traitée
	// par un autre admin (aucune demande en attente pour ce compte).
	ErrResetAlreadyProcessed = errors.New("ErrResetAlreadyProcessed")
)
