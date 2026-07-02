package routes

import (
	"github.com/labstack/echo/v4"
	fwcontext "github.com/tfnick/go-svelte-starter/api/framework/http/context"
	httpresponse "github.com/tfnick/go-svelte-starter/api/framework/http/response"
	"github.com/tfnick/go-svelte-starter/api/usecase"
)

// --- Partner routes ---

type CreateOpenAPIPartnerRequest struct {
	Name      string `json:"name"`
	AccountID string `json:"account_id"`
}

type OpenAPIPartnerResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	AccountID string `json:"account_id"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at,omitempty"`
}

func toPartnerResponse(p usecase.OpenAPIPartnerCo) OpenAPIPartnerResponse {
	return OpenAPIPartnerResponse{
		ID:        p.ID,
		Name:      p.Name,
		AccountID: p.AccountID,
		Status:    p.Status,
		CreatedAt: p.CreatedAt,
	}
}

func CreateOpenAPIPartner(c echo.Context) error {
	var req CreateOpenAPIPartnerRequest
	if err := c.Bind(&req); err != nil {
		return httpresponse.BadRequest(c, "invalid request data")
	}

	ctx := fwcontext.InternalUsecaseContext(c)
	partner, err := usecase.CreateOpenAPIPartner(ctx, usecase.CreateOpenAPIPartnerCmd{
		Name:      req.Name,
		AccountID: req.AccountID,
	})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}

	return httpresponse.Created(c, toPartnerResponse(partner))
}

func ListOpenAPIPartners(c echo.Context) error {
	ctx := fwcontext.InternalUsecaseContext(c)
	partners, err := usecase.ListOpenAPIPartners(ctx, usecase.ListOpenAPIPartnersQry{})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}

	result := make([]OpenAPIPartnerResponse, 0, len(partners))
	for i := range partners {
		result = append(result, toPartnerResponse(partners[i]))
	}
	return httpresponse.OK(c, result)
}

// --- Key routes ---

type CreateOpenAPIKeyRequest struct {
	Environment string `json:"environment"`
}

type CreateOpenAPIKeyResponse struct {
	ID          string `json:"id"`
	PartnerID   string `json:"partner_id"`
	RawToken    string `json:"raw_token"`
	Environment string `json:"environment"`
	Scopes      string `json:"scopes"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at,omitempty"`
}

type OpenAPIKeyResponse struct {
	ID          string `json:"id"`
	PartnerID   string `json:"partner_id"`
	Environment string `json:"environment"`
	Scopes      string `json:"scopes"`
	Status      string `json:"status"`
}

type OpenAPIKeyWithPartnerResponse struct {
	ID          string `json:"id"`
	PartnerID   string `json:"partner_id"`
	PartnerName string `json:"partner_name"`
	Environment string `json:"environment"`
	Scopes      string `json:"scopes"`
	Status      string `json:"status"`
	RevokedAt   string `json:"revoked_at,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
}

func CreateOpenAPIKey(c echo.Context) error {
	partnerID := c.Param("id")
	var req CreateOpenAPIKeyRequest
	if err := c.Bind(&req); err != nil {
		return httpresponse.BadRequest(c, "invalid request data")
	}

	ctx := fwcontext.InternalUsecaseContext(c)
	key, err := usecase.CreateOpenAPIKey(ctx, usecase.CreateOpenAPIKeyCmd{
		PartnerID:   partnerID,
		Environment: req.Environment,
	})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}

	return httpresponse.Created(c, CreateOpenAPIKeyResponse{
		ID:          key.ID,
		PartnerID:   key.PartnerID,
		RawToken:    key.RawToken,
		Environment: key.Environment,
		Scopes:      key.Scopes,
		Status:      key.Status,
		CreatedAt:   key.CreatedAt,
	})
}

func ListOpenAPIKeysByPartnerID(c echo.Context) error {
	partnerID := c.Param("id")
	ctx := fwcontext.InternalUsecaseContext(c)
	keys, err := usecase.ListOpenAPIKeysByPartnerID(ctx, usecase.ListOpenAPIKeysByPartnerQry{PartnerID: partnerID})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}

	result := make([]OpenAPIKeyResponse, 0, len(keys))
	for i := range keys {
		k := keys[i]
		result = append(result, OpenAPIKeyResponse{
			ID:          k.ID,
			PartnerID:   k.PartnerID,
			Environment: k.Environment,
			Scopes:      k.Scopes,
			Status:      k.Status,
		})
	}
	return httpresponse.OK(c, result)
}

func RevokeOpenAPIKey(c echo.Context) error {
	keyID := c.Param("id")
	ctx := fwcontext.InternalUsecaseContext(c)
	if err := usecase.RevokeOpenAPIKey(ctx, usecase.RevokeOpenAPIKeyCmd{KeyID: keyID}); err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.OKEmpty(c)
}

func ListAllOpenAPIKeys(c echo.Context) error {
	ctx := fwcontext.InternalUsecaseContext(c)
	keys, err := usecase.ListOpenAPIKeys(ctx, usecase.ListOpenAPIKeysQry{})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}

	result := make([]OpenAPIKeyWithPartnerResponse, 0, len(keys))
	for i := range keys {
		k := keys[i]
		result = append(result, OpenAPIKeyWithPartnerResponse{
			ID:          k.ID,
			PartnerID:   k.PartnerID,
			PartnerName: k.PartnerName,
			Environment: k.Environment,
			Scopes:      k.Scopes,
			Status:      k.Status,
			RevokedAt:   k.RevokedAt,
			CreatedAt:   k.CreatedAt,
		})
	}
	return httpresponse.OK(c, result)
}
