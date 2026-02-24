package errors

import "errors"

var (
	ErrorInternalServer       = errors.New("ErrorInternalServer")
	ErrorUserNotFound         = errors.New("ErrorUserNotFound")
	ErrorDataRetrievalFailed  = errors.New("ErrorDataRetrievalFailed")
	ErrorInvalidProfileID     = errors.New("ErrorInvalidProfileID")
	ErrorProfileAlreadyExists = errors.New("ErrorProfileAlreadyExists")
	ErrorAuthServiceUnavailable = errors.New("ErrorAuthServiceUnavailable")
)
