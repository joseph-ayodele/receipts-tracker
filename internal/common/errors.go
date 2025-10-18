package common

import (
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// AppError represents application-specific errors
type AppError struct {
	Code    string
	Message string
	Cause   error
}

func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error {
	return e.Cause
}

// Common application errors
var (
	ErrNotFound     = errors.New("resource not found")
	ErrInvalidInput = errors.New("invalid input")
	ErrUnauthorized = errors.New("unauthorized")
	ErrInternal     = errors.New("internal error")
	ErrDatabase     = errors.New("database error")
	ErrValidation   = errors.New("validation failed")
)

// Error constructors
func NewAppError(code, message string, cause error) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}

func WrapError(err error, message string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", message, err)
}

// gRPC error helpers
func InvalidArgumentError(message string) error {
	return status.Error(codes.InvalidArgument, message)
}

func NotFoundError(message string) error {
	return status.Error(codes.NotFound, message)
}

func InternalError(message string) error {
	return status.Error(codes.Internal, message)
}

func InvalidArgumentErrorf(format string, args ...interface{}) error {
	return InvalidArgumentError(fmt.Sprintf(format, args...))
}

func InternalErrorf(format string, args ...interface{}) error {
	return InternalError(fmt.Sprintf(format, args...))
}
