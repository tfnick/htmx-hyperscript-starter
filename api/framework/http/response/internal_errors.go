package httpresponse

import (
	"net/http"

	"github.com/labstack/echo/v4"
	fwcontext "github.com/tfnick/go-svelte-starter/api/framework/http/context"
	"github.com/tfnick/go-svelte-starter/api/framework/logging"
)

var internalAPILogger = logging.For("http")

func InternalServerError(c echo.Context, err error, message string) error {
	logInternalAPIError(c, err, message)
	return Error(c, http.StatusInternalServerError, message)
}

func NotFoundError(c echo.Context, err error, message string) error {
	logInternalAPIError(c, err, message)
	return Error(c, http.StatusNotFound, message)
}

func logInternalAPIError(c echo.Context, err error, message string) {
	event := internalAPILogger.Error().
		Err(err).
		Str("surface", "api").
		Str("method", c.Request().Method).
		Str("route", routePath(c)).
		Str("path", c.Request().URL.Path).
		Str("client_error", message)

	if requestID := fwcontext.GetRequestID(c); requestID != "" {
		event.Str("request_id", requestID)
	}

	event.Msg("internal api handler error")
}

func routePath(c echo.Context) string {
	if c.Path() != "" {
		return c.Path()
	}
	return c.Request().URL.Path
}
