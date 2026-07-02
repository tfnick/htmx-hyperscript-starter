package usecase_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
	"github.com/tfnick/go-svelte-starter/api/usecase"
	"github.com/tfnick/go-svelte-starter/api/usecase/integrations/llm"
	"github.com/tfnick/sqlx"
)

type fakeLLMAdapter struct {
	t *testing.T
}

func (a fakeLLMAdapter) Generate(ctx context.Context, cfg llm.ProviderConfig, req llm.GenerateRequest) (llm.GenerateResult, error) {
	a.t.Helper()
	if cfg.ChannelCode != "usecase-deepseek" {
		a.t.Fatalf("unexpected channel code: %s", cfg.ChannelCode)
	}
	if cfg.ModelCode != "usecase-summary-fast" || cfg.ProviderModelID != "usecase-provider-model" {
		a.t.Fatalf("unexpected model mapping: %#v", cfg)
	}
	if cfg.APIKey != "usecase-api-key" {
		a.t.Fatalf("unexpected decrypted api key: %q", cfg.APIKey)
	}
	if req.ResponseFormat != llm.ResponseFormatJSON || req.Operation != llm.OperationTextSummary {
		a.t.Fatalf("unexpected request: %#v", req)
	}
	if len(req.Messages) < 2 || !strings.Contains(req.Messages[1].Content, "Requirement prompt:\nSummarize for an executive audience") {
		a.t.Fatalf("expected prompt in user message: %#v", req.Messages)
	}

	return llm.GenerateResult{
		Content:           `{"key_points":"hello","risks":"none"}`,
		ProviderRequestID: "provider-request-1",
		Usage: llm.Usage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		},
	}, nil
}

type deepSeekDefaultModelAdapter struct {
	t *testing.T
}

func (a deepSeekDefaultModelAdapter) Generate(ctx context.Context, cfg llm.ProviderConfig, req llm.GenerateRequest) (llm.GenerateResult, error) {
	a.t.Helper()
	if cfg.ChannelCode != "usecase-deepseek-channel-only" {
		a.t.Fatalf("unexpected channel code: %s", cfg.ChannelCode)
	}
	if cfg.ModelCode != "deepseek-chat" || cfg.ProviderModelID != "deepseek-chat" {
		a.t.Fatalf("unexpected default model mapping: %#v", cfg)
	}
	if cfg.APIKey != "usecase-channel-only-api-key" {
		a.t.Fatalf("unexpected decrypted api key: %q", cfg.APIKey)
	}

	return llm.GenerateResult{
		Content:           `{"summary":"ok"}`,
		ProviderRequestID: "provider-request-default",
		Usage: llm.Usage{
			PromptTokens:     8,
			CompletionTokens: 2,
			TotalTokens:      10,
		},
	}, nil
}

func TestSummarizeTextWithLLMUsesDBConfigAndRecordsInvocation(t *testing.T) {
	manager := setupUsecaseOrderTxDB(t)
	appDB, err := manager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}
	seedIntegrationConfig(t, appDB, "usecase-deepseek", "usecase-summary-fast", "usecase-provider-model", "usecase-api-key")
	if err := usecase.RegisterLLMAdapter("llm.deepseek.usecase-test", fakeLLMAdapter{t: t}); err != nil {
		t.Fatalf("register fake adapter: %v", err)
	}
	if _, err := appDB.ExecP(`UPDATE integration_channels SET adapter_key = 'llm.deepseek.usecase-test' WHERE channel_code = 'usecase-deepseek'`); err != nil {
		t.Fatalf("update adapter key: %v", err)
	}

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	result, err := usecase.SummarizeTextWithLLM(ctx, usecase.SummarizeTextWithLLMCmd{
		Text:       "Long business text",
		Prompt:     "Summarize for an executive audience",
		Dimensions: []string{"key_points", "risks"},
	})
	if err != nil {
		t.Fatalf("summarize text: %v", err)
	}

	if result.ChannelCode != "usecase-deepseek" || result.ModelCode != "usecase-summary-fast" {
		t.Fatalf("unexpected summary metadata: %#v", result)
	}
	if result.Summary["key_points"] != "hello" || result.Summary["risks"] != "none" {
		t.Fatalf("unexpected summary: %#v", result.Summary)
	}

	var row struct {
		Status            string `db:"status"`
		ProviderRequestID string `db:"provider_request_id"`
		ErrorCategory     string `db:"error_category"`
		UsageJSON         string `db:"usage_json"`
	}
	if err := appDB.GetP(&row, `SELECT status, provider_request_id, error_category, usage_json FROM integration_invocations WHERE id = ?`, result.InvocationID); err != nil {
		t.Fatalf("load invocation: %v", err)
	}
	if row.Status != "succeeded" || row.ProviderRequestID != "provider-request-1" || row.ErrorCategory != "" {
		t.Fatalf("unexpected invocation row: %#v", row)
	}
	var usage llm.Usage
	if err := json.Unmarshal([]byte(row.UsageJSON), &usage); err != nil {
		t.Fatalf("decode usage json: %v", err)
	}
	if usage.TotalTokens != 15 {
		t.Fatalf("unexpected usage: %#v", usage)
	}
}

func TestSummarizeTextWithLLMUsesDefaultDeepSeekModelForChannelOnlyConfig(t *testing.T) {
	manager := setupUsecaseOrderTxDB(t)
	appDB, err := manager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}
	seedDeepSeekChannelOnlyConfig(t, appDB, "usecase-deepseek-channel-only", "usecase-channel-only-api-key")
	if err := usecase.RegisterLLMAdapter("llm.deepseek.openai_compatible", deepSeekDefaultModelAdapter{t: t}); err != nil {
		t.Fatalf("register fake adapter: %v", err)
	}

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	result, err := usecase.SummarizeTextWithLLM(ctx, usecase.SummarizeTextWithLLMCmd{
		Text:       "Long business text",
		Dimensions: []string{"summary"},
	})
	if err != nil {
		t.Fatalf("summarize text: %v", err)
	}

	if result.ChannelCode != "usecase-deepseek-channel-only" || result.ModelCode != "deepseek-chat" {
		t.Fatalf("unexpected summary metadata: %#v", result)
	}
	if result.Summary["summary"] != "ok" {
		t.Fatalf("unexpected summary: %#v", result.Summary)
	}
}

func TestSummarizeTextWithLLMRejectsInvalidInput(t *testing.T) {
	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	_, err := usecase.SummarizeTextWithLLM(ctx, usecase.SummarizeTextWithLLMCmd{
		Text:       "",
		Dimensions: []string{"key_points"},
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if fwusecase.CodeOf(err) != fwusecase.CodeValidation {
		t.Fatalf("expected validation, got %s: %v", fwusecase.CodeOf(err), err)
	}
}

func seedDeepSeekChannelOnlyConfig(t *testing.T, appDB *sqlx.Engine, channelCode string, apiKey string) {
	t.Helper()

	credentialValue, err := credentialsForTest(apiKey)
	if err != nil {
		t.Fatalf("prepare credential: %v", err)
	}
	credentialID := channelCode + "-credential"
	channelID := channelCode + "-channel"

	if _, err := appDB.ExecP(`
		INSERT INTO integration_credentials (id, credential_type, ciphertext, key_version, masked_value, value_text, enabled)
		VALUES (?, 'api_key', ?, '', '', ?, 1)
	`, credentialID, credentialValue, credentialValue); err != nil {
		t.Fatalf("insert integration credential: %v", err)
	}
	if _, err := appDB.ExecP(`
		INSERT INTO integration_channels (
			id, scenario, channel_code, provider_code, adapter_key, environment, enabled, priority, credential_id, config_json
		) VALUES (?, 'llm', ?, 'deepseek', 'llm.deepseek.openai_compatible', 'test', 1, 1, ?, '{"base_url":"https://api.deepseek.com"}')
	`, channelID, channelCode, credentialID); err != nil {
		t.Fatalf("insert integration channel: %v", err)
	}
}

func seedIntegrationConfig(t *testing.T, appDB *sqlx.Engine, channelCode string, modelCode string, providerModelID string, apiKey string) {
	t.Helper()

	credentialValue, err := credentialsForTest(apiKey)
	if err != nil {
		t.Fatalf("prepare credential: %v", err)
	}
	credentialID := channelCode + "-credential"
	channelID := channelCode + "-channel"
	modelID := modelCode + "-model"

	if _, err := appDB.ExecP(`
		INSERT INTO integration_credentials (id, credential_type, ciphertext, key_version, masked_value, value_text, enabled)
		VALUES (?, 'api_key', ?, '', '', ?, 1)
	`, credentialID, credentialValue, credentialValue); err != nil {
		t.Fatalf("insert integration credential: %v", err)
	}
	if _, err := appDB.ExecP(`
		INSERT INTO integration_channels (
			id, scenario, channel_code, provider_code, adapter_key, environment, enabled, priority, credential_id, config_json
		) VALUES (?, 'llm', ?, 'deepseek', 'llm.deepseek.openai_compatible', 'test', 1, 1, ?, '{"base_url":"https://api.deepseek.com"}')
	`, channelID, channelCode, credentialID); err != nil {
		t.Fatalf("insert integration channel: %v", err)
	}
	if _, err := appDB.ExecP(`
		INSERT INTO integration_model_options (
			id, scenario, channel_id, model_code, provider_model_id, default_params_json, enabled
		) VALUES (?, 'llm', ?, ?, ?, '{"temperature":0.2}', 1)
	`, modelID, channelID, modelCode, providerModelID); err != nil {
		t.Fatalf("insert integration model option: %v", err)
	}
	if _, err := appDB.ExecP(`
		INSERT INTO integration_operation_configs (
			id, scenario, operation, channel_code, model_code, enabled
		) VALUES (?, 'llm', 'text_summary', ?, ?, 1)
	`, channelCode+"-text-summary-config", channelCode, modelCode); err != nil {
		t.Fatalf("insert integration operation config: %v", err)
	}
}
