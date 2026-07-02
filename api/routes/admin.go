package routes

import (
	"github.com/labstack/echo/v4"
	fwcontext "github.com/tfnick/go-svelte-starter/api/framework/http/context"
	httpresponse "github.com/tfnick/go-svelte-starter/api/framework/http/response"
	"github.com/tfnick/go-svelte-starter/api/usecase"
)

// ReloadSharedDB reloads the shared database after its file is replaced.
func ReloadSharedDB(c echo.Context) error {
	ctx := fwcontext.InternalUsecaseContext(c)
	if err := usecase.ReloadSharedDB(ctx, usecase.ReloadSharedDBCmd{}); err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}

	return httpresponse.OKMessage(c, "shared database reloaded")
}
