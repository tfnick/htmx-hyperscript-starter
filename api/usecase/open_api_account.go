package usecase

import (
	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
	"github.com/tfnick/go-svelte-starter/api/models"
)

type OpenAPIAccountQry struct {
	AccountID string
}

type OpenAPIAccountCo struct {
	ID            string
	ExternalRef   string
	Name          string
	Email         string
	Status        string
	EmailVerified bool
	CreatedAt     string
}

func GetOpenAPIAccount(ctx fwusecase.Context, qry OpenAPIAccountQry) (OpenAPIAccountCo, error) {
	if qry.AccountID == "" {
		return OpenAPIAccountCo{}, fwusecase.E(fwusecase.CodeUnauthorized, "missing consumer context", nil)
	}
	if ctx.Surface == fwusecase.SurfaceOpenAPI && !ctx.Consumer.Authenticated {
		return OpenAPIAccountCo{}, fwusecase.E(fwusecase.CodeUnauthorized, "missing consumer context", nil)
	}
	if ctx.Surface == fwusecase.SurfaceOpenAPI && qry.AccountID != ctx.Consumer.AccountID {
		return OpenAPIAccountCo{}, fwusecase.E(fwusecase.CodeForbidden, "account context mismatch", nil)
	}

	account, err := models.GetOpenAPIAccountByConsumerAccountID(ctx.Std(), qry.AccountID)
	if err != nil {
		return OpenAPIAccountCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to load account", err)
	}
	if account == nil {
		return OpenAPIAccountCo{}, fwusecase.E(fwusecase.CodeNotFound, "account not found", nil)
	}

	return OpenAPIAccountCo{
		ID:            account.AccountID,
		ExternalRef:   account.ExternalRef,
		Name:          account.Name,
		Email:         account.Email,
		Status:        openAPIAccountStatus(account.IsActive),
		EmailVerified: account.EmailVerified == 1,
		CreatedAt:     account.CreatedAt,
	}, nil
}

func openAPIAccountStatus(isActive int) string {
	if isActive == 1 {
		return "active"
	}
	return "inactive"
}
