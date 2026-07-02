package models_test

import (
	"errors"
	"testing"

	"github.com/tfnick/go-svelte-starter/api/models"
)

func TestDictionaryTypeAndValueCRUD(t *testing.T) {
	setupModelsTestDB(t)

	dictionaryType, err := models.CreateDictionaryType(t.Context(), models.SaveDictionaryTypeCmd{
		TypeKey:     "order_status",
		Name:        "Order status",
		Enabled:     true,
		Description: "Order lifecycle status",
	})
	if err != nil {
		t.Fatalf("create dictionary type: %v", err)
	}
	if dictionaryType.ID == "" || dictionaryType.TypeKey != "order_status" || dictionaryType.Enabled != 1 {
		t.Fatalf("unexpected dictionary type: %#v", dictionaryType)
	}

	updatedType, err := models.UpdateDictionaryType(t.Context(), models.SaveDictionaryTypeCmd{
		ID:          dictionaryType.ID,
		TypeKey:     "order_state",
		Name:        "Order state",
		Enabled:     true,
		Description: "Updated",
	})
	if err != nil {
		t.Fatalf("update dictionary type: %v", err)
	}
	if updatedType.TypeKey != "order_state" || updatedType.Description != "Updated" {
		t.Fatalf("unexpected updated type: %#v", updatedType)
	}

	value, err := models.CreateDictionaryValue(t.Context(), models.SaveDictionaryValueCmd{
		DictionaryTypeID: dictionaryType.ID,
		ValueCode:        "pending",
		Label:            "Pending",
		SortOrder:        20,
		Enabled:          true,
		Description:      "Waiting for payment",
	})
	if err != nil {
		t.Fatalf("create dictionary value: %v", err)
	}
	if value.ID == "" || value.TypeKey != "order_state" || value.ValueCode != "pending" {
		t.Fatalf("unexpected dictionary value: %#v", value)
	}

	updatedValue, err := models.UpdateDictionaryValue(t.Context(), models.SaveDictionaryValueCmd{
		ID:               value.ID,
		DictionaryTypeID: dictionaryType.ID,
		ValueCode:        "paid",
		Label:            "Paid",
		SortOrder:        10,
		Enabled:          true,
		Description:      "Paid order",
	})
	if err != nil {
		t.Fatalf("update dictionary value: %v", err)
	}
	if updatedValue.ValueCode != "paid" || updatedValue.SortOrder != 10 {
		t.Fatalf("unexpected updated value: %#v", updatedValue)
	}

	disabledValue, err := models.SetDictionaryValueEnabled(t.Context(), value.ID, false)
	if err != nil {
		t.Fatalf("disable dictionary value: %v", err)
	}
	if disabledValue.Enabled != 0 {
		t.Fatalf("expected disabled value, got %#v", disabledValue)
	}

	values, err := models.ListDictionaryValues(t.Context(), dictionaryType.ID)
	if err != nil {
		t.Fatalf("list dictionary values: %v", err)
	}
	if len(values) != 1 || values[0].ID != value.ID {
		t.Fatalf("unexpected dictionary values: %#v", values)
	}
}

func TestDictionaryOptionsReturnOnlyEnabledRows(t *testing.T) {
	setupModelsTestDB(t)

	dictionaryType, err := models.CreateDictionaryType(t.Context(), models.SaveDictionaryTypeCmd{
		TypeKey: "feature_area",
		Name:    "Feature area",
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("create dictionary type: %v", err)
	}
	if _, err := models.CreateDictionaryValue(t.Context(), models.SaveDictionaryValueCmd{
		DictionaryTypeID: dictionaryType.ID,
		ValueCode:        "checkout",
		Label:            "Checkout",
		SortOrder:        10,
		Enabled:          true,
	}); err != nil {
		t.Fatalf("create enabled value: %v", err)
	}
	if _, err := models.CreateDictionaryValue(t.Context(), models.SaveDictionaryValueCmd{
		DictionaryTypeID: dictionaryType.ID,
		ValueCode:        "hidden",
		Label:            "Hidden",
		SortOrder:        20,
		Enabled:          false,
	}); err != nil {
		t.Fatalf("create disabled value: %v", err)
	}

	options, err := models.ListDictionaryOptions(t.Context(), []string{"feature_area", "missing"})
	if err != nil {
		t.Fatalf("list dictionary options: %v", err)
	}
	if len(options["feature_area"]) != 1 || options["feature_area"][0].ValueCode != "checkout" {
		t.Fatalf("unexpected enabled options: %#v", options)
	}
	if len(options["missing"]) != 0 {
		t.Fatalf("expected empty missing dictionary, got %#v", options["missing"])
	}

	if _, err := models.SetDictionaryTypeEnabled(t.Context(), dictionaryType.ID, false); err != nil {
		t.Fatalf("disable dictionary type: %v", err)
	}
	options, err = models.ListDictionaryOptions(t.Context(), []string{"feature_area"})
	if err != nil {
		t.Fatalf("list disabled dictionary options: %v", err)
	}
	if len(options["feature_area"]) != 0 {
		t.Fatalf("expected disabled type to return no options, got %#v", options)
	}
}

func TestDictionaryDuplicateConflicts(t *testing.T) {
	setupModelsTestDB(t)

	cmd := models.SaveDictionaryTypeCmd{
		TypeKey: "duplicate_type",
		Name:    "Duplicate type",
		Enabled: true,
	}
	dictionaryType, err := models.CreateDictionaryType(t.Context(), cmd)
	if err != nil {
		t.Fatalf("create dictionary type: %v", err)
	}
	if _, err := models.CreateDictionaryType(t.Context(), cmd); !errors.Is(err, models.ErrDictionaryConflict) {
		t.Fatalf("expected dictionary type conflict, got %v", err)
	}

	valueCmd := models.SaveDictionaryValueCmd{
		DictionaryTypeID: dictionaryType.ID,
		ValueCode:        "same",
		Label:            "Same",
		Enabled:          true,
	}
	if _, err := models.CreateDictionaryValue(t.Context(), valueCmd); err != nil {
		t.Fatalf("create dictionary value: %v", err)
	}
	if _, err := models.CreateDictionaryValue(t.Context(), valueCmd); !errors.Is(err, models.ErrDictionaryConflict) {
		t.Fatalf("expected dictionary value conflict, got %v", err)
	}
}
