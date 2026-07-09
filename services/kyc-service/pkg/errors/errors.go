package errors

import "errors"

var (
	ErrorInternalServer            = errors.New("ErrorInternalServer")
	ErrorInquiryAlreadyActive      = errors.New("ErrorInquiryAlreadyActive")
	ErrorInquiryNotFound           = errors.New("ErrorInquiryNotFound")
	ErrorInquiryNotResumable       = errors.New("ErrorInquiryNotResumable")
	ErrorUnauthorized              = errors.New("ErrorUnauthorized")
	ErrorInvalidWebhookSignature   = errors.New("ErrorInvalidWebhookSignature")
	ErrorInvalidDecision           = errors.New("ErrorInvalidDecision")
	ErrorInvalidDateRange          = errors.New("ErrorInvalidDateRange")
	ErrorReviewNotOverridable      = errors.New("ErrorReviewNotOverridable")
	ErrorOnlyRejectionOverridable  = errors.New("ErrorOnlyRejectionOverridable")
	ErrorDocumentAlreadyReviewed   = errors.New("ErrorDocumentAlreadyReviewed")
	ErrorReviewNotFound            = errors.New("ErrorReviewNotFound")
	ErrorMissingUserID             = errors.New("ErrorMissingUserID")
	ErrorMissingDocumentType       = errors.New("ErrorMissingDocumentType")
	ErrorMissingDocumentID         = errors.New("ErrorMissingDocumentID")
	ErrorDocumentMismatch          = errors.New("ErrorDocumentMismatch")
	ErrorMissingInquiryID          = errors.New("ErrorMissingInquiryID")
	ErrorMissingReviewID           = errors.New("ErrorMissingReviewID")
	ErrorFileServiceUnavailable    = errors.New("ErrorFileServiceUnavailable")
	ErrorPersonaUnavailable        = errors.New("ErrorPersonaUnavailable")
	ErrorUserNotFound              = errors.New("ErrorUserNotFound")
	ErrorVehicleDocumentNotAllowed = errors.New("ErrorVehicleDocumentNotAllowed")
	ErrorCompanionDocumentMissing  = errors.New("ErrorCompanionDocumentMissing")
)
