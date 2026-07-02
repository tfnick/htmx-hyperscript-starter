package models_test

import (
	"errors"
	"testing"

	"github.com/tfnick/go-svelte-starter/api/models"
)

func TestVariableCRUDAndOrdering(t *testing.T) {
	setupModelsTestDB(t)

	first, err := models.CreateVariable(t.Context(), models.SaveVariableCmd{
		Key:         "checkout.max_retry",
		Name:        "Checkout max retry",
		ValueType:   models.VariableValueTypeNumber,
		ValueJSON:   "3",
		Enabled:     true,
		Description: "Maximum checkout retries",
	})
	if err != nil {
		t.Fatalf("create first variable: %v", err)
	}
	if first.ID == "" || first.Key != "checkout.max_retry" || first.Enabled != 1 {
		t.Fatalf("unexpected first variable: %#v", first)
	}

	second, err := models.CreateVariable(t.Context(), models.SaveVariableCmd{
		Key:       "feature.new_checkout",
		Name:      "New checkout",
		ValueType: models.VariableValueTypeBoolean,
		ValueJSON: "true",
		Enabled:   false,
	})
	if err != nil {
		t.Fatalf("create second variable: %v", err)
	}

	updated, err := models.UpdateVariable(t.Context(), models.SaveVariableCmd{
		ID:          first.ID,
		Key:         "checkout.max_attempts",
		Name:        "Checkout max attempts",
		ValueType:   models.VariableValueTypeNumber,
		ValueJSON:   "5",
		Enabled:     true,
		Description: "Updated",
	})
	if err != nil {
		t.Fatalf("update variable: %v", err)
	}
	if updated.Key != "checkout.max_attempts" || updated.ValueJSON != "5" || updated.Description != "Updated" {
		t.Fatalf("unexpected updated variable: %#v", updated)
	}

	disabled, err := models.SetVariableEnabled(t.Context(), first.ID, false)
	if err != nil {
		t.Fatalf("disable variable: %v", err)
	}
	if disabled.Enabled != 0 {
		t.Fatalf("expected disabled variable, got %#v", disabled)
	}

	variables, err := models.ListVariables(t.Context())
	if err != nil {
		t.Fatalf("list variables: %v", err)
	}
	if len(variables) != 2 {
		t.Fatalf("expected two variables, got %#v", variables)
	}
	if variables[0].ID != first.ID || variables[1].ID != second.ID {
		t.Fatalf("unexpected variable ordering: %#v", variables)
	}
}

func TestCreateVariableReturnsConflictOnDuplicateKey(t *testing.T) {
	setupModelsTestDB(t)

	cmd := models.SaveVariableCmd{
		Key:       "feature.rollout",
		Name:      "Feature rollout",
		ValueType: models.VariableValueTypeBoolean,
		ValueJSON: "false",
		Enabled:   true,
	}
	if _, err := models.CreateVariable(t.Context(), cmd); err != nil {
		t.Fatalf("create variable: %v", err)
	}

	_, err := models.CreateVariable(t.Context(), cmd)
	if !errors.Is(err, models.ErrVariableConflict) {
		t.Fatalf("expected conflict error, got %v", err)
	}
}
