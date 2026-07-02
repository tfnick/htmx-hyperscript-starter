package usecase

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/tfnick/go-svelte-starter/api/framework/cache"
	"github.com/tfnick/go-svelte-starter/api/framework/data/modelerror"
	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
	"github.com/tfnick/go-svelte-starter/api/models"
)

const parameterIntegrationCredentialTypeDictionary = "integration_credential_type"

type ListParameterIntegrationChannelsQry struct {
	Scenario string
}

type SaveParameterIntegrationChannelCmd struct {
	ID              string
	Scenario        string
	ChannelCode     string
	ProviderCode    string
	AdapterKey      string
	Environment     string
	Enabled         bool
	Priority        int
	WebhookEnabled  bool
	IsPrimary       bool
	ConfigJSON      string
	MetadataJSON    string
	CredentialType  string
	CredentialValue string
	ModelCode       string
	ProviderModelID string
	Operation       string
}

type SetParameterIntegrationChannelEnabledCmd struct {
	ID      string
	Enabled bool
}

type ParameterIntegrationChannelCo struct {
	ID              string
	Scenario        string
	ChannelCode     string
	ProviderCode    string
	AdapterKey      string
	Environment     string
	Enabled         bool
	Priority        int
	CredentialType  string
	CredentialValue string
	WebhookEnabled  bool
	IsPrimary       bool
	ConfigJSON      string
	MetadataJSON    string
	ModelCode       string
	ProviderModelID string
	CreatedAt       string
	UpdatedAt       string
}

func ListParameterIntegrationChannels(ctx fwusecase.Context, qry ListParameterIntegrationChannelsQry) ([]ParameterIntegrationChannelCo, error) {
	scenario, err := normalizeIntegrationScenario(qry.Scenario)
	if err != nil {
		return nil, err
	}

	channels, err := models.ListIntegrationChannelConfigs(ctx.Std(), scenario)
	if err != nil {
		return nil, fwusecase.E(fwusecase.CodeInternal, "failed to load integration channels", err)
	}
	return parameterIntegrationChannelCosFromModels(channels), nil
}

func CreateParameterIntegrationChannel(ctx fwusecase.Context, cmd SaveParameterIntegrationChannelCmd) (ParameterIntegrationChannelCo, error) {
	input, err := parameterIntegrationChannelInput(cmd, true)
	if err != nil {
		return ParameterIntegrationChannelCo{}, err
	}
	if err := ensureParameterCredentialTypeAllowed(ctx, input.CredentialType); err != nil {
		return ParameterIntegrationChannelCo{}, err
	}

	var created models.IntegrationChannelConfig
	err = fwusecase.WithAppTx(ctx, func(txCtx fwusecase.Context) error {
		credential, err := models.CreateIntegrationCredential(txCtx.Std(), models.CreateIntegrationCredentialCmd{
			CredentialType: input.CredentialType,
			ValueText:      input.CredentialValue,
			Enabled:        true,
		})
		if err != nil {
			return fwusecase.E(fwusecase.CodeInternal, "failed to create integration credential", err)
		}
		if err := clearSSPrimaryIfNeeded(txCtx, input); err != nil {
			return err
		}
		created, err = models.CreateIntegrationChannel(txCtx.Std(), models.CreateIntegrationChannelCmd{
			Scenario:       input.Scenario,
			ChannelCode:    input.ChannelCode,
			ProviderCode:   input.ProviderCode,
			AdapterKey:     input.AdapterKey,
			Environment:    input.Environment,
			Enabled:        input.Enabled,
			Priority:       input.Priority,
			CredentialID:   credential.ID,
			WebhookEnabled: input.WebhookEnabled,
			IsPrimary:      input.IsPrimary,
			ConfigJSON:     input.ConfigJSON,
			MetadataJSON:   input.MetadataJSON,
		})
		if err != nil {
			if errors.Is(err, models.ErrIntegrationChannelConflict) {
				return fwusecase.E(fwusecase.CodeConflict, "integration channel already exists", err)
			}
			return fwusecase.E(fwusecase.CodeInternal, "failed to create integration channel", err)
		}

		if input.ModelCode != "" && input.ProviderModelID != "" {
			if err := saveParameterIntegrationModelSelection(txCtx, created.ID, input); err != nil {
				return err
			}
			created, err = models.GetIntegrationChannelConfigByID(txCtx.Std(), created.ID)
			if err != nil {
				return fwusecase.E(fwusecase.CodeInternal, "failed to reload integration channel", err)
			}
		}
		return nil
	})
	if err != nil {
		return ParameterIntegrationChannelCo{}, err
	}
	return parameterIntegrationChannelCoFromModel(created), nil
}

func UpdateParameterIntegrationChannel(ctx fwusecase.Context, cmd SaveParameterIntegrationChannelCmd) (ParameterIntegrationChannelCo, error) {
	if strings.TrimSpace(cmd.ID) == "" {
		return ParameterIntegrationChannelCo{}, fwusecase.E(fwusecase.CodeValidation, "integration channel ID is required", nil)
	}
	input, err := parameterIntegrationChannelInput(cmd, false)
	if err != nil {
		return ParameterIntegrationChannelCo{}, err
	}
	if err := ensureParameterCredentialTypeAllowed(ctx, input.CredentialType); err != nil {
		return ParameterIntegrationChannelCo{}, err
	}

	var updated models.IntegrationChannelConfig
	err = fwusecase.WithAppTx(ctx, func(txCtx fwusecase.Context) error {
		current, err := models.GetIntegrationChannelConfigByID(txCtx.Std(), input.ID)
		if err != nil {
			if errors.Is(err, modelerror.ErrNotFound) {
				return fwusecase.E(fwusecase.CodeNotFound, "integration channel not found", err)
			}
			return fwusecase.E(fwusecase.CodeInternal, "failed to load integration channel", err)
		}
		updateCredential := input.CredentialValue != "" || !parameterIntegrationSchemaRequiresCredential(input.AdapterKey)
		if _, err := models.UpdateIntegrationCredential(txCtx.Std(), models.UpdateIntegrationCredentialCmd{
			ID:             current.CredentialID,
			CredentialType: input.CredentialType,
			ValueText:      input.CredentialValue,
			Enabled:        true,
			UpdateValue:    updateCredential,
		}); err != nil {
			if errors.Is(err, modelerror.ErrNotFound) {
				return fwusecase.E(fwusecase.CodeNotFound, "integration credential not found", err)
			}
			return fwusecase.E(fwusecase.CodeInternal, "failed to update integration credential", err)
		}
		if err := clearSSPrimaryIfNeeded(txCtx, input); err != nil {
			return err
		}

		updated, err = models.UpdateIntegrationChannel(txCtx.Std(), models.UpdateIntegrationChannelCmd{
			ID:             input.ID,
			Scenario:       input.Scenario,
			ChannelCode:    input.ChannelCode,
			ProviderCode:   input.ProviderCode,
			AdapterKey:     input.AdapterKey,
			Environment:    input.Environment,
			Enabled:        input.Enabled,
			Priority:       input.Priority,
			WebhookEnabled: input.WebhookEnabled,
			IsPrimary:      input.IsPrimary,
			ConfigJSON:     input.ConfigJSON,
			MetadataJSON:   input.MetadataJSON,
		})
		if err != nil {
			if errors.Is(err, models.ErrIntegrationChannelConflict) {
				return fwusecase.E(fwusecase.CodeConflict, "integration channel already exists", err)
			}
			if errors.Is(err, modelerror.ErrNotFound) {
				return fwusecase.E(fwusecase.CodeNotFound, "integration channel not found", err)
			}
			return fwusecase.E(fwusecase.CodeInternal, "failed to update integration channel", err)
		}
		if input.ModelCode != "" && input.ProviderModelID != "" {
			if err := saveParameterIntegrationModelSelection(txCtx, updated.ID, input); err != nil {
				return err
			}
			updated, err = models.GetIntegrationChannelConfigByID(txCtx.Std(), updated.ID)
			if err != nil {
				return fwusecase.E(fwusecase.CodeInternal, "failed to reload integration channel", err)
			}
		}
		return nil
	})
	if err != nil {
		return ParameterIntegrationChannelCo{}, err
	}
	return parameterIntegrationChannelCoFromModel(updated), nil
}

func saveParameterIntegrationModelSelection(ctx fwusecase.Context, channelID string, input parameterIntegrationChannelInputData) error {
	if _, err := models.SetIntegrationChannelModelOption(ctx.Std(), models.CreateIntegrationModelOptionCmd{
		Scenario:        input.Scenario,
		ChannelID:       channelID,
		ModelCode:       input.ModelCode,
		ProviderModelID: input.ProviderModelID,
		Enabled:         true,
	}); err != nil {
		return fwusecase.E(fwusecase.CodeInternal, "failed to save integration model option", err)
	}

	if input.Operation == "" {
		return nil
	}
	if _, err := models.UpsertIntegrationOperationConfig(ctx.Std(), models.CreateIntegrationOperationConfigCmd{
		Scenario:    input.Scenario,
		Operation:   input.Operation,
		ChannelCode: input.ChannelCode,
		ModelCode:   input.ModelCode,
		Enabled:     true,
	}); err != nil {
		return fwusecase.E(fwusecase.CodeInternal, "failed to save integration operation config", err)
	}
	return nil
}

func SetParameterIntegrationChannelEnabled(ctx fwusecase.Context, cmd SetParameterIntegrationChannelEnabledCmd) (ParameterIntegrationChannelCo, error) {
	id := strings.TrimSpace(cmd.ID)
	if id == "" {
		return ParameterIntegrationChannelCo{}, fwusecase.E(fwusecase.CodeValidation, "integration channel ID is required", nil)
	}

	channel, err := models.SetIntegrationChannelEnabled(ctx.Std(), id, cmd.Enabled)
	if err != nil {
		if errors.Is(err, modelerror.ErrNotFound) {
			return ParameterIntegrationChannelCo{}, fwusecase.E(fwusecase.CodeNotFound, "integration channel not found", err)
		}
		return ParameterIntegrationChannelCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to update integration channel enabled state", err)
	}
	return parameterIntegrationChannelCoFromModel(channel), nil
}

type parameterIntegrationChannelInputData struct {
	ID              string
	Scenario        string
	ChannelCode     string
	ProviderCode    string
	AdapterKey      string
	Environment     string
	Enabled         bool
	Priority        int
	WebhookEnabled  bool
	IsPrimary       bool
	ConfigJSON      string
	MetadataJSON    string
	CredentialType  string
	CredentialValue string
	ModelCode       string
	ProviderModelID string
	Operation       string
}

func parameterIntegrationChannelInput(cmd SaveParameterIntegrationChannelCmd, create bool) (parameterIntegrationChannelInputData, error) {
	scenario, err := normalizeIntegrationScenario(cmd.Scenario)
	if err != nil {
		return parameterIntegrationChannelInputData{}, err
	}

	input := parameterIntegrationChannelInputData{
		ID:              strings.TrimSpace(cmd.ID),
		Scenario:        scenario,
		ChannelCode:     strings.TrimSpace(cmd.ChannelCode),
		ProviderCode:    strings.TrimSpace(cmd.ProviderCode),
		AdapterKey:      strings.TrimSpace(cmd.AdapterKey),
		Environment:     strings.TrimSpace(cmd.Environment),
		Enabled:         cmd.Enabled,
		Priority:        cmd.Priority,
		WebhookEnabled:  cmd.WebhookEnabled,
		IsPrimary:       cmd.IsPrimary && cmd.Enabled && parameterIntegrationScenarioAllowsPrimary(scenario),
		CredentialType:  strings.TrimSpace(cmd.CredentialType),
		CredentialValue: strings.TrimSpace(cmd.CredentialValue),
		ModelCode:       strings.TrimSpace(cmd.ModelCode),
		ProviderModelID: strings.TrimSpace(cmd.ProviderModelID),
		Operation:       strings.TrimSpace(cmd.Operation),
	}
	if input.Environment == "" {
		input.Environment = "production"
	}
	if input.Priority <= 0 {
		input.Priority = 100
	}

	if input.ChannelCode == "" {
		return parameterIntegrationChannelInputData{}, fwusecase.E(fwusecase.CodeValidation, "channel code is required", nil)
	}
	if input.ProviderCode == "" {
		return parameterIntegrationChannelInputData{}, fwusecase.E(fwusecase.CodeValidation, "provider code is required", nil)
	}
	if input.AdapterKey == "" {
		return parameterIntegrationChannelInputData{}, fwusecase.E(fwusecase.CodeValidation, "adapter key is required", nil)
	}
	if input.CredentialType == "" {
		return parameterIntegrationChannelInputData{}, fwusecase.E(fwusecase.CodeValidation, "credential type is required", nil)
	}
	if create && input.CredentialValue == "" && parameterIntegrationSchemaRequiresCredential(input.AdapterKey) {
		return parameterIntegrationChannelInputData{}, fwusecase.E(fwusecase.CodeValidation, "credential value is required", nil)
	}

	configJSON, err := normalizeSafeJSONObject(cmd.ConfigJSON, "config JSON")
	if err != nil {
		return parameterIntegrationChannelInputData{}, err
	}
	metadataJSON, err := normalizeSafeJSONObject(cmd.MetadataJSON, "metadata JSON")
	if err != nil {
		return parameterIntegrationChannelInputData{}, err
	}
	input.ConfigJSON = configJSON
	input.MetadataJSON = metadataJSON
	if err := validateParameterIntegrationSchema(input, create); err != nil {
		return parameterIntegrationChannelInputData{}, err
	}
	normalizeParameterIntegrationModelSelection(&input)
	return input, nil
}

func normalizeParameterIntegrationModelSelection(input *parameterIntegrationChannelInputData) {
	if input == nil || input.ModelCode == "" {
		return
	}
	if input.ProviderModelID == "" {
		input.ProviderModelID = input.ModelCode
	}
	if input.Scenario != models.IntegrationScenarioEmbedding || input.AdapterKey != "embedding.siliconflow.openai_compatible" {
		return
	}

	providerModelID := input.ProviderModelID
	modelCode := embeddingModelCodeForProviderModel(providerModelID, "")
	if modelCode == "" {
		modelCode = embeddingModelCodeForProviderModel(input.ModelCode, input.ModelCode)
	}
	if modelCode != "" {
		input.ModelCode = modelCode
	}
}

func clearSSPrimaryIfNeeded(ctx fwusecase.Context, input parameterIntegrationChannelInputData) error {
	if !input.IsPrimary {
		return nil
	}
	if err := models.ClearIntegrationChannelPrimary(ctx.Std(), input.Scenario); err != nil {
		return fwusecase.E(fwusecase.CodeInternal, "failed to update primary provider", err)
	}
	return nil
}

func ensureParameterCredentialTypeAllowed(ctx fwusecase.Context, credentialType string) error {
	options, err := cache.Cached(ctx.Std(), "dictionary.options", parameterIntegrationCredentialTypeDictionary, 30*time.Minute, func() (map[string][]models.DictionaryValue, error) {
		return models.ListDictionaryOptions(ctx.Std(), []string{parameterIntegrationCredentialTypeDictionary})
	})
	if err != nil {
		return fwusecase.E(fwusecase.CodeInternal, "failed to load credential type dictionary", err)
	}
	for _, option := range options[parameterIntegrationCredentialTypeDictionary] {
		if option.ValueCode == credentialType {
			return nil
		}
	}
	return fwusecase.E(fwusecase.CodeValidation, "invalid credential type", nil)
}

func normalizeIntegrationScenario(value string) (string, error) {
	scenario := strings.TrimSpace(strings.ToLower(value))
	switch scenario {
	case models.IntegrationScenarioPayment, models.IntegrationScenarioLLM, models.IntegrationScenarioSMS, models.IntegrationScenarioEmail, models.IntegrationScenarioOSS, models.IntegrationScenarioEmbedding, models.IntegrationScenarioExternalNotification:
		return scenario, nil
	default:
		return "", fwusecase.E(fwusecase.CodeValidation, "invalid integration scenario", nil)
	}
}

func parameterIntegrationScenarioAllowsPrimary(scenario string) bool {
	switch scenario {
	case models.IntegrationScenarioOSS, models.IntegrationScenarioLLM, models.IntegrationScenarioEmbedding, models.IntegrationScenarioExternalNotification:
		return true
	default:
		return false
	}
}

func normalizeSafeJSONObject(value string, label string) (string, error) {
	raw := strings.TrimSpace(value)
	if raw == "" {
		raw = "{}"
	}

	decoded, err := decodeJSONObject(raw, label)
	if err != nil {
		return "", err
	}
	if hasSensitiveJSONKey(decoded) {
		return "", fwusecase.E(fwusecase.CodeValidation, label+" contains sensitive keys", nil)
	}

	encoded, err := json.Marshal(decoded)
	if err != nil {
		return "", fwusecase.E(fwusecase.CodeInternal, "failed to encode "+label, err)
	}
	return string(encoded), nil
}

func decodeJSONObject(value string, label string) (map[string]interface{}, error) {
	var root interface{}
	if err := json.Unmarshal([]byte(value), &root); err != nil {
		return nil, fwusecase.E(fwusecase.CodeValidation, label+" is invalid", err)
	}
	decoded, ok := root.(map[string]interface{})
	if !ok {
		return nil, fwusecase.E(fwusecase.CodeValidation, label+" must be a JSON object", nil)
	}
	if decoded == nil {
		decoded = map[string]interface{}{}
	}
	return decoded, nil
}

func hasSensitiveJSONKey(value map[string]interface{}) bool {
	for key, item := range value {
		normalizedKey := strings.ToLower(strings.ReplaceAll(key, "_", ""))
		if strings.Contains(normalizedKey, "secret") ||
			strings.Contains(normalizedKey, "password") ||
			strings.Contains(normalizedKey, "apikey") ||
			strings.Contains(normalizedKey, "token") ||
			strings.Contains(normalizedKey, "privatekey") {
			return true
		}
		if hasSensitiveJSONValue(item) {
			return true
		}
	}
	return false
}

func hasSensitiveJSONValue(value interface{}) bool {
	switch typed := value.(type) {
	case map[string]interface{}:
		return hasSensitiveJSONKey(typed)
	case []interface{}:
		for _, item := range typed {
			if hasSensitiveJSONValue(item) {
				return true
			}
		}
	}
	return false
}

func parameterIntegrationChannelCoFromModel(channel models.IntegrationChannelConfig) ParameterIntegrationChannelCo {
	return ParameterIntegrationChannelCo{
		ID:              channel.ID,
		Scenario:        channel.Scenario,
		ChannelCode:     channel.ChannelCode,
		ProviderCode:    channel.ProviderCode,
		AdapterKey:      channel.AdapterKey,
		Environment:     channel.Environment,
		Enabled:         channel.Enabled == 1,
		Priority:        channel.Priority,
		CredentialType:  channel.CredentialType,
		CredentialValue: channel.CredentialValue,
		WebhookEnabled:  channel.WebhookEnabled == 1,
		IsPrimary:       parameterIntegrationScenarioAllowsPrimary(channel.Scenario) && channel.IsPrimary == 1,
		ConfigJSON:      channel.ConfigJSON,
		MetadataJSON:    channel.MetadataJSON,
		ModelCode:       channel.ModelCode,
		ProviderModelID: channel.ProviderModelID,
		CreatedAt:       channel.CreatedAt,
		UpdatedAt:       channel.UpdatedAt,
	}
}

func parameterIntegrationChannelCosFromModels(channels []models.IntegrationChannelConfig) []ParameterIntegrationChannelCo {
	result := make([]ParameterIntegrationChannelCo, 0, len(channels))
	for i := range channels {
		result = append(result, parameterIntegrationChannelCoFromModel(channels[i]))
	}
	return result
}
