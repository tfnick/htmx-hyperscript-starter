package routes

import (
	"strings"

	"github.com/labstack/echo/v4"
	fwcontext "github.com/tfnick/go-svelte-starter/api/framework/http/context"
	httpresponse "github.com/tfnick/go-svelte-starter/api/framework/http/response"
	"github.com/tfnick/go-svelte-starter/api/usecase"
)

type DictionaryOptionResponse struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type DictionaryBatchResponse struct {
	Dictionaries map[string][]DictionaryOptionResponse `json:"dictionaries"`
}

type DictionaryTypeRequest struct {
	TypeKey     string `json:"type_key"`
	Name        string `json:"name"`
	Enabled     bool   `json:"enabled"`
	Description string `json:"description"`
}

type SetDictionaryTypeEnabledRequest struct {
	Enabled bool `json:"enabled"`
}

type DictionaryValueRequest struct {
	DictionaryTypeID string `json:"dictionary_type_id"`
	ValueCode        string `json:"value_code"`
	Label            string `json:"label"`
	SortOrder        int    `json:"sort_order"`
	Enabled          bool   `json:"enabled"`
	Description      string `json:"description"`
}

type SetDictionaryValueEnabledRequest struct {
	Enabled bool `json:"enabled"`
}

type DictionaryTypeResponse struct {
	ID          string `json:"id"`
	TypeKey     string `json:"type_key"`
	Name        string `json:"name"`
	Enabled     bool   `json:"enabled"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type DictionaryValueResponse struct {
	ID               string `json:"id"`
	DictionaryTypeID string `json:"dictionary_type_id"`
	TypeKey          string `json:"type_key"`
	ValueCode        string `json:"value_code"`
	Label            string `json:"label"`
	SortOrder        int    `json:"sort_order"`
	Enabled          bool   `json:"enabled"`
	Description      string `json:"description"`
	CreatedAt        string `json:"created_at"`
	UpdatedAt        string `json:"updated_at"`
}

func ToDictionaryOptionResponse(option usecase.DictionaryOptionCo) DictionaryOptionResponse {
	return DictionaryOptionResponse{
		Value: option.Value,
		Label: option.Label,
	}
}

func ToDictionaryOptionResponses(options []usecase.DictionaryOptionCo) []DictionaryOptionResponse {
	responses := make([]DictionaryOptionResponse, 0, len(options))
	for i := range options {
		responses = append(responses, ToDictionaryOptionResponse(options[i]))
	}
	return responses
}

func ToDictionaryBatchResponse(batch usecase.DictionaryBatchCo) DictionaryBatchResponse {
	dictionaries := make(map[string][]DictionaryOptionResponse, len(batch.Dictionaries))
	for dictionaryType, options := range batch.Dictionaries {
		dictionaries[dictionaryType] = ToDictionaryOptionResponses(options)
	}
	return DictionaryBatchResponse{Dictionaries: dictionaries}
}

func GetDictionaries(c echo.Context) error {
	types := parseDictionaryTypes(c.QueryParam("types"))
	if len(types) == 0 {
		return httpresponse.BadRequest(c, "types is required")
	}

	ctx := fwcontext.InternalUsecaseContext(c)
	batch, err := usecase.GetDictionaries(ctx, usecase.DictionaryBatchQry{Types: types})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}

	return httpresponse.OK(c, ToDictionaryBatchResponse(batch))
}

func ListDictionaryTypes(c echo.Context) error {
	ctx := fwcontext.InternalUsecaseContext(c)
	types, err := usecase.ListDictionaryTypes(ctx, usecase.ListDictionaryTypesQry{})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.OK(c, toDictionaryTypeResponses(types))
}

func CreateDictionaryType(c echo.Context) error {
	var req DictionaryTypeRequest
	if err := c.Bind(&req); err != nil {
		return httpresponse.BadRequest(c, "invalid request data")
	}

	ctx := fwcontext.InternalUsecaseContext(c)
	dictionaryType, err := usecase.CreateDictionaryType(ctx, dictionaryTypeCmdFromRequest("", req))
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.Created(c, toDictionaryTypeResponse(dictionaryType))
}

func UpdateDictionaryType(c echo.Context) error {
	var req DictionaryTypeRequest
	if err := c.Bind(&req); err != nil {
		return httpresponse.BadRequest(c, "invalid request data")
	}

	ctx := fwcontext.InternalUsecaseContext(c)
	dictionaryType, err := usecase.UpdateDictionaryType(ctx, dictionaryTypeCmdFromRequest(c.Param("id"), req))
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.OK(c, toDictionaryTypeResponse(dictionaryType))
}

func SetDictionaryTypeEnabled(c echo.Context) error {
	var req SetDictionaryTypeEnabledRequest
	if err := c.Bind(&req); err != nil {
		return httpresponse.BadRequest(c, "invalid request data")
	}

	ctx := fwcontext.InternalUsecaseContext(c)
	dictionaryType, err := usecase.SetDictionaryTypeEnabled(ctx, usecase.SetDictionaryTypeEnabledCmd{
		ID:      c.Param("id"),
		Enabled: req.Enabled,
	})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.OK(c, toDictionaryTypeResponse(dictionaryType))
}

func ListDictionaryValues(c echo.Context) error {
	ctx := fwcontext.InternalUsecaseContext(c)
	values, err := usecase.ListDictionaryValues(ctx, usecase.ListDictionaryValuesQry{
		DictionaryTypeID: c.Param("type_id"),
	})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.OK(c, toDictionaryValueResponses(values))
}

func CreateDictionaryValue(c echo.Context) error {
	var req DictionaryValueRequest
	if err := c.Bind(&req); err != nil {
		return httpresponse.BadRequest(c, "invalid request data")
	}

	ctx := fwcontext.InternalUsecaseContext(c)
	value, err := usecase.CreateDictionaryValue(ctx, dictionaryValueCmdFromRequest("", c.Param("type_id"), req))
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.Created(c, toDictionaryValueResponse(value))
}

func UpdateDictionaryValue(c echo.Context) error {
	var req DictionaryValueRequest
	if err := c.Bind(&req); err != nil {
		return httpresponse.BadRequest(c, "invalid request data")
	}

	ctx := fwcontext.InternalUsecaseContext(c)
	value, err := usecase.UpdateDictionaryValue(ctx, dictionaryValueCmdFromRequest(c.Param("id"), c.Param("type_id"), req))
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.OK(c, toDictionaryValueResponse(value))
}

func SetDictionaryValueEnabled(c echo.Context) error {
	var req SetDictionaryValueEnabledRequest
	if err := c.Bind(&req); err != nil {
		return httpresponse.BadRequest(c, "invalid request data")
	}

	ctx := fwcontext.InternalUsecaseContext(c)
	value, err := usecase.SetDictionaryValueEnabled(ctx, usecase.SetDictionaryValueEnabledCmd{
		ID:      c.Param("id"),
		Enabled: req.Enabled,
	})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.OK(c, toDictionaryValueResponse(value))
}

func parseDictionaryTypes(raw string) []string {
	parts := strings.Split(raw, ",")
	seen := make(map[string]struct{}, len(parts))
	types := make([]string, 0, len(parts))
	for _, part := range parts {
		dictionaryType := strings.TrimSpace(part)
		if dictionaryType == "" {
			continue
		}
		if _, exists := seen[dictionaryType]; exists {
			continue
		}
		seen[dictionaryType] = struct{}{}
		types = append(types, dictionaryType)
	}
	return types
}

func dictionaryTypeCmdFromRequest(id string, req DictionaryTypeRequest) usecase.SaveDictionaryTypeCmd {
	return usecase.SaveDictionaryTypeCmd{
		ID:          id,
		TypeKey:     req.TypeKey,
		Name:        req.Name,
		Enabled:     req.Enabled,
		Description: req.Description,
	}
}

func dictionaryValueCmdFromRequest(id string, typeID string, req DictionaryValueRequest) usecase.SaveDictionaryValueCmd {
	dictionaryTypeID := typeID
	if strings.TrimSpace(dictionaryTypeID) == "" {
		dictionaryTypeID = req.DictionaryTypeID
	}
	return usecase.SaveDictionaryValueCmd{
		ID:               id,
		DictionaryTypeID: dictionaryTypeID,
		ValueCode:        req.ValueCode,
		Label:            req.Label,
		SortOrder:        req.SortOrder,
		Enabled:          req.Enabled,
		Description:      req.Description,
	}
}

func toDictionaryTypeResponse(dictionaryType usecase.DictionaryTypeCo) DictionaryTypeResponse {
	return DictionaryTypeResponse{
		ID:          dictionaryType.ID,
		TypeKey:     dictionaryType.TypeKey,
		Name:        dictionaryType.Name,
		Enabled:     dictionaryType.Enabled,
		Description: dictionaryType.Description,
		CreatedAt:   dictionaryType.CreatedAt,
		UpdatedAt:   dictionaryType.UpdatedAt,
	}
}

func toDictionaryTypeResponses(types []usecase.DictionaryTypeCo) []DictionaryTypeResponse {
	responses := make([]DictionaryTypeResponse, 0, len(types))
	for i := range types {
		responses = append(responses, toDictionaryTypeResponse(types[i]))
	}
	return responses
}

func toDictionaryValueResponse(value usecase.DictionaryValueCo) DictionaryValueResponse {
	return DictionaryValueResponse{
		ID:               value.ID,
		DictionaryTypeID: value.DictionaryTypeID,
		TypeKey:          value.TypeKey,
		ValueCode:        value.ValueCode,
		Label:            value.Label,
		SortOrder:        value.SortOrder,
		Enabled:          value.Enabled,
		Description:      value.Description,
		CreatedAt:        value.CreatedAt,
		UpdatedAt:        value.UpdatedAt,
	}
}

func toDictionaryValueResponses(values []usecase.DictionaryValueCo) []DictionaryValueResponse {
	responses := make([]DictionaryValueResponse, 0, len(values))
	for i := range values {
		responses = append(responses, toDictionaryValueResponse(values[i]))
	}
	return responses
}
