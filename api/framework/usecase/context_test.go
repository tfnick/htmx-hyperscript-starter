package usecase_test

import (
	"context"
	"testing"

	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
)

func TestContextKeepsStandardContextAndCallerMetadata(t *testing.T) {
	base := context.WithValue(context.Background(), "request", "req-1")
	callCtx := fwusecase.NewContext(base, fwusecase.SurfaceOpenAPI)
	callCtx.RequestID = "req-1"
	callCtx.Consumer = fwusecase.ConsumerContext{
		Authenticated: true,
		AccountID:     "account-1",
		Scopes:        []string{"account:read"},
	}

	if callCtx.Std().Value("request") != "req-1" {
		t.Fatalf("expected standard context to be preserved")
	}
	if callCtx.Surface != fwusecase.SurfaceOpenAPI || callCtx.RequestID != "req-1" {
		t.Fatalf("expected surface and request ID metadata, got %#v", callCtx)
	}
	if !callCtx.HasConsumerScope("account:read") || callCtx.HasConsumerScope("orders:write") {
		t.Fatalf("expected scope helper to inspect consumer scopes")
	}
}

func TestContextUsesBackgroundWhenNil(t *testing.T) {
	callCtx := fwusecase.NewContext(nil, fwusecase.SurfaceInternalAPI)
	if callCtx.Std() == nil {
		t.Fatalf("expected non-nil standard context")
	}
}
