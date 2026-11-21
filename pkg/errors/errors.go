// Package errors provides shared error types and utilities for all services.
package errors

import (
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// AppError represents an application-level error.
type AppError struct {
	Code    codes.Code
	Message string
	Err     error
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *AppError) Unwrap() error {
	return e.Err
}

// GRPCStatus returns the gRPC status for this error.
func (e *AppError) GRPCStatus() *status.Status {
	return status.New(e.Code, e.Message)
}

// Common error constructors

// NotFound creates a not found error.
func NotFound(message string) *AppError {
	return &AppError{Code: codes.NotFound, Message: message}
}

// InvalidArgument creates an invalid argument error.
func InvalidArgument(message string) *AppError {
	return &AppError{Code: codes.InvalidArgument, Message: message}
}

// Internal creates an internal error.
func Internal(message string, err error) *AppError {
	return &AppError{Code: codes.Internal, Message: message, Err: err}
}

// Unauthenticated creates an unauthenticated error.
func Unauthenticated(message string) *AppError {
	return &AppError{Code: codes.Unauthenticated, Message: message}
}

// PermissionDenied creates a permission denied error.
func PermissionDenied(message string) *AppError {
	return &AppError{Code: codes.PermissionDenied, Message: message}
}

// AlreadyExists creates an already exists error.
func AlreadyExists(message string) *AppError {
	return &AppError{Code: codes.AlreadyExists, Message: message}
}
