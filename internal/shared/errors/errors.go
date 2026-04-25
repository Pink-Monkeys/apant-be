package errors

import (
	stdErrors "errors"
	"fmt"
	"net/http"
)

// AppError keeps an HTTP code attached to domain/application failures.
type AppError struct {
	Code    int
	Message string
	Err     error
}

func (e *AppError) Error() string {
	if e.Err == nil {
		return e.Message
	}
	return fmt.Sprintf("%s: %v", e.Message, e.Err)
}

func (e *AppError) Unwrap() error {
	return e.Err
}

func New(code int, message string) *AppError {
	return &AppError{Code: code, Message: message}
}

func Wrap(code int, message string, err error) *AppError {
	return &AppError{Code: code, Message: message, Err: err}
}

func Resolve(err error) (int, string) {
	if err == nil {
		return http.StatusOK, ""
	}

	var appErr *AppError
	if stdErrors.As(err, &appErr) {
		if appErr.Code <= 0 {
			return http.StatusInternalServerError, appErr.Message
		}
		return appErr.Code, appErr.Message
	}

	return http.StatusInternalServerError, "internal server error"
}
