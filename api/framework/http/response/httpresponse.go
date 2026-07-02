package httpresponse

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

type InternalErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type InternalErrorEnvelope struct {
	Success bool              `json:"success"`
	Error   InternalErrorBody `json:"error"`
}

type InternalSuccessEnvelope struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data"`
}

type MessageBody struct {
	Message string `json:"message"`
}

func Error(c echo.Context, status int, message string) error {
	return ErrorWithCode(c, status, statusErrorCode(status), message)
}

func ErrorWithCode(c echo.Context, status int, code string, message string) error {
	return c.JSON(status, InternalErrorEnvelope{
		Success: false,
		Error: InternalErrorBody{
			Code:    code,
			Message: message,
		},
	})
}

func Message(c echo.Context, status int, message string) error {
	return Success(c, status, MessageBody{Message: message})
}

func Success(c echo.Context, status int, data interface{}) error {
	if data == nil {
		data = struct{}{}
	}
	return c.JSON(status, InternalSuccessEnvelope{
		Success: true,
		Data:    data,
	})
}

func OK(c echo.Context, data interface{}) error {
	return Success(c, http.StatusOK, data)
}

func Created(c echo.Context, data interface{}) error {
	return Success(c, http.StatusCreated, data)
}

func OKEmpty(c echo.Context) error {
	return Success(c, http.StatusOK, struct{}{})
}

func BadRequest(c echo.Context, message string) error {
	return ErrorWithCode(c, http.StatusBadRequest, "validation", message)
}

func Unauthorized(c echo.Context, message string) error {
	return ErrorWithCode(c, http.StatusUnauthorized, "unauthorized", message)
}

func Forbidden(c echo.Context, message string) error {
	return ErrorWithCode(c, http.StatusForbidden, "forbidden", message)
}

func OKMessage(c echo.Context, message string) error {
	return Message(c, http.StatusOK, message)
}

func statusErrorCode(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "validation"
	case http.StatusUnauthorized:
		return "unauthorized"
	case http.StatusForbidden:
		return "forbidden"
	case http.StatusNotFound:
		return "not_found"
	case http.StatusConflict:
		return "conflict"
	default:
		return "internal_error"
	}
}
