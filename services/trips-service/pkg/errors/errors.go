package errors

import "errors"

var (
	ErrorInternalServer           = errors.New("ErrorInternalServer")
	ErrorTripNotFound             = errors.New("ErrorTripNotFound")
	ErrorDriverNotVerified        = errors.New("ErrorDriverNotVerified")
	ErrorDriverNotFound           = errors.New("ErrorDriverNotFound")
	ErrorInvalidInput             = errors.New("ErrorInvalidInput")
	ErrorInvalidDatetime          = errors.New("ErrorInvalidDatetime")
	ErrorInvalidWaypoints         = errors.New("ErrorInvalidWaypoints")
	ErrorUnauthorized             = errors.New("ErrorUnauthorized")
	ErrorDataRetrievalFailed      = errors.New("ErrorDataRetrievalFailed")
)
