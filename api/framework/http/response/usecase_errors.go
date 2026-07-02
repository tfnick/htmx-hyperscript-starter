package httpresponse

import (
	"net/http"

	"github.com/labstack/echo/v4"
	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
)

func InternalUsecaseError(c echo.Context, err error) error {
	message := fwusecase.MessageOf(err, "internal error")

	switch fwusecase.CodeOf(err) {
	case fwusecase.CodeValidation:
		return ErrorWithCode(c, http.StatusBadRequest, "validation", message)
	case fwusecase.CodeUnauthorized:
		return ErrorWithCode(c, http.StatusUnauthorized, "unauthorized", message)
	case fwusecase.CodeForbidden:
		return ErrorWithCode(c, http.StatusForbidden, "forbidden", message)
	case fwusecase.CodeNotFound:
		return ErrorWithCode(c, http.StatusNotFound, "not_found", message)
	case fwusecase.CodeConflict:
		return ErrorWithCode(c, http.StatusConflict, "conflict", message)
	default:
		return InternalServerError(c, fwusecase.LogErrorOf(err), message)
	}
}

func OpenAPIUsecaseError(c echo.Context, err error) error {
	status, code := openAPIStatusAndCode(fwusecase.CodeOf(err))
	return c.JSON(status, ErrorResponse(code, fwusecase.MessageOf(err, "internal error")))
}

func openAPIStatusAndCode(code fwusecase.ErrorCode) (int, string) {
	switch code {
	case fwusecase.CodeValidation:
		return http.StatusBadRequest, "validation"
	case fwusecase.CodeUnauthorized:
		return http.StatusUnauthorized, "unauthorized"
	case fwusecase.CodeForbidden:
		return http.StatusForbidden, "forbidden"
	case fwusecase.CodeNotFound:
		return http.StatusNotFound, "not_found"
	case fwusecase.CodeConflict:
		return http.StatusConflict, "conflict"
	default:
		return http.StatusInternalServerError, "internal_error"
	}
}
