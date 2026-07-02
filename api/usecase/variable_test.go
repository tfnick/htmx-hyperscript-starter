package usecase_test

import (
	"testing"

	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
	"github.com/tfnick/go-svelte-starter/api/models"
	"github.com/tfnick/go-svelte-starter/api/usecase"
)

func TestCreateVariableNormalizesTypedValues(t *testing.T) {
	setupUsecaseOrderTxDB(t)
	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)

	stringVar, err := usecase.CreateVariable(ctx, usecase.SaveVariableCmd{
		Key:       "Greeting.Message",
		Name:      "Greeting message",
		ValueType: models.VariableValueTypeString,
		ValueJSON: "hello",
		Enabled:   true,
	})
	if err != nil {
		t.Fatalf("create string variable: %v", err)
	}
	if stringVar.Key != "greeting.message" || stringVar.ValueJSON != `"hello"` {
		t.Fatalf("unexpected string variable: %#v", stringVar)
	}

	numberVar, err := usecase.CreateVariable(ctx, usecase.SaveVariableCmd{
		Key:       "checkout.max_retry",
		Name:      "Checkout max retry",
		ValueType: models.VariableValueTypeNumber,
		ValueJSON: "3",
		Enabled:   true,
	})
	if err != nil {
		t.Fatalf("create number variable: %v", err)
	}
	if numberVar.ValueJSON != "3" {
		t.Fatalf("unexpected number variable: %#v", numberVar)
	}

	jsonVar, err := usecase.CreateVariable(ctx, usecase.SaveVariableCmd{
		Key:       "checkout.thresholds",
		Name:      "Checkout thresholds",
		ValueType: models.VariableValueTypeJSON,
		ValueJSON: "{\n  \"max\": 10\n}",
		Enabled:   true,
	})
	if err != nil {
		t.Fatalf("create json variable: %v", err)
	}
	if jsonVar.ValueJSON != `{"max":10}` {
		t.Fatalf("unexpected json variable: %#v", jsonVar)
	}
}

func TestVariableValidationRejectsInvalidInput(t *testing.T) {
	setupUsecaseOrderTxDB(t)
	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)

	cases := []struct {
		name string
		cmd  usecase.SaveVariableCmd
	}{
		{
			name: "invalid key",
			cmd: usecase.SaveVariableCmd{
				Key:       "Bad Key",
				Name:      "Bad",
				ValueType: models.VariableValueTypeString,
				ValueJSON: "value",
			},
		},
		{
			name: "number value mismatch",
			cmd: usecase.SaveVariableCmd{
				Key:       "valid.number",
				Name:      "Bad number",
				ValueType: models.VariableValueTypeNumber,
				ValueJSON: `"not-number"`,
			},
		},
		{
			name: "boolean value mismatch",
			cmd: usecase.SaveVariableCmd{
				Key:       "valid.boolean",
				Name:      "Bad boolean",
				ValueType: models.VariableValueTypeBoolean,
				ValueJSON: `"true"`,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := usecase.CreateVariable(ctx, tc.cmd)
			if fwusecase.CodeOf(err) != fwusecase.CodeValidation {
				t.Fatalf("expected validation error, got %v", err)
			}
		})
	}
}

func TestVariableConflictAndEnableToggle(t *testing.T) {
	setupUsecaseOrderTxDB(t)
	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)

	cmd := usecase.SaveVariableCmd{
		Key:       "feature.new_checkout",
		Name:      "New checkout",
		ValueType: models.VariableValueTypeBoolean,
		ValueJSON: "true",
		Enabled:   true,
	}
	variable, err := usecase.CreateVariable(ctx, cmd)
	if err != nil {
		t.Fatalf("create variable: %v", err)
	}
	if _, err := usecase.CreateVariable(ctx, cmd); fwusecase.CodeOf(err) != fwusecase.CodeConflict {
		t.Fatalf("expected conflict error, got %v", err)
	}

	disabled, err := usecase.SetVariableEnabled(ctx, usecase.SetVariableEnabledCmd{
		ID:      variable.ID,
		Enabled: false,
	})
	if err != nil {
		t.Fatalf("disable variable: %v", err)
	}
	if disabled.Enabled {
		t.Fatalf("expected disabled variable, got %#v", disabled)
	}
}
