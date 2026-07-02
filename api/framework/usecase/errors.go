package usecase

import "errors"

type ErrorCode string

const (
	CodeValidation   ErrorCode = "validation"
	CodeNotFound     ErrorCode = "not_found"
	CodeUnauthorized ErrorCode = "unauthorized"
	CodeForbidden    ErrorCode = "forbidden"
	CodeConflict     ErrorCode = "conflict"
	CodeInternal     ErrorCode = "internal"
)

type Error struct {
	Code    ErrorCode
	Message string
	Cause   error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Code != "" {
		return string(e.Code)
	}
	return "usecase error"
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func E(code ErrorCode, message string, cause error) error {
	return &Error{
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}

func CodeOf(err error) ErrorCode {
	if err == nil {
		return ""
	}

	var usecaseErr *Error
	if errors.As(err, &usecaseErr) && usecaseErr.Code != "" {
		return usecaseErr.Code
	}
	return CodeInternal
}

func MessageOf(err error, fallback string) string {
	if err == nil {
		return fallback
	}

	var usecaseErr *Error
	if errors.As(err, &usecaseErr) && usecaseErr.Message != "" {
		return usecaseErr.Message
	}
	if fallback != "" {
		return fallback
	}
	return "internal error"
}

func LogErrorOf(err error) error {
	if err == nil {
		return nil
	}

	var usecaseErr *Error
	if errors.As(err, &usecaseErr) && usecaseErr.Cause != nil {
		return LogErrorOf(usecaseErr.Cause)
	}
	return err
}

func IsCode(err error, code ErrorCode) bool {
	return CodeOf(err) == code
}
