package errors

import "errors"

var (
	ErrorInternalServer           = errors.New("ErrorInternalServer")
	ErrorInvalidInput             = errors.New("ErrorInvalidInput")
	ErrorUnauthorized             = errors.New("ErrorUnauthorized")
	ErrorDataRetrievalFailed      = errors.New("ErrorDataRetrievalFailed")
	ErrorPaymentNotFound          = errors.New("ErrorPaymentNotFound")
	ErrorPaymentAlreadyHeld       = errors.New("ErrorPaymentAlreadyHeld")
	ErrorPaymentAlreadyFailed     = errors.New("ErrorPaymentAlreadyFailed")
	ErrorInvalidPaymentStatus     = errors.New("ErrorInvalidPaymentStatus")
	ErrorRefundNotFound           = errors.New("ErrorRefundNotFound")
	ErrorRefundAlreadyProcessed   = errors.New("ErrorRefundAlreadyProcessed")
	ErrorRefundAmountExceedsPayment = errors.New("ErrorRefundAmountExceedsPayment")
	ErrorPayoutNotFound           = errors.New("ErrorPayoutNotFound")
	ErrorPayoutAlreadyExists      = errors.New("ErrorPayoutAlreadyExists")
	ErrorInsufficientBalance      = errors.New("ErrorInsufficientBalance")
	ErrorFedaPayAPIError          = errors.New("ErrorFedaPayAPIError")
	ErrorWebhookVerificationFailed = errors.New("ErrorWebhookVerificationFailed")
	ErrorDuplicateWebhookEvent    = errors.New("ErrorDuplicateWebhookEvent")
	ErrorDuplicatePayment         = errors.New("ErrorDuplicatePayment")
	ErrorBookingNotFound          = errors.New("ErrorBookingNotFound")
	ErrorUserNotFound             = errors.New("ErrorUserNotFound")
)
