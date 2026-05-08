package errors

import "errors"

var (
	ErrMissingBookingID          = errors.New("booking_id is required")
	ErrMissingThreadID           = errors.New("thread_id is required")
	ErrMissingMessageID          = errors.New("message_id is required")
	ErrEmptyMessage              = errors.New("message content cannot be empty")
	ErrMessageTooLong            = errors.New("message exceeds maximum length")
	ErrBookingNotFound           = errors.New("booking not found")
	ErrBookingNotApproved        = errors.New("booking must be approved to start a conversation")
	ErrTripAlreadyEnded          = errors.New("conversation closed: trip has ended")
	ErrThreadNotFound            = errors.New("conversation thread not found")
	ErrThreadClosed              = errors.New("conversation is closed: trip has ended")
	ErrMessageNotFound           = errors.New("message not found")
	ErrMessageNotFlagged         = errors.New("message is not flagged")
	ErrUnauthorized              = errors.New("not authorized to access this conversation")
	ErrBookingServiceUnavailable = errors.New("booking service unavailable")
	ErrTripServiceUnavailable    = errors.New("trips service unavailable")
)
