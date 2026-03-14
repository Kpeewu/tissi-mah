package errors

import "errors"

var (
	ErrorInternalServer        = errors.New("ErrorInternalServer")
	ErrorDocumentNotFound      = errors.New("ErrorDocumentNotFound")
	ErrorUnauthorized          = errors.New("ErrorUnauthorized")
	ErrorInvalidDocumentType   = errors.New("ErrorInvalidDocumentType")
	ErrorInvalidMimeType       = errors.New("ErrorInvalidMimeType")
	ErrorUploadFailed          = errors.New("ErrorUploadFailed")
	ErrorDataRetrievalFailed   = errors.New("ErrorDataRetrievalFailed")
	ErrorReviewNotFound        = errors.New("ErrorReviewNotFound")
	ErrorInvalidReviewDecision = errors.New("ErrorInvalidReviewDecision")
	ErrorFileTooLarge              = errors.New("ErrorFileTooLarge")
	ErrorMissingDocumentReference  = errors.New("ErrorMissingDocumentReference")
	ErrorMultipleDocumentReference = errors.New("ErrorMultipleDocumentReference")
)
