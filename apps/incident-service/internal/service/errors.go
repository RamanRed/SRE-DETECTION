package service

import "errors"

type ErrorCode string

const (
	CodeInvalid     ErrorCode = "INVALID"
	CodeNotFound    ErrorCode = "NOT_FOUND"
	CodeConflict    ErrorCode = "CONFLICT"
	CodeUnavailable ErrorCode = "UNAVAILABLE"
)

type Error struct {
	Code    ErrorCode
	Message string
	Cause   error
}

func (e *Error) Error() string {
	return e.Message
}

func (e *Error) Unwrap() error {
	return e.Cause
}

func IsCode(err error, code ErrorCode) bool {
	var serviceError *Error
	return errors.As(err, &serviceError) && serviceError.Code == code
}

func errorWith(code ErrorCode, message string, cause error) error {
	return &Error{Code: code, Message: message, Cause: cause}
}
