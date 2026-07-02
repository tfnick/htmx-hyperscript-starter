package routes

import (
	"github.com/labstack/echo/v4"
	fwcontext "github.com/tfnick/go-svelte-starter/api/framework/http/context"
	httpresponse "github.com/tfnick/go-svelte-starter/api/framework/http/response"
	"github.com/tfnick/go-svelte-starter/api/usecase"
)

type SaveProductRequest struct {
	Name                 string `json:"name"`
	Description          string `json:"description"`
	Price                int64  `json:"price"`
	Currency             string `json:"currency"`
	Enabled              bool   `json:"enabled"`
	CreemProductID       string `json:"creem_product_id"`
	BillingType          string `json:"billing_type"`
	MembershipLevel      string `json:"membership_level"`
	SubscriptionInterval string `json:"subscription_interval"`
}

type ProductResponse struct {
	ID                   string `json:"id"`
	Name                 string `json:"name"`
	Description          string `json:"description,omitempty"`
	Price                int64  `json:"price"`
	Currency             string `json:"currency"`
	Stock                int    `json:"stock"`
	Enabled              bool   `json:"enabled"`
	CreemProductID       string `json:"creem_product_id"`
	BillingType          string `json:"billing_type"`
	MembershipLevel      string `json:"membership_level"`
	SubscriptionInterval string `json:"subscription_interval"`
	CreatedAt            string `json:"created_at,omitempty"`
	UpdatedAt            string `json:"updated_at,omitempty"`
}

type ProductMutationResponse struct {
	Message string          `json:"message"`
	Product ProductResponse `json:"product"`
}

func ToProductResponse(product usecase.ProductCo) ProductResponse {
	return ProductResponse{
		ID:                   product.ID,
		Name:                 product.Name,
		Description:          product.Description,
		Price:                product.Price,
		Currency:             product.Currency,
		Stock:                product.Stock,
		Enabled:              product.Enabled,
		CreemProductID:       product.CreemProductID,
		BillingType:          product.BillingType,
		MembershipLevel:      product.MembershipLevel,
		SubscriptionInterval: product.SubscriptionInterval,
		CreatedAt:            product.CreatedAt,
		UpdatedAt:            product.UpdatedAt,
	}
}

func ToProductResponses(products []usecase.ProductCo) []ProductResponse {
	responses := make([]ProductResponse, 0, len(products))
	for i := range products {
		responses = append(responses, ToProductResponse(products[i]))
	}
	return responses
}

func ListProducts(c echo.Context) error {
	ctx := fwcontext.InternalUsecaseContext(c)
	products, err := usecase.ListProducts(ctx)
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.OK(c, ToProductResponses(products))
}

func CreateProduct(c echo.Context) error {
	var req SaveProductRequest
	if err := c.Bind(&req); err != nil {
		return httpresponse.BadRequest(c, "invalid request data")
	}

	ctx := fwcontext.InternalUsecaseContext(c)
	product, err := usecase.CreateProduct(ctx, saveProductCmdFromRequest("", req))
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.Created(c, ProductMutationResponse{
		Message: "product created",
		Product: ToProductResponse(product),
	})
}

func UpdateProduct(c echo.Context) error {
	var req SaveProductRequest
	if err := c.Bind(&req); err != nil {
		return httpresponse.BadRequest(c, "invalid request data")
	}

	ctx := fwcontext.InternalUsecaseContext(c)
	product, err := usecase.UpdateProduct(ctx, saveProductCmdFromRequest(c.Param("id"), req))
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.OK(c, ProductMutationResponse{
		Message: "product updated",
		Product: ToProductResponse(product),
	})
}

func saveProductCmdFromRequest(id string, req SaveProductRequest) usecase.SaveProductCmd {
	return usecase.SaveProductCmd{
		ID:                   id,
		Name:                 req.Name,
		Description:          req.Description,
		Price:                req.Price,
		Currency:             req.Currency,
		Enabled:              req.Enabled,
		CreemProductID:       req.CreemProductID,
		BillingType:          req.BillingType,
		MembershipLevel:      req.MembershipLevel,
		SubscriptionInterval: req.SubscriptionInterval,
	}
}
