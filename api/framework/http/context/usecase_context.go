package fwcontext

import (
	"github.com/labstack/echo/v4"
	"github.com/tfnick/go-svelte-starter/api/framework/authz"
	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
)

func InternalUsecaseContext(c echo.Context) fwusecase.Context {
	callCtx := fwusecase.NewContext(c.Request().Context(), fwusecase.SurfaceInternalAPI)
	callCtx.RequestID = GetRequestID(c)

	if user := GetCurrentUser(c); user != nil {
		role := authz.NormalizeRole(user.Role, user.IsAdmin == 1)
		callCtx.Actor = fwusecase.ActorContext{
			Authenticated:  true,
			UserID:         user.ID,
			Name:           user.Name,
			Email:          user.Email,
			IsAdmin:        user.IsAdmin == 1,
			Role:           role,
			OrganizationID: user.OrganizationID,
			Permissions:    authz.PermissionsForRole(role),
			DataScope:      authz.DataScopeForRole(role),
		}
	}

	return callCtx
}

func OpenAPIUsecaseContext(c echo.Context) fwusecase.Context {
	callCtx := fwusecase.NewContext(c.Request().Context(), fwusecase.SurfaceOpenAPI)
	callCtx.RequestID = GetRequestID(c)

	if consumer := GetOpenAPIConsumer(c); consumer != nil {
		callCtx.Consumer = fwusecase.ConsumerContext{
			Authenticated: true,
			KeyID:         consumer.KeyID,
			PartnerID:     consumer.PartnerID,
			AccountID:     consumer.AccountID,
			Environment:   consumer.Environment,
			Scopes:        append([]string(nil), consumer.Scopes...),
		}
	}

	return callCtx
}
