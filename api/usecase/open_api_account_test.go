package usecase_test

import (
	"testing"

	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
	"github.com/tfnick/go-svelte-starter/api/usecase"
)

func TestGetOpenAPIAccountRequiresMatchingConsumerContext(t *testing.T) {
	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceOpenAPI)
	ctx.Consumer = fwusecase.ConsumerContext{
		Authenticated: true,
		AccountID:     "account-1",
	}

	_, err := usecase.GetOpenAPIAccount(ctx, usecase.OpenAPIAccountQry{AccountID: "account-2"})
	if err == nil {
		t.Fatalf("expected account context mismatch error")
	}
	if fwusecase.CodeOf(err) != fwusecase.CodeForbidden {
		t.Fatalf("expected forbidden error, got %q", fwusecase.CodeOf(err))
	}
}

func TestGetOpenAPIAccountRequiresAuthenticatedConsumerForOpenAPI(t *testing.T) {
	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceOpenAPI)

	_, err := usecase.GetOpenAPIAccount(ctx, usecase.OpenAPIAccountQry{AccountID: "account-1"})
	if err == nil {
		t.Fatalf("expected missing consumer context error")
	}
	if fwusecase.CodeOf(err) != fwusecase.CodeUnauthorized {
		t.Fatalf("expected unauthorized error, got %q", fwusecase.CodeOf(err))
	}
}
