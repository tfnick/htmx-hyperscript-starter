package fwcontext

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/tfnick/go-svelte-starter/api/framework/authz"
	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
	"github.com/tfnick/go-svelte-starter/api/models"
)

func TestInternalUsecaseContextCarriesAuthenticatedActor(t *testing.T) {
	router := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	c := router.NewContext(req, httptest.NewRecorder())
	c.Set(RequestIDContextKey, "req-1")
	SetCurrentUser(c, &models.User{
		ID:    "user-1",
		Name:  "Ada",
		Email: "ada@example.com",
		Role:  authz.RolePlatformAdmin,
	})

	ctx := InternalUsecaseContext(c)

	if ctx.Surface != fwusecase.SurfaceInternalAPI {
		t.Fatalf("expected internal API surface, got %q", ctx.Surface)
	}
	if ctx.RequestID != "req-1" {
		t.Fatalf("expected request id, got %q", ctx.RequestID)
	}
	if !ctx.Actor.Authenticated || ctx.Actor.UserID != "user-1" {
		t.Fatalf("expected authenticated actor, got %#v", ctx.Actor)
	}
	if ctx.Actor.Role != authz.RolePlatformAdmin || ctx.Actor.DataScope != authz.DataScopePlatform {
		t.Fatalf("expected platform actor scope, got %#v", ctx.Actor)
	}
	if !ctx.HasActorPermission(authz.PermissionSettingsManage) {
		t.Fatalf("expected settings management permission")
	}
	if ctx.Consumer.Authenticated {
		t.Fatalf("did not expect OpenAPI consumer in internal context")
	}
}

func TestOpenAPIUsecaseContextCarriesConsumer(t *testing.T) {
	router := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/open-api/v1/account/me", nil)
	c := router.NewContext(req, httptest.NewRecorder())
	SetOpenAPIConsumer(c, &OpenAPIConsumerContext{
		KeyID:       "key-1",
		PartnerID:   "partner-1",
		AccountID:   "account-1",
		Environment: "test",
		Scopes:      []string{"account:read"},
	})

	ctx := OpenAPIUsecaseContext(c)

	if ctx.Surface != fwusecase.SurfaceOpenAPI {
		t.Fatalf("expected OpenAPI surface, got %q", ctx.Surface)
	}
	if !ctx.Consumer.Authenticated || ctx.Consumer.AccountID != "account-1" {
		t.Fatalf("expected authenticated consumer, got %#v", ctx.Consumer)
	}
	if ctx.Actor.Authenticated {
		t.Fatalf("did not expect internal actor in OpenAPI context")
	}
	if !ctx.HasConsumerScope("account:read") {
		t.Fatalf("expected account read scope")
	}
}
