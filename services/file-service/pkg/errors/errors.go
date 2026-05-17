package errors

import "errors"

var (
	ErrorInternalServer            = errors.New("ErrorInternalServer")
	ErrorDocumentNotFound          = errors.New("ErrorDocumentNotFound")
	ErrorUnauthorized              = errors.New("ErrorUnauthorized")
	ErrorInvalidDocumentType       = errors.New("ErrorInvalidDocumentType")
	ErrorInvalidMimeType           = errors.New("ErrorInvalidMimeType")
	ErrorUploadFailed              = errors.New("ErrorUploadFailed")
	ErrorDataRetrievalFailed       = errors.New("ErrorDataRetrievalFailed")
	ErrorReviewNotFound            = errors.New("ErrorReviewNotFound")
	ErrorInvalidReviewDecision     = errors.New("ErrorInvalidReviewDecision")
	ErrorFileTooLarge              = errors.New("ErrorFileTooLarge")
	ErrorMissingUserID             = errors.New("ErrorMissingUserID")
	ErrorMissingDocumentReference  = errors.New("ErrorMissingDocumentReference")
	ErrorMultipleDocumentReference = errors.New("ErrorMultipleDocumentReference")
	ErrorInvalidReviewStatus       = errors.New("ErrorInvalidReviewStatus")
	ErrorInvalidReviewType         = errors.New("ErrorInvalidReviewType")
	ErrorInvalidReasonRejection    = errors.New("ErrorInvalidReasonRejection")
	ErrorUserServiceUnavailable    = errors.New("ErrorUserServiceUnavailable")
	ErrorInvalidInput              = errors.New("ErrorInvalidInput")
	ErrorContentBlocked            = errors.New("ErrorContentBlocked")
)
