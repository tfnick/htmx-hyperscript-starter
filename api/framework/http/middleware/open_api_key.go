package middleware

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	fwcontext "github.com/tfnick/go-svelte-starter/api/framework/http/context"
	httpresponse "github.com/tfnick/go-svelte-starter/api/framework/http/response"
	"github.com/tfnick/go-svelte-starter/api/models"
)

const OpenAPIConsumerContextKey = fwcontext.OpenAPIConsumerContextKey

type OpenAPIConsumerContext = fwcontext.OpenAPIConsumerContext

func RequireOpenAPIKey() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			apiKey := readAPIKey(c)
			if apiKey == "" {
				return c.JSON(http.StatusUnauthorized, httpresponse.ErrorResponse("unauthorized", "missing api key"))
			}

			consumer, err := models.ResolveOpenAPIConsumer(c.Request().Context(), apiKey)
			if err != nil {
				return c.JSON(http.StatusUnauthorized, httpresponse.ErrorResponse("unauthorized", "invalid api key"))
			}

			fwcontext.SetOpenAPIConsumer(c, &OpenAPIConsumerContext{
				KeyID:       consumer.KeyID,
				PartnerID:   consumer.PartnerID,
				AccountID:   consumer.AccountID,
				Scopes:      consumer.Scopes,
				Environment: consumer.Environment,
			})

			return next(c)
		}
	}
}

func GetOpenAPIConsumer(c echo.Context) *OpenAPIConsumerContext {
	return fwcontext.GetOpenAPIConsumer(c)
}

func readAPIKey(c echo.Context) string {
	auth := c.Request().Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
	}
	return strings.TrimSpace(c.Request().Header.Get("X-API-Key"))
}
