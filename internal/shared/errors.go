package shared

import (
	"errors"
	"fmt"
	"net/http"
)

type AppError struct {
	Code       string `json:"code"`
	Path       string `json:"path,omitempty"`
	Message    string `json:"message"`
	Hint       string `json:"hint,omitempty"`
	HTTPStatus int    `json:"-"`
}

func (e *AppError) Error() string {
	if e.Path == "" {
		return fmt.Sprintf("%s: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("%s at %s: %s", e.Code, e.Path, e.Message)
}

func NewError(code, path, message string) *AppError {
	return &AppError{Code: code, Path: path, Message: message, HTTPStatus: http.StatusInternalServerError}
}

func BadRequest(code, path, message string) *AppError {
	return &AppError{Code: code, Path: path, Message: message, HTTPStatus: http.StatusBadRequest}
}

func Unauthorized(code, path, message string) *AppError {
	return &AppError{Code: code, Path: path, Message: message, HTTPStatus: http.StatusUnauthorized}
}

func NotFound(code, path, message string) *AppError {
	return &AppError{Code: code, Path: path, Message: message, HTTPStatus: http.StatusNotFound}
}

func Internal(code, path, message string) *AppError {
	return &AppError{Code: code, Path: path, Message: message, HTTPStatus: http.StatusInternalServerError}
}

func AsAppError(err error) *AppError {
	if err == nil {
		return nil
	}
	var appErr *AppError
	if errors.As(err, &appErr) {
		if appErr.HTTPStatus == 0 {
			appErr.HTTPStatus = http.StatusInternalServerError
		}
		return appErr
	}
	return Internal("INTERNAL_ERROR", "", err.Error())
}
