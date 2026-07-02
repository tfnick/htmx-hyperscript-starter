package usecase

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/tfnick/go-svelte-starter/api/framework/cache"
	"github.com/tfnick/go-svelte-starter/api/framework/data/modelerror"
	"github.com/tfnick/go-svelte-starter/api/framework/integrations/providererror"
	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
	"github.com/tfnick/go-svelte-starter/api/models"
	"github.com/tfnick/go-svelte-starter/api/usecase/integrations/llm"
)

const (
	maxSummaryTextChars   = 20000
	maxSummaryPromptChars = 4000
	maxSummaryDimensions  = 8
)

type SummarizeTextWithLLMCmd struct {
	Text       string
	Prompt     string
	Dimensions []string
}

type LLMSummaryCo struct {
	Summary      map[string]string
	ModelCode    string
	ChannelCode  string
	InvocationID string
}

type llmChannelConfig struct {
	BaseURL string `json:"base_url"`
}

func SummarizeTextWithLLM(ctx fwusecase.Context, cmd SummarizeTextWithLLMCmd) (LLMSummaryCo, error) {
	dimensions, err := normalizeSummaryDimensions(cmd.Dimensions)
	if err != nil {
		return LLMSummaryCo{}, err
	}
	text := strings.TrimSpace(cmd.Text)
	if text == "" {
		return LLMSummaryCo{}, fwusecase.E(fwusecase.CodeValidation, "text is required", nil)
	}
	if len([]rune(text)) > maxSummaryTextChars {
		return LLMSummaryCo{}, fwusecase.E(fwusecase.CodeValidation, "text is too long", nil)
	}
	prompt := strings.TrimSpace(cmd.Prompt)
	if len([]rune(prompt)) > maxSummaryPromptChars {
		return LLMSummaryCo{}, fwusecase.E(fwusecase.CodeValidation, "prompt is too long", nil)
	}

	config, err := cache.Cached(ctx.Std(), "config.llm", "enabled:text_summary", 5*time.Minute, func() (models.IntegrationLLMConfig, error) {
		return models.GetEnabledLLMConfig(ctx.Std(), models.LLMConfigQuery{
			Scenario:  models.IntegrationScenarioLLM,
			Operation: llm.OperationTextSummary,
		})
	})
	if err != nil {
		if errors.Is(err, modelerror.ErrNotFound) {
			return LLMSummaryCo{}, fwusecase.E(fwusecase.CodeInternal, "LLM channel is not configured", err)
		}
		return LLMSummaryCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to load LLM configuration", err)
	}

	startedAt := time.Now()
	invocation, err := models.CreateIntegrationInvocation(ctx.Std(), models.CreateIntegrationInvocationCmd{
		Scenario:     models.IntegrationScenarioLLM,
		ChannelID:    config.Channel.ID,
		ChannelCode:  config.Channel.ChannelCode,
		ProviderCode: config.Channel.ProviderCode,
		Operation:    llm.OperationTextSummary,
		ModelCode:    config.Model.ModelCode,
	})
	if err != nil {
		return LLMSummaryCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to record LLM invocation", err)
	}

	providerCfg, err := llmProviderConfig(config)
	if err != nil {
		return LLMSummaryCo{}, failLLMInvocation(ctx, invocation.ID, startedAt, providererror.CategoryValidation, false, fwusecase.E(fwusecase.CodeInternal, "LLM channel is not configured", err))
	}

	adapter, ok := registeredLLMAdapter(config.Channel.AdapterKey)
	if !ok {
		cause := fmt.Errorf("LLM adapter not registered: %s", config.Channel.AdapterKey)
		return LLMSummaryCo{}, failLLMInvocation(ctx, invocation.ID, startedAt, providererror.CategoryPermanent, false, fwusecase.E(fwusecase.CodeInternal, "LLM adapter is not configured", cause))
	}

	request := buildSummaryGenerateRequest(text, prompt, dimensions)
	result, err := adapter.Generate(ctx.Std(), providerCfg, request)
	if err != nil {
		category, retryable, providerRequestID := providerFailureMetadata(err)
		completeErr := models.CompleteIntegrationInvocation(ctx.Std(), models.CompleteIntegrationInvocationCmd{
			ID:                invocation.ID,
			Status:            models.IntegrationInvocationStatusFailed,
			ProviderRequestID: providerRequestID,
			ErrorCategory:     category,
			Retryable:         retryable,
			DurationMS:        time.Since(startedAt).Milliseconds(),
		})
		if completeErr != nil {
			err = fmt.Errorf("%w; complete LLM invocation failed: %v", err, completeErr)
		}
		return LLMSummaryCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to generate summary", err)
	}

	summary, err := parseSummaryJSON(result.Content, dimensions)
	if err != nil {
		return LLMSummaryCo{}, failLLMInvocation(ctx, invocation.ID, startedAt, providererror.CategoryValidation, false, fwusecase.E(fwusecase.CodeInternal, "failed to parse LLM summary", err))
	}

	usageJSON, err := json.Marshal(result.Usage)
	if err != nil {
		return LLMSummaryCo{}, failLLMInvocation(ctx, invocation.ID, startedAt, providererror.CategoryPermanent, false, fwusecase.E(fwusecase.CodeInternal, "failed to record LLM usage", err))
	}
	if err := models.CompleteIntegrationInvocation(ctx.Std(), models.CompleteIntegrationInvocationCmd{
		ID:                invocation.ID,
		Status:            models.IntegrationInvocationStatusSucceeded,
		ProviderRequestID: result.ProviderRequestID,
		UsageJSON:         string(usageJSON),
		DurationMS:        time.Since(startedAt).Milliseconds(),
	}); err != nil {
		return LLMSummaryCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to record LLM invocation", err)
	}

	return LLMSummaryCo{
		Summary:      summary,
		ModelCode:    config.Model.ModelCode,
		ChannelCode:  config.Channel.ChannelCode,
		InvocationID: invocation.ID,
	}, nil
}

func normalizeSummaryDimensions(dimensions []string) ([]string, error) {
	if len(dimensions) == 0 {
		return nil, fwusecase.E(fwusecase.CodeValidation, "dimensions are required", nil)
	}
	if len(dimensions) > maxSummaryDimensions {
		return nil, fwusecase.E(fwusecase.CodeValidation, "too many dimensions", nil)
	}

	seen := map[string]bool{}
	normalized := make([]string, 0, len(dimensions))
	for _, dimension := range dimensions {
		dimension = strings.TrimSpace(dimension)
		if dimension == "" {
			return nil, fwusecase.E(fwusecase.CodeValidation, "dimension is required", nil)
		}
		if len(dimension) > 64 {
			return nil, fwusecase.E(fwusecase.CodeValidation, "dimension is too long", nil)
		}
		if seen[dimension] {
			continue
		}
		seen[dimension] = true
		normalized = append(normalized, dimension)
	}
	if len(normalized) == 0 {
		return nil, fwusecase.E(fwusecase.CodeValidation, "dimensions are required", nil)
	}
	return normalized, nil
}

func llmProviderConfig(config models.IntegrationLLMConfig) (llm.ProviderConfig, error) {
	channelConfig := llmChannelConfig{}
	if strings.TrimSpace(config.Channel.ConfigJSON) != "" {
		if err := json.Unmarshal([]byte(config.Channel.ConfigJSON), &channelConfig); err != nil {
			return llm.ProviderConfig{}, fmt.Errorf("parse channel config failed: %w", err)
		}
	}
	if strings.TrimSpace(channelConfig.BaseURL) == "" {
		return llm.ProviderConfig{}, fmt.Errorf("channel base_url is required")
	}

	apiKey := config.Credential.ValueText
	if strings.TrimSpace(apiKey) == "" {
		return llm.ProviderConfig{}, fmt.Errorf("credential is empty")
	}

	return llm.ProviderConfig{
		ChannelCode:       config.Channel.ChannelCode,
		ProviderCode:      config.Channel.ProviderCode,
		AdapterKey:        config.Channel.AdapterKey,
		BaseURL:           channelConfig.BaseURL,
		APIKey:            apiKey,
		ModelCode:         config.Model.ModelCode,
		ProviderModelID:   config.Model.ProviderModelID,
		DefaultParamsJSON: config.Model.DefaultParamsJSON,
	}, nil
}

func buildSummaryGenerateRequest(text string, prompt string, dimensions []string) llm.GenerateRequest {
	dimensionsJSON, _ := json.Marshal(dimensions)
	userContent := fmt.Sprintf("Dimensions: %s\n\nText:\n%s", string(dimensionsJSON), text)
	if strings.TrimSpace(prompt) != "" {
		userContent = fmt.Sprintf("Dimensions: %s\n\nRequirement prompt:\n%s\n\nText:\n%s", string(dimensionsJSON), prompt, text)
	}

	return llm.GenerateRequest{
		Operation:      llm.OperationTextSummary,
		ResponseFormat: llm.ResponseFormatJSON,
		Messages: []llm.Message{
			{
				Role:    "system",
				Content: "Summarize the user's text for each requested dimension. Return only one strict JSON object whose keys exactly match the requested dimension names and whose values are concise strings.",
			},
			{
				Role:    "user",
				Content: userContent,
			},
		},
	}
}

func parseSummaryJSON(content string, dimensions []string) (map[string]string, error) {
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(content), &raw); err != nil {
		return nil, fmt.Errorf("summary content is not a JSON object: %w", err)
	}

	summary := make(map[string]string, len(dimensions))
	for _, dimension := range dimensions {
		value, ok := raw[dimension]
		if !ok {
			return nil, fmt.Errorf("summary missing dimension %q", dimension)
		}
		switch v := value.(type) {
		case string:
			summary[dimension] = v
		default:
			encoded, err := json.Marshal(v)
			if err != nil {
				return nil, fmt.Errorf("encode dimension %q failed: %w", dimension, err)
			}
			summary[dimension] = string(encoded)
		}
	}
	return summary, nil
}

func providerFailureMetadata(err error) (string, bool, string) {
	if providerErr, ok := providererror.From(err); ok {
		return providerErr.Category, providerErr.Retryable, providerErr.ProviderRequestID
	}
	return providererror.CategoryTemporary, true, ""
}

func failLLMInvocation(ctx fwusecase.Context, invocationID string, startedAt time.Time, category string, retryable bool, err error) error {
	completeErr := models.CompleteIntegrationInvocation(ctx.Std(), models.CompleteIntegrationInvocationCmd{
		ID:            invocationID,
		Status:        models.IntegrationInvocationStatusFailed,
		ErrorCategory: category,
		Retryable:     retryable,
		DurationMS:    time.Since(startedAt).Milliseconds(),
	})
	if completeErr != nil {
		return fmt.Errorf("%w; complete LLM invocation failed: %v", err, completeErr)
	}
	return err
}
