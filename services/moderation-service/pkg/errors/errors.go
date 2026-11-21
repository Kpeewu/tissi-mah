package errors

import "errors"

var (
	// ─── Erreurs générales ────────────────────────────────────────────────────
	ErrorInternalServer      = errors.New("ErrorInternalServer")
	ErrorInvalidInput        = errors.New("ErrorInvalidInput")
	ErrorDataRetrievalFailed = errors.New("ErrorDataRetrievalFailed")

	// ─── Validation du contenu ───────────────────────────────────────────────
	ErrorContentBlocked       = errors.New("ErrorContentBlocked")       // Contenu rejeté (décision finale)
	ErrorUnsupportedMediaType = errors.New("ErrorUnsupportedMediaType") // Type MIME non autorisé pour upload
	ErrorInvalidImageFormat   = errors.New("ErrorInvalidImageFormat")   // Magic bytes invalides ou fichier corrompu
	ErrorFileTooLarge         = errors.New("ErrorFileTooLarge")         // Taille dépasse la limite configurée

	// ─── Compte utilisateur ──────────────────────────────────────────────────
	ErrorAccountSuspended = errors.New("ErrorAccountSuspended") // Compte suspendu temporairement
	ErrorAccountBanned    = errors.New("ErrorAccountBanned")    // Compte banni définitivement

	// ─── Disponibilité des services ──────────────────────────────────────────
	ErrorAuthServiceUnavailable = errors.New("ErrorAuthServiceUnavailable") // auth-service injoignable
	ErrorUserServiceUnavailable = errors.New("ErrorUserServiceUnavailable") // user-service injoignable
	ErrorModerationAPIFailed    = errors.New("ErrorModerationAPIFailed")    // Perspective API / Vision indisponible
)
