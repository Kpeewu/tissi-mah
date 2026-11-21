package errors

import "errors"

var (
	ErrorInvalidInput    = errors.New("ErrorInvalidInput")
	ErrorProviderFailed  = errors.New("ErrorProviderFailed")
	ErrorInternalServer  = errors.New("ErrorInternalServer")
)
