package errors

import "errors"

var (
	ErrorInternalServer          = errors.New("ErrorInternalServer")
	ErrorInvalidInput            = errors.New("ErrorInvalidInput")
	ErrorBookingNotFound         = errors.New("ErrorBookingNotFound")
	ErrorTripNotFound            = errors.New("ErrorTripNotFound")
	ErrorTripNotAvailable        = errors.New("ErrorTripNotAvailable")
	ErrorNoSeatsAvailable        = errors.New("ErrorNoSeatsAvailable")
	ErrorPassengerNotFound       = errors.New("ErrorPassengerNotFound")
	ErrorDriverNotFound          = errors.New("ErrorDriverNotFound")
	ErrorUnauthorized            = errors.New("ErrorUnauthorized")
	ErrorInvalidStatusTransition = errors.New("ErrorInvalidStatusTransition")
	ErrorBookingAlreadyCancelled = errors.New("ErrorBookingAlreadyCancelled")
	ErrorBookingNotPending       = errors.New("ErrorBookingNotPending")
	ErrorBookingNotApproved      = errors.New("ErrorBookingNotApproved")
	ErrorBookingNotInProgress    = errors.New("ErrorBookingNotInProgress")
	ErrorPassengerIsDriver       = errors.New("ErrorPassengerIsDriver")
	ErrorDuplicateBooking        = errors.New("ErrorDuplicateBooking")
	ErrorPaymentAlreadyConfirmed = errors.New("ErrorPaymentAlreadyConfirmed")
	ErrorPaymentAlreadyFailed    = errors.New("ErrorPaymentAlreadyFailed")
	ErrorDataRetrievalFailed     = errors.New("ErrorDataRetrievalFailed")
	ErrorSeatReservationFailed   = errors.New("ErrorSeatReservationFailed")
	ErrorInvalidWaypoints        = errors.New("ErrorInvalidWaypoints")
	ErrorPassengerNotVerified    = errors.New("ErrorPassengerNotVerified")
)
