package errors

import "errors"

var (
	ErrorInternalServer         = errors.New("ErrorInternalServer")
	ErrorUserNotFound           = errors.New("ErrorUserNotFound")
	ErrorDataRetrievalFailed    = errors.New("ErrorDataRetrievalFailed")
	ErrorInvalidUserID          = errors.New("ErrorInvalidUserID")
	ErrorProfileAlreadyExists   = errors.New("ErrorProfileAlreadyExists")
	ErrorAuthServiceUnavailable = errors.New("ErrorAuthServiceUnavailable")
)
