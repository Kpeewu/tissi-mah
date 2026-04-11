package errors

import "errors"

var (
	ErrorInternalServer          = errors.New("ErrorInternalServer")
	ErrorInquiryAlreadyActive    = errors.New("ErrorInquiryAlreadyActive")
	ErrorInquiryNotFound         = errors.New("ErrorInquiryNotFound")
	ErrorInquiryNotResumable     = errors.New("ErrorInquiryNotResumable")
	ErrorUnauthorized            = errors.New("ErrorUnauthorized")
	ErrorInvalidWebhookSignature = errors.New("ErrorInvalidWebhookSignature")
	ErrorInvalidDecision         = errors.New("ErrorInvalidDecision")
	ErrorReviewNotOverridable    = errors.New("ErrorReviewNotOverridable")
	ErrorReviewNotFound          = errors.New("ErrorReviewNotFound")
	ErrorMissingUserID           = errors.New("ErrorMissingUserID")
	ErrorMissingDocumentType     = errors.New("ErrorMissingDocumentType")
	ErrorMissingInquiryID        = errors.New("ErrorMissingInquiryID")
	ErrorMissingReviewID         = errors.New("ErrorMissingReviewID")
	ErrorFileServiceUnavailable  = errors.New("ErrorFileServiceUnavailable")
	ErrorPersonaUnavailable      = errors.New("ErrorPersonaUnavailable")
	ErrorUserNotFound            = errors.New("ErrorUserNotFound")
)
