package routes

import (
	"github.com/labstack/echo/v4"
	fwcontext "github.com/tfnick/go-svelte-starter/api/framework/http/context"
	httpresponse "github.com/tfnick/go-svelte-starter/api/framework/http/response"
	"github.com/tfnick/go-svelte-starter/api/usecase"
)

type VariableRequest struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	ValueType   string `json:"value_type"`
	ValueJSON   string `json:"value_json"`
	Enabled     bool   `json:"enabled"`
	Description string `json:"description"`
}

type SetVariableEnabledRequest struct {
	Enabled bool `json:"enabled"`
}

type VariableResponse struct {
	ID          string `json:"id"`
	Key         string `json:"key"`
	Name        string `json:"name"`
	ValueType   string `json:"value_type"`
	ValueJSON   string `json:"value_json"`
	Enabled     bool   `json:"enabled"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

func ListVariables(c echo.Context) error {
	ctx := fwcontext.InternalUsecaseContext(c)
	variables, err := usecase.ListVariables(ctx, usecase.ListVariablesQry{})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.OK(c, toVariableResponses(variables))
}

func CreateVariable(c echo.Context) error {
	var req VariableRequest
	if err := c.Bind(&req); err != nil {
		return httpresponse.BadRequest(c, "invalid request data")
	}

	ctx := fwcontext.InternalUsecaseContext(c)
	variable, err := usecase.CreateVariable(ctx, variableCmdFromRequest("", req))
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.Created(c, toVariableResponse(variable))
}

func UpdateVariable(c echo.Context) error {
	var req VariableRequest
	if err := c.Bind(&req); err != nil {
		return httpresponse.BadRequest(c, "invalid request data")
	}

	ctx := fwcontext.InternalUsecaseContext(c)
	variable, err := usecase.UpdateVariable(ctx, variableCmdFromRequest(c.Param("id"), req))
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.OK(c, toVariableResponse(variable))
}

func SetVariableEnabled(c echo.Context) error {
	var req SetVariableEnabledRequest
	if err := c.Bind(&req); err != nil {
		return httpresponse.BadRequest(c, "invalid request data")
	}

	ctx := fwcontext.InternalUsecaseContext(c)
	variable, err := usecase.SetVariableEnabled(ctx, usecase.SetVariableEnabledCmd{
		ID:      c.Param("id"),
		Enabled: req.Enabled,
	})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.OK(c, toVariableResponse(variable))
}

func variableCmdFromRequest(id string, req VariableRequest) usecase.SaveVariableCmd {
	return usecase.SaveVariableCmd{
		ID:          id,
		Key:         req.Key,
		Name:        req.Name,
		ValueType:   req.ValueType,
		ValueJSON:   req.ValueJSON,
		Enabled:     req.Enabled,
		Description: req.Description,
	}
}

func toVariableResponse(variable usecase.VariableCo) VariableResponse {
	return VariableResponse{
		ID:          variable.ID,
		Key:         variable.Key,
		Name:        variable.Name,
		ValueType:   variable.ValueType,
		ValueJSON:   variable.ValueJSON,
		Enabled:     variable.Enabled,
		Description: variable.Description,
		CreatedAt:   variable.CreatedAt,
		UpdatedAt:   variable.UpdatedAt,
	}
}

func toVariableResponses(variables []usecase.VariableCo) []VariableResponse {
	responses := make([]VariableResponse, 0, len(variables))
	for i := range variables {
		responses = append(responses, toVariableResponse(variables[i]))
	}
	return responses
}
