package usecase_test

import (
	"context"
	"testing"

	"github.com/tfnick/go-svelte-starter/api/db"
	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
	"github.com/tfnick/go-svelte-starter/api/usecase"
)

func TestReloadSharedDBReturnsInternalErrorWhenSharedDBMissing(t *testing.T) {
	previous := db.DefaultManager
	db.DefaultManager = db.NewDBManager()
	t.Cleanup(func() {
		db.DefaultManager = previous
	})

	ctx := fwusecase.NewContext(context.Background(), fwusecase.SurfaceSystem)
	err := usecase.ReloadSharedDB(ctx, usecase.ReloadSharedDBCmd{})
	if err == nil {
		t.Fatal("expected error")
	}
	if fwusecase.CodeOf(err) != fwusecase.CodeInternal {
		t.Fatalf("expected internal error code, got %s", fwusecase.CodeOf(err))
	}
	if fwusecase.MessageOf(err, "") != "failed to reload shared database" {
		t.Fatalf("unexpected message: %q", fwusecase.MessageOf(err, ""))
	}
}
