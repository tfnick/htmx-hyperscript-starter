package usecase

import (
	"database/sql"
	"errors"
	"strings"

	"github.com/tfnick/go-svelte-starter/api/framework/data/modelerror"
	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
	"github.com/tfnick/go-svelte-starter/api/models"
)

const (
	MembershipLevelBasic   = "basic"
	MembershipLevelPremium = "premium"
	MembershipLevelSuper   = "super"

	ProductBillingTypeOneTime      = "one_time"
	ProductBillingTypeSubscription = "subscription"

	SubscriptionIntervalMonth       = "month"
	SubscriptionIntervalThreeMonths = "three_months"
	SubscriptionIntervalSixMonths   = "six_months"
	SubscriptionIntervalYear        = "year"

	PermanentMembershipExpiresAt = "2099-12-31 23:59:59"
)

type ProductCo struct {
	ID                   string
	Name                 string
	Description          string
	Price                int64
	Currency             string
	Stock                int
	Enabled              bool
	CreemProductID       string
	BillingType          string
	MembershipLevel      string
	SubscriptionInterval string
	CreatedAt            string
	UpdatedAt            string
}

type SaveProductCmd struct {
	ID                   string
	Name                 string
	Description          string
	Price                int64
	Currency             string
	Enabled              bool
	CreemProductID       string
	BillingType          string
	MembershipLevel      string
	SubscriptionInterval string
}

func ListProducts(ctx fwusecase.Context) ([]ProductCo, error) {
	products, err := models.ListProducts(ctx.Std())
	if err != nil {
		return nil, fwusecase.E(fwusecase.CodeInternal, "failed to load products", err)
	}

	result := make([]ProductCo, 0, len(products))
	for i := range products {
		result = append(result, productCoFromModel(&products[i]))
	}
	return result, nil
}

func CreateProduct(ctx fwusecase.Context, cmd SaveProductCmd) (ProductCo, error) {
	product, err := productModelFromSaveCmd(cmd)
	if err != nil {
		return ProductCo{}, err
	}

	if err := models.CreateProduct(ctx.Std(), product); err != nil {
		return ProductCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to create product", err)
	}

	created, err := models.GetProductByID(ctx.Std(), product.ID)
	if err != nil {
		return ProductCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to load product", err)
	}
	return productCoFromModel(created), nil
}

func UpdateProduct(ctx fwusecase.Context, cmd SaveProductCmd) (ProductCo, error) {
	if strings.TrimSpace(cmd.ID) == "" {
		return ProductCo{}, fwusecase.E(fwusecase.CodeValidation, "product ID is required", nil)
	}

	product, err := productModelFromSaveCmd(cmd)
	if err != nil {
		return ProductCo{}, err
	}

	if err := models.UpdateProduct(ctx.Std(), product); err != nil {
		if errors.Is(err, modelerror.ErrNotFound) {
			return ProductCo{}, fwusecase.E(fwusecase.CodeNotFound, "product not found", err)
		}
		return ProductCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to update product", err)
	}

	updated, err := models.GetProductByID(ctx.Std(), product.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ProductCo{}, fwusecase.E(fwusecase.CodeNotFound, "product not found", err)
		}
		return ProductCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to load product", err)
	}
	return productCoFromModel(updated), nil
}

func productModelFromSaveCmd(cmd SaveProductCmd) (*models.Product, error) {
	name := strings.TrimSpace(cmd.Name)
	creemProductID := strings.TrimSpace(cmd.CreemProductID)
	billingType := strings.TrimSpace(cmd.BillingType)
	membershipLevel := strings.TrimSpace(cmd.MembershipLevel)
	subscriptionInterval := strings.TrimSpace(cmd.SubscriptionInterval)

	if name == "" {
		return nil, fwusecase.E(fwusecase.CodeValidation, "product name is required", nil)
	}
	if creemProductID == "" {
		return nil, fwusecase.E(fwusecase.CodeValidation, "Creem product ID is required", nil)
	}
	if cmd.Price < 0 {
		return nil, fwusecase.E(fwusecase.CodeValidation, "product price cannot be negative", nil)
	}
	if billingType == "" {
		billingType = ProductBillingTypeOneTime
	}
	if !isValidProductBillingType(billingType) {
		return nil, fwusecase.E(fwusecase.CodeValidation, "invalid product billing type", nil)
	}
	if membershipLevel == "" {
		membershipLevel = MembershipLevelBasic
	}
	if !isValidMembershipLevel(membershipLevel) {
		return nil, fwusecase.E(fwusecase.CodeValidation, "invalid membership level", nil)
	}
	if billingType == ProductBillingTypeSubscription {
		if !isValidSubscriptionInterval(subscriptionInterval) {
			return nil, fwusecase.E(fwusecase.CodeValidation, "invalid subscription interval", nil)
		}
	} else {
		subscriptionInterval = ""
	}

	enabled := 0
	if cmd.Enabled {
		enabled = 1
	}

	return &models.Product{
		ID:                   strings.TrimSpace(cmd.ID),
		Name:                 name,
		Description:          strings.TrimSpace(cmd.Description),
		Price:                cmd.Price,
		Currency:             strings.ToUpper(strings.TrimSpace(cmd.Currency)),
		Stock:                0,
		Enabled:              enabled,
		CreemProductID:       creemProductID,
		BillingType:          billingType,
		MembershipLevel:      membershipLevel,
		SubscriptionInterval: subscriptionInterval,
	}, nil
}

func isValidProductBillingType(value string) bool {
	switch value {
	case ProductBillingTypeOneTime, ProductBillingTypeSubscription:
		return true
	default:
		return false
	}
}

func isValidMembershipLevel(value string) bool {
	switch value {
	case MembershipLevelBasic, MembershipLevelPremium, MembershipLevelSuper:
		return true
	default:
		return false
	}
}

func isValidSubscriptionInterval(value string) bool {
	switch value {
	case SubscriptionIntervalMonth, SubscriptionIntervalThreeMonths, SubscriptionIntervalSixMonths, SubscriptionIntervalYear:
		return true
	default:
		return false
	}
}

func productCoFromModel(product *models.Product) ProductCo {
	if product == nil {
		return ProductCo{}
	}

	return ProductCo{
		ID:                   product.ID,
		Name:                 product.Name,
		Description:          product.Description,
		Price:                product.Price,
		Currency:             product.Currency,
		Stock:                product.Stock,
		Enabled:              product.Enabled == 1,
		CreemProductID:       product.CreemProductID,
		BillingType:          product.BillingType,
		MembershipLevel:      product.MembershipLevel,
		SubscriptionInterval: product.SubscriptionInterval,
		CreatedAt:            product.CreatedAt,
		UpdatedAt:            product.UpdatedAt,
	}
}
