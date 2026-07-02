package usecase

import (
	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
	"github.com/tfnick/go-svelte-starter/api/models"
)

// --- Partners ---

type CreateOpenAPIPartnerCmd struct {
	Name      string
	AccountID string
}

type OpenAPIPartnerCo struct {
	ID        string
	Name      string
	AccountID string
	Status    string
	CreatedAt string
}

type ListOpenAPIPartnersQry struct{}

func CreateOpenAPIPartner(ctx fwusecase.Context, cmd CreateOpenAPIPartnerCmd) (OpenAPIPartnerCo, error) {
	if cmd.Name == "" {
		return OpenAPIPartnerCo{}, fwusecase.E(fwusecase.CodeValidation, "partner name is required", nil)
	}
	if cmd.AccountID == "" {
		return OpenAPIPartnerCo{}, fwusecase.E(fwusecase.CodeValidation, "account_id is required", nil)
	}

	partner := &models.OpenAPIPartner{Name: cmd.Name, AccountID: cmd.AccountID}
	if err := models.CreateOpenAPIPartner(ctx.Std(), partner); err != nil {
		return OpenAPIPartnerCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to create partner", err)
	}

	return OpenAPIPartnerCo{
		ID:        partner.ID,
		Name:      partner.Name,
		AccountID: partner.AccountID,
		Status:    partner.Status,
		CreatedAt: partner.CreatedAt,
	}, nil
}

func ListOpenAPIPartners(ctx fwusecase.Context, _ ListOpenAPIPartnersQry) ([]OpenAPIPartnerCo, error) {
	partners, err := models.ListOpenAPIPartners(ctx.Std())
	if err != nil {
		return nil, fwusecase.E(fwusecase.CodeInternal, "failed to list partners", err)
	}

	result := make([]OpenAPIPartnerCo, 0, len(partners))
	for i := range partners {
		p := partners[i]
		result = append(result, OpenAPIPartnerCo{
			ID:        p.ID,
			Name:      p.Name,
			AccountID: p.AccountID,
			Status:    p.Status,
			CreatedAt: p.CreatedAt,
		})
	}
	return result, nil
}

// --- Keys ---

type CreateOpenAPIKeyCmd struct {
	PartnerID   string
	Environment string
}

type CreateOpenAPIKeyCo struct {
	ID          string
	PartnerID   string
	RawToken    string
	Environment string
	Scopes      string
	Status      string
	CreatedAt   string
}

type ListOpenAPIKeysByPartnerQry struct {
	PartnerID string
}

type OpenAPIKeyCo struct {
	ID          string
	PartnerID   string
	Environment string
	Scopes      string
	Status      string
}

type ListOpenAPIKeysQry struct{}

type OpenAPIKeyWithPartnerCo struct {
	ID          string
	PartnerID   string
	PartnerName string
	Environment string
	Scopes      string
	Status      string
	RevokedAt   string
	CreatedAt   string
}

type RevokeOpenAPIKeyCmd struct {
	KeyID string
}

func CreateOpenAPIKey(ctx fwusecase.Context, cmd CreateOpenAPIKeyCmd) (CreateOpenAPIKeyCo, error) {
	if cmd.PartnerID == "" {
		return CreateOpenAPIKeyCo{}, fwusecase.E(fwusecase.CodeValidation, "partner_id is required", nil)
	}
	if cmd.Environment == "" {
		cmd.Environment = "prod"
	}

	raw, hash, err := models.GenerateOpenAPIToken()
	if err != nil {
		return CreateOpenAPIKeyCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to generate token", err)
	}

	key, err := models.CreateOpenAPIKey(ctx.Std(), cmd.PartnerID, hash, cmd.Environment, "")
	if err != nil {
		return CreateOpenAPIKeyCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to create key", err)
	}

	return CreateOpenAPIKeyCo{
		ID:          key.ID,
		PartnerID:   key.PartnerID,
		RawToken:    raw,
		Environment: key.Environment,
		Scopes:      key.Scopes,
		Status:      key.Status,
		CreatedAt:   key.CreatedAt,
	}, nil
}

func ListOpenAPIKeysByPartnerID(ctx fwusecase.Context, qry ListOpenAPIKeysByPartnerQry) ([]OpenAPIKeyCo, error) {
	if qry.PartnerID == "" {
		return nil, fwusecase.E(fwusecase.CodeValidation, "partner_id is required", nil)
	}

	keys, err := models.ListOpenAPIKeysByPartnerID(ctx.Std(), qry.PartnerID)
	if err != nil {
		return nil, fwusecase.E(fwusecase.CodeInternal, "failed to list keys", err)
	}

	result := make([]OpenAPIKeyCo, 0, len(keys))
	for i := range keys {
		k := keys[i]
		result = append(result, OpenAPIKeyCo{
			ID:          k.ID,
			PartnerID:   k.PartnerID,
			Environment: k.Environment,
			Scopes:      k.Scopes,
			Status:      k.Status,
		})
	}
	return result, nil
}

func RevokeOpenAPIKey(ctx fwusecase.Context, cmd RevokeOpenAPIKeyCmd) error {
	if cmd.KeyID == "" {
		return fwusecase.E(fwusecase.CodeValidation, "key_id is required", nil)
	}
	if err := models.RevokeOpenAPIKey(ctx.Std(), cmd.KeyID); err != nil {
		return fwusecase.E(fwusecase.CodeInternal, "failed to revoke key", err)
	}
	return nil
}

func ListOpenAPIKeys(ctx fwusecase.Context, _ ListOpenAPIKeysQry) ([]OpenAPIKeyWithPartnerCo, error) {
	keys, err := models.ListOpenAPIKeys(ctx.Std())
	if err != nil {
		return nil, fwusecase.E(fwusecase.CodeInternal, "failed to list keys", err)
	}

	result := make([]OpenAPIKeyWithPartnerCo, 0, len(keys))
	for i := range keys {
		k := keys[i]
		result = append(result, OpenAPIKeyWithPartnerCo{
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
	return result, nil
}
