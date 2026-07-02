package usecase

import (
	"github.com/tfnick/go-svelte-starter/api/framework/database"
	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
)

type ReloadSharedDBCmd struct{}

func ReloadSharedDB(ctx fwusecase.Context, _ ReloadSharedDBCmd) error {
	if err := database.Reopen("shared"); err != nil {
		return fwusecase.E(fwusecase.CodeInternal, "failed to reload shared database", err)
	}
	return nil
}
