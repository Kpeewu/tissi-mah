package errors

import "errors"

var (
	ErrorInternalServer           = errors.New("ErrorInternalServer")
	ErrorUnauthorized             = errors.New("ErrorUnauthorized")
	ErrorInvalidDecision          = errors.New("ErrorInvalidDecision")
	ErrorInvalidDateRange         = errors.New("ErrorInvalidDateRange")
	ErrorReviewNotOverridable     = errors.New("ErrorReviewNotOverridable")
	ErrorOnlyRejectionOverridable = errors.New("ErrorOnlyRejectionOverridable")
	ErrorDocumentAlreadyReviewed  = errors.New("ErrorDocumentAlreadyReviewed")
	ErrorReviewNotFound           = errors.New("ErrorReviewNotFound")
	ErrorMissingUserID            = errors.New("ErrorMissingUserID")
	ErrorMissingDocumentID        = errors.New("ErrorMissingDocumentID")
	ErrorDocumentMismatch         = errors.New("ErrorDocumentMismatch")
	ErrorMissingReviewID          = errors.New("ErrorMissingReviewID")
	ErrorFileServiceUnavailable   = errors.New("ErrorFileServiceUnavailable")
	ErrorUserNotFound             = errors.New("ErrorUserNotFound")
	ErrorDocumentNotFound         = errors.New("ErrorDocumentNotFound")
	ErrorCompanionDocumentMissing = errors.New("ErrorCompanionDocumentMissing")
)
