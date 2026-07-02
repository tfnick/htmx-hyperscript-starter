package routes

import (
	"github.com/labstack/echo/v4"
	fwcontext "github.com/tfnick/go-svelte-starter/api/framework/http/context"
	"github.com/tfnick/go-svelte-starter/api/framework/http/middleware"
	httpresponse "github.com/tfnick/go-svelte-starter/api/framework/http/response"
	"github.com/tfnick/go-svelte-starter/api/usecase"
)

func TriggerExportToast(c echo.Context) error {
	currentUser := middleware.GetCurrentUser(c)
	if currentUser == nil {
		return httpresponse.Unauthorized(c, "not logged in")
	}

	ctx := fwcontext.InternalUsecaseContext(c)
	if err := usecase.TriggerExportToast(ctx, usecase.TriggerExportToastCmd{UserID: currentUser.ID}); err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.OKMessage(c, "export notification sent")
}
