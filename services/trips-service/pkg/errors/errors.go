package errors

import "errors"

var (
	ErrorInternalServer               = errors.New("ErrorInternalServer")
	ErrorTripNotFound                 = errors.New("ErrorTripNotFound")
	ErrorDriverNotVerified            = errors.New("ErrorDriverNotVerified")
	ErrorDriverNotFound               = errors.New("ErrorDriverNotFound")
	ErrorInvalidInput                 = errors.New("ErrorInvalidInput")
	ErrorInvalidDatetime              = errors.New("ErrorInvalidDatetime")
	ErrorInvalidWaypoints             = errors.New("ErrorInvalidWaypoints")
	ErrorUnauthorized                 = errors.New("ErrorUnauthorized")
	ErrorDataRetrievalFailed          = errors.New("ErrorDataRetrievalFailed")
	ErrorTripNotScheduled             = errors.New("ErrorTripNotScheduled")
	ErrorVehicleNotFound              = errors.New("ErrorVehicleNotFound")
	ErrorVehicleInsufficientSeats     = errors.New("ErrorVehicleInsufficientSeats")
	ErrorTripDepartureTooSoon         = errors.New("ErrorTripDepartureTooSoon")
	ErrorDriverAlreadyHasActiveTrip   = errors.New("ErrorDriverAlreadyHasActiveTrip")
	ErrorTripNotInProgress            = errors.New("ErrorTripNotInProgress")
	ErrorWaypointNotFound             = errors.New("ErrorWaypointNotFound")
	ErrorWaypointNotAStop             = errors.New("ErrorWaypointNotAStop")
	ErrorWaypointAlreadyArrived       = errors.New("ErrorWaypointAlreadyArrived")
	ErrorPreviousWaypointNotConfirmed = errors.New("ErrorPreviousWaypointNotConfirmed")
	ErrorAnotherStopAlreadyActive     = errors.New("ErrorAnotherStopAlreadyActive")
	ErrorWaypointNotArrived           = errors.New("ErrorWaypointNotArrived")
	ErrorWaypointAlreadyDeparted      = errors.New("ErrorWaypointAlreadyDeparted")
)
