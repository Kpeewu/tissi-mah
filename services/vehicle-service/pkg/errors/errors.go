package errors

import "errors"

var (
	ErrorInternalServer       = errors.New("ErrorInternalServer")
	ErrorVehicleNotFound      = errors.New("ErrorVehicleNotFound")
	ErrorUnauthorized         = errors.New("ErrorUnauthorized")
	ErrorLicencePlateConflict = errors.New("ErrorLicencePlateConflict")
	ErrorDataRetrievalFailed  = errors.New("ErrorDataRetrievalFailed")
	ErrorInvalidInput         = errors.New("ErrorInvalidInput")
)
