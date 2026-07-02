package routes

import (
	"github.com/labstack/echo/v4"
	fwcontext "github.com/tfnick/go-svelte-starter/api/framework/http/context"
	httpresponse "github.com/tfnick/go-svelte-starter/api/framework/http/response"
	"github.com/tfnick/go-svelte-starter/api/usecase"
)

type ParameterIntegrationChannelRequest struct {
	Scenario            string `json:"scenario"`
	ChannelCode         string `json:"channel_code"`
	ProviderCode        string `json:"provider_code"`
	AdapterKey          string `json:"adapter_key"`
	Environment         string `json:"environment"`
	Enabled             bool   `json:"enabled"`
	Priority            int    `json:"priority"`
	WebhookEnabled      bool   `json:"webhook_enabled"`
	IsPrimary           bool   `json:"is_primary"`
	ConfigJSON          string `json:"config_json"`
	MetadataJSON        string `json:"metadata_json"`
	CredentialType      string `json:"credential_type"`
	CredentialValue     string `json:"credential_value"`
	CredentialPlaintext string `json:"credential_plaintext"`
	ModelCode           string `json:"model_code"`
	ProviderModelID     string `json:"provider_model_id"`
	Operation           string `json:"operation"`
}

type SetParameterIntegrationChannelEnabledRequest struct {
	Enabled bool `json:"enabled"`
}

type ParameterIntegrationSchemaOptionResponse struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type ParameterIntegrationSchemaFieldResponse struct {
	Key            string                                     `json:"key"`
	Label          string                                     `json:"label"`
	Kind           string                                     `json:"kind"`
	Required       bool                                       `json:"required"`
	Placeholder    string                                     `json:"placeholder"`
	HelpText       string                                     `json:"help_text"`
	DefaultValue   string                                     `json:"default_value"`
	DictionaryType string                                     `json:"dictionary_type"`
	Sensitive      bool                                       `json:"sensitive"`
	Options        []ParameterIntegrationSchemaOptionResponse `json:"options"`
}

type ParameterIntegrationAdapterSchemaResponse struct {
	Scenario            string                                    `json:"scenario"`
	AdapterKey          string                                    `json:"adapter_key"`
	Label               string                                    `json:"label"`
	Description         string                                    `json:"description"`
	ProviderCode        string                                    `json:"provider_code"`
	CredentialType      string                                    `json:"credential_type"`
	CredentialFormat    string                                    `json:"credential_format"`
	ModelDictionaryType string                                    `json:"model_dictionary_type"`
	AdvancedJSON        bool                                      `json:"advanced_json"`
	ConfigFields        []ParameterIntegrationSchemaFieldResponse `json:"config_fields"`
	CredentialFields    []ParameterIntegrationSchemaFieldResponse `json:"credential_fields"`
}

type ParameterIntegrationChannelResponse struct {
	ID              string `json:"id"`
	Scenario        string `json:"scenario"`
	ChannelCode     string `json:"channel_code"`
	ProviderCode    string `json:"provider_code"`
	AdapterKey      string `json:"adapter_key"`
	Environment     string `json:"environment"`
	Enabled         bool   `json:"enabled"`
	Priority        int    `json:"priority"`
	CredentialType  string `json:"credential_type"`
	CredentialValue string `json:"credential_value"`
	WebhookEnabled  bool   `json:"webhook_enabled"`
	IsPrimary       bool   `json:"is_primary"`
	ConfigJSON      string `json:"config_json"`
	MetadataJSON    string `json:"metadata_json"`
	ModelCode       string `json:"model_code"`
	ProviderModelID string `json:"provider_model_id"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}

func ListParameterIntegrationSchemas(c echo.Context) error {
	ctx := fwcontext.InternalUsecaseContext(c)
	schemas, err := usecase.ListParameterIntegrationSchemas(ctx, usecase.ListParameterIntegrationSchemasQry{
		Scenario: c.QueryParam("scenario"),
	})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.OK(c, toParameterIntegrationAdapterSchemaResponses(schemas))
}

func ListParameterIntegrationChannels(c echo.Context) error {
	ctx := fwcontext.InternalUsecaseContext(c)
	channels, err := usecase.ListParameterIntegrationChannels(ctx, usecase.ListParameterIntegrationChannelsQry{
		Scenario: c.QueryParam("scenario"),
	})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.OK(c, toParameterIntegrationChannelResponses(channels))
}

func CreateParameterIntegrationChannel(c echo.Context) error {
	var req ParameterIntegrationChannelRequest
	if err := c.Bind(&req); err != nil {
		return httpresponse.BadRequest(c, "invalid request data")
	}

	ctx := fwcontext.InternalUsecaseContext(c)
	channel, err := usecase.CreateParameterIntegrationChannel(ctx, parameterIntegrationChannelCmdFromRequest("", req))
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.Created(c, toParameterIntegrationChannelResponse(channel))
}

func UpdateParameterIntegrationChannel(c echo.Context) error {
	var req ParameterIntegrationChannelRequest
	if err := c.Bind(&req); err != nil {
		return httpresponse.BadRequest(c, "invalid request data")
	}

	ctx := fwcontext.InternalUsecaseContext(c)
	channel, err := usecase.UpdateParameterIntegrationChannel(ctx, parameterIntegrationChannelCmdFromRequest(c.Param("id"), req))
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.OK(c, toParameterIntegrationChannelResponse(channel))
}

func SetParameterIntegrationChannelEnabled(c echo.Context) error {
	var req SetParameterIntegrationChannelEnabledRequest
	if err := c.Bind(&req); err != nil {
		return httpresponse.BadRequest(c, "invalid request data")
	}

	ctx := fwcontext.InternalUsecaseContext(c)
	channel, err := usecase.SetParameterIntegrationChannelEnabled(ctx, usecase.SetParameterIntegrationChannelEnabledCmd{
		ID:      c.Param("id"),
		Enabled: req.Enabled,
	})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.OK(c, toParameterIntegrationChannelResponse(channel))
}

func parameterIntegrationChannelCmdFromRequest(id string, req ParameterIntegrationChannelRequest) usecase.SaveParameterIntegrationChannelCmd {
	credentialValue := req.CredentialValue
	if credentialValue == "" {
		credentialValue = req.CredentialPlaintext
	}
	return usecase.SaveParameterIntegrationChannelCmd{
		ID:              id,
		Scenario:        req.Scenario,
		ChannelCode:     req.ChannelCode,
		ProviderCode:    req.ProviderCode,
		AdapterKey:      req.AdapterKey,
		Environment:     req.Environment,
		Enabled:         req.Enabled,
		Priority:        req.Priority,
		WebhookEnabled:  req.WebhookEnabled,
		IsPrimary:       req.IsPrimary,
		ConfigJSON:      req.ConfigJSON,
		MetadataJSON:    req.MetadataJSON,
		CredentialType:  req.CredentialType,
		CredentialValue: credentialValue,
		ModelCode:       req.ModelCode,
		ProviderModelID: req.ProviderModelID,
		Operation:       req.Operation,
	}
}

func toParameterIntegrationSchemaOptionResponse(option usecase.ParameterIntegrationSchemaOptionCo) ParameterIntegrationSchemaOptionResponse {
	return ParameterIntegrationSchemaOptionResponse{
		Value: option.Value,
		Label: option.Label,
	}
}

func toParameterIntegrationSchemaOptionResponses(options []usecase.ParameterIntegrationSchemaOptionCo) []ParameterIntegrationSchemaOptionResponse {
	responses := make([]ParameterIntegrationSchemaOptionResponse, 0, len(options))
	for i := range options {
		responses = append(responses, toParameterIntegrationSchemaOptionResponse(options[i]))
	}
	return responses
}

func toParameterIntegrationSchemaFieldResponse(field usecase.ParameterIntegrationSchemaFieldCo) ParameterIntegrationSchemaFieldResponse {
	return ParameterIntegrationSchemaFieldResponse{
		Key:            field.Key,
		Label:          field.Label,
		Kind:           field.Kind,
		Required:       field.Required,
		Placeholder:    field.Placeholder,
		HelpText:       field.HelpText,
		DefaultValue:   field.DefaultValue,
		DictionaryType: field.DictionaryType,
		Sensitive:      field.Sensitive,
		Options:        toParameterIntegrationSchemaOptionResponses(field.Options),
	}
}

func toParameterIntegrationSchemaFieldResponses(fields []usecase.ParameterIntegrationSchemaFieldCo) []ParameterIntegrationSchemaFieldResponse {
	responses := make([]ParameterIntegrationSchemaFieldResponse, 0, len(fields))
	for i := range fields {
		responses = append(responses, toParameterIntegrationSchemaFieldResponse(fields[i]))
	}
	return responses
}

func toParameterIntegrationAdapterSchemaResponse(schema usecase.ParameterIntegrationAdapterSchemaCo) ParameterIntegrationAdapterSchemaResponse {
	return ParameterIntegrationAdapterSchemaResponse{
		Scenario:            schema.Scenario,
		AdapterKey:          schema.AdapterKey,
		Label:               schema.Label,
		Description:         schema.Description,
		ProviderCode:        schema.ProviderCode,
		CredentialType:      schema.CredentialType,
		CredentialFormat:    schema.CredentialFormat,
		ModelDictionaryType: schema.ModelDictionaryType,
		AdvancedJSON:        schema.AdvancedJSON,
		ConfigFields:        toParameterIntegrationSchemaFieldResponses(schema.ConfigFields),
		CredentialFields:    toParameterIntegrationSchemaFieldResponses(schema.CredentialFields),
	}
}

func toParameterIntegrationAdapterSchemaResponses(schemas []usecase.ParameterIntegrationAdapterSchemaCo) []ParameterIntegrationAdapterSchemaResponse {
	responses := make([]ParameterIntegrationAdapterSchemaResponse, 0, len(schemas))
	for i := range schemas {
		responses = append(responses, toParameterIntegrationAdapterSchemaResponse(schemas[i]))
	}
	return responses
}

func toParameterIntegrationChannelResponse(channel usecase.ParameterIntegrationChannelCo) ParameterIntegrationChannelResponse {
	return ParameterIntegrationChannelResponse{
		ID:              channel.ID,
		Scenario:        channel.Scenario,
		ChannelCode:     channel.ChannelCode,
		ProviderCode:    channel.ProviderCode,
		AdapterKey:      channel.AdapterKey,
		Environment:     channel.Environment,
		Enabled:         channel.Enabled,
		Priority:        channel.Priority,
		CredentialType:  channel.CredentialType,
		CredentialValue: channel.CredentialValue,
		WebhookEnabled:  channel.WebhookEnabled,
		IsPrimary:       channel.IsPrimary,
		ConfigJSON:      channel.ConfigJSON,
		MetadataJSON:    channel.MetadataJSON,
		ModelCode:       channel.ModelCode,
		ProviderModelID: channel.ProviderModelID,
		CreatedAt:       channel.CreatedAt,
		UpdatedAt:       channel.UpdatedAt,
	}
}

func toParameterIntegrationChannelResponses(channels []usecase.ParameterIntegrationChannelCo) []ParameterIntegrationChannelResponse {
	responses := make([]ParameterIntegrationChannelResponse, 0, len(channels))
	for i := range channels {
		responses = append(responses, toParameterIntegrationChannelResponse(channels[i]))
	}
	return responses
}
