package errors

import "errors"

var (
	ErrorInvalidInput   = errors.New("ErrorInvalidInput")
	ErrorFCMFailed      = errors.New("ErrorFCMFailed")
	ErrorInternalServer = errors.New("ErrorInternalServer")
)
