package usecase_test

import (
	"testing"

	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
	"github.com/tfnick/go-svelte-starter/api/usecase"
)

func TestGetDictionariesLoadsEnabledDatabaseOptions(t *testing.T) {
	setupUsecaseOrderTxDB(t)
	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)

	batch, err := usecase.GetDictionaries(ctx, usecase.DictionaryBatchQry{
		Types: []string{"product_category", "product_category", "missing"},
	})
	if err != nil {
		t.Fatalf("get dictionaries: %v", err)
	}
	if len(batch.Dictionaries["product_category"]) != 4 {
		t.Fatalf("expected seeded product categories, got %#v", batch.Dictionaries["product_category"])
	}
	if len(batch.Dictionaries["missing"]) != 0 {
		t.Fatalf("expected empty missing dictionary, got %#v", batch.Dictionaries["missing"])
	}
}

func TestGetDictionariesLoadsIntegrationCredentialTypes(t *testing.T) {
	setupUsecaseOrderTxDB(t)
	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)

	batch, err := usecase.GetDictionaries(ctx, usecase.DictionaryBatchQry{
		Types: []string{"integration_credential_type"},
	})
	if err != nil {
		t.Fatalf("get dictionaries: %v", err)
	}
	values := batch.Dictionaries["integration_credential_type"]
	if len(values) != 6 {
		t.Fatalf("expected seeded integration credential types, got %#v", values)
	}
	if values[0].Value != "none" || values[1].Value != "payment_bundle" || values[2].Value != "api_key" || values[3].Value != "smtp_password" || values[4].Value != "s3_access_key" || values[5].Value != "webhook_url" {
		t.Fatalf("unexpected integration credential type ordering: %#v", values)
	}
}

func TestDictionaryTypeValidationAndConflict(t *testing.T) {
	setupUsecaseOrderTxDB(t)
	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)

	dictionaryType, err := usecase.CreateDictionaryType(ctx, usecase.SaveDictionaryTypeCmd{
		TypeKey: "Region",
		Name:    "Region",
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("create dictionary type: %v", err)
	}
	if dictionaryType.TypeKey != "region" {
		t.Fatalf("expected normalized type key, got %#v", dictionaryType)
	}

	if _, err := usecase.CreateDictionaryType(ctx, usecase.SaveDictionaryTypeCmd{
		TypeKey: "region",
		Name:    "Region",
		Enabled: true,
	}); fwusecase.CodeOf(err) != fwusecase.CodeConflict {
		t.Fatalf("expected conflict error, got %v", err)
	}

	if _, err := usecase.CreateDictionaryType(ctx, usecase.SaveDictionaryTypeCmd{
		TypeKey: "bad key",
		Name:    "Bad",
		Enabled: true,
	}); fwusecase.CodeOf(err) != fwusecase.CodeValidation {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestDictionaryValueValidationConflictAndToggle(t *testing.T) {
	setupUsecaseOrderTxDB(t)
	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)

	dictionaryType, err := usecase.CreateDictionaryType(ctx, usecase.SaveDictionaryTypeCmd{
		TypeKey: "order_status",
		Name:    "Order status",
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("create dictionary type: %v", err)
	}

	value, err := usecase.CreateDictionaryValue(ctx, usecase.SaveDictionaryValueCmd{
		DictionaryTypeID: dictionaryType.ID,
		ValueCode:        "Pending",
		Label:            "Pending",
		SortOrder:        10,
		Enabled:          true,
	})
	if err != nil {
		t.Fatalf("create dictionary value: %v", err)
	}
	if value.ValueCode != "Pending" || value.TypeKey != "order_status" {
		t.Fatalf("expected preserved value code, got %#v", value)
	}

	if _, err := usecase.CreateDictionaryValue(ctx, usecase.SaveDictionaryValueCmd{
		DictionaryTypeID: dictionaryType.ID,
		ValueCode:        "Pending",
		Label:            "Pending",
		Enabled:          true,
	}); fwusecase.CodeOf(err) != fwusecase.CodeConflict {
		t.Fatalf("expected conflict error, got %v", err)
	}

	disabled, err := usecase.SetDictionaryValueEnabled(ctx, usecase.SetDictionaryValueEnabledCmd{
		ID:      value.ID,
		Enabled: false,
	})
	if err != nil {
		t.Fatalf("disable dictionary value: %v", err)
	}
	if disabled.Enabled {
		t.Fatalf("expected disabled value, got %#v", disabled)
	}

	if _, err := usecase.CreateDictionaryValue(ctx, usecase.SaveDictionaryValueCmd{
		DictionaryTypeID: dictionaryType.ID,
		ValueCode:        "bad code",
		Label:            "Bad",
		Enabled:          true,
	}); fwusecase.CodeOf(err) != fwusecase.CodeValidation {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestDictionaryValueAllowsProviderStyleCodeWithoutLowercasing(t *testing.T) {
	setupUsecaseOrderTxDB(t)
	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)

	dictionaryType, err := usecase.CreateDictionaryType(ctx, usecase.SaveDictionaryTypeCmd{
		TypeKey: "embedding_model_vendor",
		Name:    "Embedding model vendor",
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("create dictionary type: %v", err)
	}

	value, err := usecase.CreateDictionaryValue(ctx, usecase.SaveDictionaryValueCmd{
		DictionaryTypeID: dictionaryType.ID,
		ValueCode:        "Qwen/Qwen3-Embedding-0.6B",
		Label:            "Qwen3 Embedding 0.6B",
		SortOrder:        10,
		Enabled:          true,
	})
	if err != nil {
		t.Fatalf("create dictionary value with provider-style code: %v", err)
	}
	if value.ValueCode != "Qwen/Qwen3-Embedding-0.6B" {
		t.Fatalf("expected provider-style code to be preserved, got %#v", value)
	}

	updated, err := usecase.UpdateDictionaryValue(ctx, usecase.SaveDictionaryValueCmd{
		ID:               value.ID,
		DictionaryTypeID: dictionaryType.ID,
		ValueCode:        "Qwen/Qwen3-Embedding-0.6B",
		Label:            "Updated label",
		SortOrder:        20,
		Enabled:          true,
	})
	if err != nil {
		t.Fatalf("update dictionary value with provider-style code: %v", err)
	}
	if updated.ValueCode != "Qwen/Qwen3-Embedding-0.6B" || updated.Label != "Updated label" {
		t.Fatalf("expected updated provider-style value, got %#v", updated)
	}
}
