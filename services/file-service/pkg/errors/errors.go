package errors

import "errors"

var (
	ErrorInternalServer        = errors.New("ErrorInternalServer")
	ErrorDocumentNotFound      = errors.New("ErrorDocumentNotFound")
	ErrorInvalidDocumentType   = errors.New("ErrorInvalidDocumentType")
	ErrorInvalidMimeType       = errors.New("ErrorInvalidMimeType")
	ErrorUploadFailed          = errors.New("ErrorUploadFailed")
	ErrorDataRetrievalFailed   = errors.New("ErrorDataRetrievalFailed")
	ErrorReviewNotFound        = errors.New("ErrorReviewNotFound")
	ErrorInvalidReviewDecision = errors.New("ErrorInvalidReviewDecision")
	ErrorFileTooLarge          = errors.New("ErrorFileTooLarge")
)
