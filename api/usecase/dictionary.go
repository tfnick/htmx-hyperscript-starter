package usecase

import (
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/tfnick/go-svelte-starter/api/framework/cache"
	"github.com/tfnick/go-svelte-starter/api/framework/data/modelerror"
	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
	"github.com/tfnick/go-svelte-starter/api/models"
)

type DictionaryBatchQry struct {
	Types []string
}

type DictionaryOptionCo struct {
	Value string
	Label string
}

type DictionaryBatchCo struct {
	Dictionaries map[string][]DictionaryOptionCo
}

type ListDictionaryTypesQry struct{}

type SaveDictionaryTypeCmd struct {
	ID          string
	TypeKey     string
	Name        string
	Enabled     bool
	Description string
}

type SetDictionaryTypeEnabledCmd struct {
	ID      string
	Enabled bool
}

type ListDictionaryValuesQry struct {
	DictionaryTypeID string
}

type SaveDictionaryValueCmd struct {
	ID               string
	DictionaryTypeID string
	ValueCode        string
	Label            string
	SortOrder        int
	Enabled          bool
	Description      string
}

type SetDictionaryValueEnabledCmd struct {
	ID      string
	Enabled bool
}

type DictionaryTypeCo struct {
	ID          string
	TypeKey     string
	Name        string
	Enabled     bool
	Description string
	CreatedAt   string
	UpdatedAt   string
}

type DictionaryValueCo struct {
	ID               string
	DictionaryTypeID string
	TypeKey          string
	ValueCode        string
	Label            string
	SortOrder        int
	Enabled          bool
	Description      string
	CreatedAt        string
	UpdatedAt        string
}

var dictionaryKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:[._-][a-z0-9]+)*$`)
var dictionaryValueCodePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]*$`)

func GetDictionaries(ctx fwusecase.Context, qry DictionaryBatchQry) (DictionaryBatchCo, error) {
	types := normalizeDictionaryTypes(qry.Types)
	options, err := cache.Cached(ctx.Std(), "dictionary.options", "all", 30*time.Minute, func() (map[string][]models.DictionaryValue, error) {
		return models.ListDictionaryOptions(ctx.Std(), types)
	})
	if err != nil {
		return DictionaryBatchCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to load dictionaries", err)
	}

	dictionaries := make(map[string][]DictionaryOptionCo, len(types))
	for _, dictionaryType := range types {
		values := options[dictionaryType]
		dictionaries[dictionaryType] = make([]DictionaryOptionCo, 0, len(values))
		for _, value := range values {
			dictionaries[dictionaryType] = append(dictionaries[dictionaryType], DictionaryOptionCo{
				Value: value.ValueCode,
				Label: value.Label,
			})
		}
	}
	return DictionaryBatchCo{Dictionaries: dictionaries}, nil
}

func ListDictionaryTypes(ctx fwusecase.Context, _ ListDictionaryTypesQry) ([]DictionaryTypeCo, error) {
	types, err := models.ListDictionaryTypes(ctx.Std())
	if err != nil {
		return nil, fwusecase.E(fwusecase.CodeInternal, "failed to load dictionary types", err)
	}
	return dictionaryTypeCosFromModels(types), nil
}

func CreateDictionaryType(ctx fwusecase.Context, cmd SaveDictionaryTypeCmd) (DictionaryTypeCo, error) {
	input, err := dictionaryTypeInput(cmd)
	if err != nil {
		return DictionaryTypeCo{}, err
	}

	dictionaryType, err := models.CreateDictionaryType(ctx.Std(), input)
	if err != nil {
		if errors.Is(err, models.ErrDictionaryConflict) {
			return DictionaryTypeCo{}, fwusecase.E(fwusecase.CodeConflict, "dictionary type already exists", err)
		}
		return DictionaryTypeCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to create dictionary type", err)
	}
	return dictionaryTypeCoFromModel(dictionaryType), nil
}

func UpdateDictionaryType(ctx fwusecase.Context, cmd SaveDictionaryTypeCmd) (DictionaryTypeCo, error) {
	if strings.TrimSpace(cmd.ID) == "" {
		return DictionaryTypeCo{}, fwusecase.E(fwusecase.CodeValidation, "dictionary type ID is required", nil)
	}

	input, err := dictionaryTypeInput(cmd)
	if err != nil {
		return DictionaryTypeCo{}, err
	}

	dictionaryType, err := models.UpdateDictionaryType(ctx.Std(), input)
	if err != nil {
		if errors.Is(err, models.ErrDictionaryConflict) {
			return DictionaryTypeCo{}, fwusecase.E(fwusecase.CodeConflict, "dictionary type already exists", err)
		}
		if errors.Is(err, modelerror.ErrNotFound) {
			return DictionaryTypeCo{}, fwusecase.E(fwusecase.CodeNotFound, "dictionary type not found", err)
		}
		return DictionaryTypeCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to update dictionary type", err)
	}
	return dictionaryTypeCoFromModel(dictionaryType), nil
}

func SetDictionaryTypeEnabled(ctx fwusecase.Context, cmd SetDictionaryTypeEnabledCmd) (DictionaryTypeCo, error) {
	id := strings.TrimSpace(cmd.ID)
	if id == "" {
		return DictionaryTypeCo{}, fwusecase.E(fwusecase.CodeValidation, "dictionary type ID is required", nil)
	}

	dictionaryType, err := models.SetDictionaryTypeEnabled(ctx.Std(), id, cmd.Enabled)
	if err != nil {
		if errors.Is(err, modelerror.ErrNotFound) {
			return DictionaryTypeCo{}, fwusecase.E(fwusecase.CodeNotFound, "dictionary type not found", err)
		}
		return DictionaryTypeCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to update dictionary type enabled state", err)
	}
	return dictionaryTypeCoFromModel(dictionaryType), nil
}

func ListDictionaryValues(ctx fwusecase.Context, qry ListDictionaryValuesQry) ([]DictionaryValueCo, error) {
	id := strings.TrimSpace(qry.DictionaryTypeID)
	if id == "" {
		return nil, fwusecase.E(fwusecase.CodeValidation, "dictionary type ID is required", nil)
	}
	if _, err := models.GetDictionaryTypeByID(ctx.Std(), id); err != nil {
		if errors.Is(err, modelerror.ErrNotFound) {
			return nil, fwusecase.E(fwusecase.CodeNotFound, "dictionary type not found", err)
		}
		return nil, fwusecase.E(fwusecase.CodeInternal, "failed to load dictionary type", err)
	}

	values, err := models.ListDictionaryValues(ctx.Std(), id)
	if err != nil {
		return nil, fwusecase.E(fwusecase.CodeInternal, "failed to load dictionary values", err)
	}
	return dictionaryValueCosFromModels(values), nil
}

func CreateDictionaryValue(ctx fwusecase.Context, cmd SaveDictionaryValueCmd) (DictionaryValueCo, error) {
	input, err := dictionaryValueInput(cmd)
	if err != nil {
		return DictionaryValueCo{}, err
	}
	if err := ensureDictionaryTypeExists(ctx, input.DictionaryTypeID); err != nil {
		return DictionaryValueCo{}, err
	}

	value, err := models.CreateDictionaryValue(ctx.Std(), input)
	if err != nil {
		if errors.Is(err, models.ErrDictionaryConflict) {
			return DictionaryValueCo{}, fwusecase.E(fwusecase.CodeConflict, "dictionary value already exists", err)
		}
		return DictionaryValueCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to create dictionary value", err)
	}
	return dictionaryValueCoFromModel(value), nil
}

func UpdateDictionaryValue(ctx fwusecase.Context, cmd SaveDictionaryValueCmd) (DictionaryValueCo, error) {
	if strings.TrimSpace(cmd.ID) == "" {
		return DictionaryValueCo{}, fwusecase.E(fwusecase.CodeValidation, "dictionary value ID is required", nil)
	}

	input, err := dictionaryValueInput(cmd)
	if err != nil {
		return DictionaryValueCo{}, err
	}
	if err := ensureDictionaryTypeExists(ctx, input.DictionaryTypeID); err != nil {
		return DictionaryValueCo{}, err
	}

	value, err := models.UpdateDictionaryValue(ctx.Std(), input)
	if err != nil {
		if errors.Is(err, models.ErrDictionaryConflict) {
			return DictionaryValueCo{}, fwusecase.E(fwusecase.CodeConflict, "dictionary value already exists", err)
		}
		if errors.Is(err, modelerror.ErrNotFound) {
			return DictionaryValueCo{}, fwusecase.E(fwusecase.CodeNotFound, "dictionary value not found", err)
		}
		return DictionaryValueCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to update dictionary value", err)
	}
	return dictionaryValueCoFromModel(value), nil
}

func SetDictionaryValueEnabled(ctx fwusecase.Context, cmd SetDictionaryValueEnabledCmd) (DictionaryValueCo, error) {
	id := strings.TrimSpace(cmd.ID)
	if id == "" {
		return DictionaryValueCo{}, fwusecase.E(fwusecase.CodeValidation, "dictionary value ID is required", nil)
	}

	value, err := models.SetDictionaryValueEnabled(ctx.Std(), id, cmd.Enabled)
	if err != nil {
		if errors.Is(err, modelerror.ErrNotFound) {
			return DictionaryValueCo{}, fwusecase.E(fwusecase.CodeNotFound, "dictionary value not found", err)
		}
		return DictionaryValueCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to update dictionary value enabled state", err)
	}
	return dictionaryValueCoFromModel(value), nil
}

func normalizeDictionaryTypes(types []string) []string {
	seen := make(map[string]struct{}, len(types))
	result := make([]string, 0, len(types))
	for _, value := range types {
		dictionaryType := strings.TrimSpace(strings.ToLower(value))
		if dictionaryType == "" {
			continue
		}
		if _, exists := seen[dictionaryType]; exists {
			continue
		}
		seen[dictionaryType] = struct{}{}
		result = append(result, dictionaryType)
	}
	return result
}

func dictionaryTypeInput(cmd SaveDictionaryTypeCmd) (models.SaveDictionaryTypeCmd, error) {
	typeKey := strings.TrimSpace(strings.ToLower(cmd.TypeKey))
	name := strings.TrimSpace(cmd.Name)
	if typeKey == "" {
		return models.SaveDictionaryTypeCmd{}, fwusecase.E(fwusecase.CodeValidation, "dictionary type key is required", nil)
	}
	if !dictionaryKeyPattern.MatchString(typeKey) {
		return models.SaveDictionaryTypeCmd{}, fwusecase.E(fwusecase.CodeValidation, "dictionary type key is invalid", nil)
	}
	if name == "" {
		return models.SaveDictionaryTypeCmd{}, fwusecase.E(fwusecase.CodeValidation, "dictionary type name is required", nil)
	}

	return models.SaveDictionaryTypeCmd{
		ID:          strings.TrimSpace(cmd.ID),
		TypeKey:     typeKey,
		Name:        name,
		Enabled:     cmd.Enabled,
		Description: strings.TrimSpace(cmd.Description),
	}, nil
}

func dictionaryValueInput(cmd SaveDictionaryValueCmd) (models.SaveDictionaryValueCmd, error) {
	dictionaryTypeID := strings.TrimSpace(cmd.DictionaryTypeID)
	valueCode := strings.TrimSpace(cmd.ValueCode)
	label := strings.TrimSpace(cmd.Label)
	if dictionaryTypeID == "" {
		return models.SaveDictionaryValueCmd{}, fwusecase.E(fwusecase.CodeValidation, "dictionary type ID is required", nil)
	}
	if valueCode == "" {
		return models.SaveDictionaryValueCmd{}, fwusecase.E(fwusecase.CodeValidation, "dictionary value code is required", nil)
	}
	if !dictionaryValueCodePattern.MatchString(valueCode) {
		return models.SaveDictionaryValueCmd{}, fwusecase.E(fwusecase.CodeValidation, "dictionary value code is invalid", nil)
	}
	if label == "" {
		return models.SaveDictionaryValueCmd{}, fwusecase.E(fwusecase.CodeValidation, "dictionary value label is required", nil)
	}

	return models.SaveDictionaryValueCmd{
		ID:               strings.TrimSpace(cmd.ID),
		DictionaryTypeID: dictionaryTypeID,
		ValueCode:        valueCode,
		Label:            label,
		SortOrder:        cmd.SortOrder,
		Enabled:          cmd.Enabled,
		Description:      strings.TrimSpace(cmd.Description),
	}, nil
}

func ensureDictionaryTypeExists(ctx fwusecase.Context, id string) error {
	if _, err := models.GetDictionaryTypeByID(ctx.Std(), id); err != nil {
		if errors.Is(err, modelerror.ErrNotFound) {
			return fwusecase.E(fwusecase.CodeNotFound, "dictionary type not found", err)
		}
		return fwusecase.E(fwusecase.CodeInternal, "failed to load dictionary type", err)
	}
	return nil
}

func dictionaryTypeCoFromModel(dictionaryType models.DictionaryType) DictionaryTypeCo {
	return DictionaryTypeCo{
		ID:          dictionaryType.ID,
		TypeKey:     dictionaryType.TypeKey,
		Name:        dictionaryType.Name,
		Enabled:     dictionaryType.Enabled == 1,
		Description: dictionaryType.Description,
		CreatedAt:   dictionaryType.CreatedAt,
		UpdatedAt:   dictionaryType.UpdatedAt,
	}
}

func dictionaryTypeCosFromModels(types []models.DictionaryType) []DictionaryTypeCo {
	result := make([]DictionaryTypeCo, 0, len(types))
	for i := range types {
		result = append(result, dictionaryTypeCoFromModel(types[i]))
	}
	return result
}

func dictionaryValueCoFromModel(value models.DictionaryValue) DictionaryValueCo {
	return DictionaryValueCo{
		ID:               value.ID,
		DictionaryTypeID: value.DictionaryTypeID,
		TypeKey:          value.TypeKey,
		ValueCode:        value.ValueCode,
		Label:            value.Label,
		SortOrder:        value.SortOrder,
		Enabled:          value.Enabled == 1,
		Description:      value.Description,
		CreatedAt:        value.CreatedAt,
		UpdatedAt:        value.UpdatedAt,
	}
}

func dictionaryValueCosFromModels(values []models.DictionaryValue) []DictionaryValueCo {
	result := make([]DictionaryValueCo, 0, len(values))
	for i := range values {
		result = append(result, dictionaryValueCoFromModel(values[i]))
	}
	return result
}
