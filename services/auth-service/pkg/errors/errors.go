package errors

import "errors"

var (
	ErrorInternalServer          = errors.New("ErrorInternalServer")
	ErrorUserNotFound            = errors.New("ErrorUserNotFound")
	ErrorEmailNotAvailable       = errors.New("ErrorEmailNotAvailable")
	ErrorPhoneNumberNotAvailable = errors.New("ErrorPhoneNumberNotAvailable")
	ErrorDataRetrievalFailed     = errors.New("ErrorDataRetrievalFailed")
	ErrorCantDeleteAccount       = errors.New("ErrorCantDeleteAccount")
	ErrorAccountBanned           = errors.New("ErrorAccountBanned")
)
