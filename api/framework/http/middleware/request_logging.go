package middleware

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
	fwcontext "github.com/tfnick/go-svelte-starter/api/framework/http/context"
	"github.com/tfnick/go-svelte-starter/api/framework/logging"
)

var requestLogger = logging.For("http")

func RequestLogger(surface string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			requestID := fwcontext.SetRequestID(c)

			start := time.Now()
			err := next(c)
			if err != nil {
				c.Error(err)
			}

			event := requestLogger.Info()
			status := responseStatus(c)
			if err != nil || status >= http.StatusInternalServerError {
				event = requestLogger.Error()
			}
			if err != nil {
				event.Err(err)
			}

			addOpenAPIFields(event, surface, c)
			event.
				Str("surface", surface).
				Str("request_id", requestID).
				Str("method", c.Request().Method).
				Str("route", routePath(c)).
				Str("path", c.Request().URL.Path).
				Int("status", status).
				Dur("duration", time.Since(start)).
				Msg("request completed")

			return nil
		}
	}
}

func routePath(c echo.Context) string {
	path := c.Path()
	if path != "" {
		return path
	}
	return c.Request().URL.Path
}

func responseStatus(c echo.Context) int {
	status := c.Response().Status
	if status == 0 {
		return http.StatusOK
	}
	return status
}

func addOpenAPIFields(event *zerolog.Event, surface string, c echo.Context) {
	if surface != "open-api" {
		return
	}

	consumer := fwcontext.GetOpenAPIConsumer(c)
	if consumer == nil {
		return
	}

	event.
		Str("partner_id", consumer.PartnerID).
		Str("account_id", consumer.AccountID).
		Str("environment", consumer.Environment)
}
