package routes

import (
	"github.com/labstack/echo/v4"
	fwcontext "github.com/tfnick/go-svelte-starter/api/framework/http/context"
	httpresponse "github.com/tfnick/go-svelte-starter/api/framework/http/response"
	"github.com/tfnick/go-svelte-starter/api/usecase"
)

type marketingChannelsResponse struct {
	Success bool                         `json:"success"`
	Data    *usecase.MarketingChannelsCo `json:"data,omitempty"`
	Error   *httpresponse.ErrorBody      `json:"error,omitempty"`
}

func GetMarketingChannels(c echo.Context) error {
	ctx := fwcontext.OpenAPIUsecaseContext(c)

	result, err := usecase.GetMarketingChannels(ctx)
	if err != nil {
		return httpresponse.OpenAPIUsecaseError(c, err)
	}

	return c.JSON(200, marketingChannelsResponse{
		Success: true,
		Data:    &result,
	})
}
