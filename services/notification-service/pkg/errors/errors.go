package errors

import "errors"

var (
	ErrorNotFound          = errors.New("ErrorNotFound")
	ErrorInvalidInput      = errors.New("ErrorInvalidInput")
	ErrorDuplicate         = errors.New("ErrorDuplicate")
	ErrorInternalServer    = errors.New("ErrorInternalServer")
	ErrorDataRetrievalFail = errors.New("ErrorDataRetrievalFail")
	ErrorDataInsertFail    = errors.New("ErrorDataInsertFail")
	ErrorDataUpdateFail    = errors.New("ErrorDataUpdateFail")
	ErrorDataDeleteFail    = errors.New("ErrorDataDeleteFail")
	ErrorTemplateNotFound  = errors.New("ErrorTemplateNotFound")
	ErrorRoutingNotFound   = errors.New("ErrorRoutingNotFound")
	// ErrorUserNotProvisioned : le Firebase UID est valide mais user-service ne connaît
	// pas encore l'utilisateur (compte backend créé après la vérification e-mail).
	// Cas normal pendant l'inscription — à distinguer d'une vraie erreur interne.
	ErrorUserNotProvisioned = errors.New("ErrorUserNotProvisioned")
)
