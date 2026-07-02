package routes

import (
	"net/http"

	"github.com/labstack/echo/v4"
	fwcontext "github.com/tfnick/go-svelte-starter/api/framework/http/context"
	"github.com/tfnick/go-svelte-starter/api/framework/http/middleware"
	httpresponse "github.com/tfnick/go-svelte-starter/api/framework/http/response"
	"github.com/tfnick/go-svelte-starter/api/usecase"
)

type OpenAPIAccountResponse struct {
	ID            string `json:"id"`
	ExternalRef   string `json:"external_ref,omitempty"`
	Name          string `json:"name"`
	Email         string `json:"email,omitempty"`
	Status        string `json:"status"`
	EmailVerified bool   `json:"email_verified"`
	CreatedAt     string `json:"created_at,omitempty"`
}

type OpenAPIAccountEnvelope struct {
	Success bool                    `json:"success"`
	Data    OpenAPIAccountResponse  `json:"data,omitempty"`
	Error   *httpresponse.ErrorBody `json:"error,omitempty"`
}

func ToOpenAPIAccountResponse(account usecase.OpenAPIAccountCo) OpenAPIAccountResponse {
	return OpenAPIAccountResponse{
		ID:            account.ID,
		ExternalRef:   account.ExternalRef,
		Name:          account.Name,
		Email:         account.Email,
		Status:        account.Status,
		EmailVerified: account.EmailVerified,
		CreatedAt:     account.CreatedAt,
	}
}

func ToOpenAPIAccountEnvelope(account usecase.OpenAPIAccountCo) OpenAPIAccountEnvelope {
	return OpenAPIAccountEnvelope{
		Success: true,
		Data:    ToOpenAPIAccountResponse(account),
	}
}

func GetOpenAPIAccountMe(c echo.Context) error {
	consumer := middleware.GetOpenAPIConsumer(c)
	if consumer == nil {
		return c.JSON(http.StatusUnauthorized, httpresponse.ErrorResponse("unauthorized", "missing consumer context"))
	}

	ctx := fwcontext.OpenAPIUsecaseContext(c)
	account, err := usecase.GetOpenAPIAccount(ctx, usecase.OpenAPIAccountQry{AccountID: consumer.AccountID})
	if err != nil {
		return httpresponse.OpenAPIUsecaseError(c, err)
	}

	return c.JSON(http.StatusOK, ToOpenAPIAccountEnvelope(account))
}
