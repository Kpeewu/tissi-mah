package domain

import "errors"

// Erreurs de validation de format
var (
	ErrEmailInvalidFormat = errors.New("ErrorEmailInvalidFormat")
	ErrEmailTooLong       = errors.New("ErrorEmailTooLong")
	ErrPhoneInvalidFormat = errors.New("ErrorPhoneInvalidFormat")
)
