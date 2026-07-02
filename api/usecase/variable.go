package usecase

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"strings"

	"github.com/tfnick/go-svelte-starter/api/framework/data/modelerror"
	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
	"github.com/tfnick/go-svelte-starter/api/models"
)

type ListVariablesQry struct{}

type SaveVariableCmd struct {
	ID          string
	Key         string
	Name        string
	ValueType   string
	ValueJSON   string
	Enabled     bool
	Description string
}

type SetVariableEnabledCmd struct {
	ID      string
	Enabled bool
}

type VariableCo struct {
	ID          string
	Key         string
	Name        string
	ValueType   string
	ValueJSON   string
	Enabled     bool
	Description string
	CreatedAt   string
	UpdatedAt   string
}

var variableKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:[._-][a-z0-9]+)*$`)

func ListVariables(ctx fwusecase.Context, _ ListVariablesQry) ([]VariableCo, error) {
	variables, err := models.ListVariables(ctx.Std())
	if err != nil {
		return nil, fwusecase.E(fwusecase.CodeInternal, "failed to load variables", err)
	}
	return variableCosFromModels(variables), nil
}

func CreateVariable(ctx fwusecase.Context, cmd SaveVariableCmd) (VariableCo, error) {
	input, err := variableInput(cmd)
	if err != nil {
		return VariableCo{}, err
	}

	variable, err := models.CreateVariable(ctx.Std(), input)
	if err != nil {
		if errors.Is(err, models.ErrVariableConflict) {
			return VariableCo{}, fwusecase.E(fwusecase.CodeConflict, "variable key already exists", err)
		}
		return VariableCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to create variable", err)
	}
	return variableCoFromModel(variable), nil
}

func UpdateVariable(ctx fwusecase.Context, cmd SaveVariableCmd) (VariableCo, error) {
	if strings.TrimSpace(cmd.ID) == "" {
		return VariableCo{}, fwusecase.E(fwusecase.CodeValidation, "variable ID is required", nil)
	}

	input, err := variableInput(cmd)
	if err != nil {
		return VariableCo{}, err
	}

	variable, err := models.UpdateVariable(ctx.Std(), input)
	if err != nil {
		if errors.Is(err, models.ErrVariableConflict) {
			return VariableCo{}, fwusecase.E(fwusecase.CodeConflict, "variable key already exists", err)
		}
		if errors.Is(err, modelerror.ErrNotFound) {
			return VariableCo{}, fwusecase.E(fwusecase.CodeNotFound, "variable not found", err)
		}
		return VariableCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to update variable", err)
	}
	return variableCoFromModel(variable), nil
}

func SetVariableEnabled(ctx fwusecase.Context, cmd SetVariableEnabledCmd) (VariableCo, error) {
	id := strings.TrimSpace(cmd.ID)
	if id == "" {
		return VariableCo{}, fwusecase.E(fwusecase.CodeValidation, "variable ID is required", nil)
	}

	variable, err := models.SetVariableEnabled(ctx.Std(), id, cmd.Enabled)
	if err != nil {
		if errors.Is(err, modelerror.ErrNotFound) {
			return VariableCo{}, fwusecase.E(fwusecase.CodeNotFound, "variable not found", err)
		}
		return VariableCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to update variable enabled state", err)
	}
	return variableCoFromModel(variable), nil
}

func variableInput(cmd SaveVariableCmd) (models.SaveVariableCmd, error) {
	key := strings.TrimSpace(strings.ToLower(cmd.Key))
	name := strings.TrimSpace(cmd.Name)
	valueType, err := normalizeVariableValueType(cmd.ValueType)
	if err != nil {
		return models.SaveVariableCmd{}, err
	}
	valueJSON, err := normalizeVariableValueJSON(valueType, cmd.ValueJSON)
	if err != nil {
		return models.SaveVariableCmd{}, err
	}

	if key == "" {
		return models.SaveVariableCmd{}, fwusecase.E(fwusecase.CodeValidation, "variable key is required", nil)
	}
	if !variableKeyPattern.MatchString(key) {
		return models.SaveVariableCmd{}, fwusecase.E(fwusecase.CodeValidation, "variable key is invalid", nil)
	}
	if name == "" {
		return models.SaveVariableCmd{}, fwusecase.E(fwusecase.CodeValidation, "variable name is required", nil)
	}

	return models.SaveVariableCmd{
		ID:          strings.TrimSpace(cmd.ID),
		Key:         key,
		Name:        name,
		ValueType:   valueType,
		ValueJSON:   valueJSON,
		Enabled:     cmd.Enabled,
		Description: strings.TrimSpace(cmd.Description),
	}, nil
}

func normalizeVariableValueType(value string) (string, error) {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case models.VariableValueTypeString:
		return models.VariableValueTypeString, nil
	case models.VariableValueTypeNumber:
		return models.VariableValueTypeNumber, nil
	case models.VariableValueTypeBoolean:
		return models.VariableValueTypeBoolean, nil
	case models.VariableValueTypeJSON:
		return models.VariableValueTypeJSON, nil
	default:
		return "", fwusecase.E(fwusecase.CodeValidation, "invalid variable value type", nil)
	}
}

func normalizeVariableValueJSON(valueType string, value string) (string, error) {
	raw := strings.TrimSpace(value)
	switch valueType {
	case models.VariableValueTypeString:
		if raw == "" {
			return `""`, nil
		}
		var decoded string
		if err := json.Unmarshal([]byte(raw), &decoded); err == nil {
			return encodeVariableJSON(decoded, "variable value")
		}
		return encodeVariableJSON(raw, "variable value")
	case models.VariableValueTypeNumber:
		decoded, err := decodeStrictJSON(raw, "variable value")
		if err != nil {
			return "", err
		}
		if _, ok := decoded.(json.Number); !ok {
			return "", fwusecase.E(fwusecase.CodeValidation, "variable value must be a number", nil)
		}
		return raw, nil
	case models.VariableValueTypeBoolean:
		decoded, err := decodeStrictJSON(raw, "variable value")
		if err != nil {
			return "", err
		}
		if _, ok := decoded.(bool); !ok {
			return "", fwusecase.E(fwusecase.CodeValidation, "variable value must be a boolean", nil)
		}
		return raw, nil
	case models.VariableValueTypeJSON:
		if raw == "" {
			raw = "{}"
		}
		decoded, err := decodeStrictJSON(raw, "variable value")
		if err != nil {
			return "", err
		}
		return encodeVariableJSON(decoded, "variable value")
	default:
		return "", fwusecase.E(fwusecase.CodeValidation, "invalid variable value type", nil)
	}
}

func decodeStrictJSON(raw string, label string) (interface{}, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, fwusecase.E(fwusecase.CodeValidation, label+" is required", nil)
	}

	decoder := json.NewDecoder(bytes.NewBufferString(raw))
	decoder.UseNumber()
	var decoded interface{}
	if err := decoder.Decode(&decoded); err != nil {
		return nil, fwusecase.E(fwusecase.CodeValidation, label+" is invalid", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, fwusecase.E(fwusecase.CodeValidation, label+" is invalid", err)
	}
	return decoded, nil
}

func encodeVariableJSON(value interface{}, label string) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", fwusecase.E(fwusecase.CodeInternal, "failed to encode "+label, err)
	}
	return string(encoded), nil
}

func variableCoFromModel(variable models.Variable) VariableCo {
	return VariableCo{
		ID:          variable.ID,
		Key:         variable.Key,
		Name:        variable.Name,
		ValueType:   variable.ValueType,
		ValueJSON:   variable.ValueJSON,
		Enabled:     variable.Enabled == 1,
		Description: variable.Description,
		CreatedAt:   variable.CreatedAt,
		UpdatedAt:   variable.UpdatedAt,
	}
}

func variableCosFromModels(variables []models.Variable) []VariableCo {
	result := make([]VariableCo, 0, len(variables))
	for i := range variables {
		result = append(result, variableCoFromModel(variables[i]))
	}
	return result
}
