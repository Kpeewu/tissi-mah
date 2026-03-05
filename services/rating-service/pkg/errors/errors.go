package errors

import "errors"

var (
	ErrorInternalServer      = errors.New("ErrorInternalServer")
	ErrorRatingNotFound      = errors.New("ErrorRatingNotFound")
	ErrorRatingAlreadyExists = errors.New("ErrorRatingAlreadyExists")
	ErrorInvalidStars        = errors.New("ErrorInvalidStars")
	ErrorSelfRating          = errors.New("ErrorSelfRating")
	ErrorDataRetrievalFailed = errors.New("ErrorDataRetrievalFailed")
	ErrorUnauthorizedAction  = errors.New("ErrorUnauthorizedAction")
	ErrorCantDeleteRating    = errors.New("ErrorCantDeleteRating")
	ErrorUserNotFound        = errors.New("ErrorUserNotFound")
	ErrorMissingRaterID      = errors.New("ErrorMissingRaterID")
)
