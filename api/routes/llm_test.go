package routes_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/tfnick/go-svelte-starter/api/db"
	"github.com/tfnick/go-svelte-starter/api/routes"
	"github.com/tfnick/go-svelte-starter/api/usecase"
	"github.com/tfnick/go-svelte-starter/api/usecase/integrations/llm"
	"github.com/tfnick/sqlx"
)

type routeFakeLLMAdapter struct{}

func (routeFakeLLMAdapter) Generate(ctx context.Context, cfg llm.ProviderConfig, req llm.GenerateRequest) (llm.GenerateResult, error) {
	if len(req.Messages) < 2 || !strings.Contains(req.Messages[1].Content, "Requirement prompt:\nFocus on action items") {
		return llm.GenerateResult{}, errRouteMissingPrompt
	}
	return llm.GenerateResult{
		Content:           `{"actions":"follow up"}`,
		ProviderRequestID: "route-provider-request",
		Usage:             llm.Usage{TotalTokens: 3},
	}, nil
}

var errRouteMissingPrompt = errors.New("missing prompt in LLM request")

func TestSummarizeTextWithLLMUsesInternalEnvelope(t *testing.T) {
	setupRouteTestDBs(t)
	appDB, err := db.DefaultManager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}
	seedRouteIntegrationConfig(t, appDB)
	if err := usecase.RegisterLLMAdapter("llm.deepseek.route-test", routeFakeLLMAdapter{}); err != nil {
		t.Fatalf("register adapter: %v", err)
	}

	router := echo.New()
	body := strings.NewReader(`{"text":"please summarize","prompt":"Focus on action items","dimensions":["actions"]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/llm/summaries", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)

	if err := routes.SummarizeTextWithLLM(c); err != nil {
		t.Fatalf("summarize route: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}
	var envelope struct {
		Success bool                      `json:"success"`
		Data    routes.LLMSummaryResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !envelope.Success || envelope.Data.Summary["actions"] != "follow up" {
		t.Fatalf("unexpected envelope: %#v body=%s", envelope, rec.Body.String())
	}
	if envelope.Data.ModelCode != "route-summary-fast" || envelope.Data.ChannelCode != "route-deepseek" || envelope.Data.InvocationID == "" {
		t.Fatalf("unexpected response metadata: %#v", envelope.Data)
	}
}

func seedRouteIntegrationConfig(t *testing.T, appDB *sqlx.Engine) {
	t.Helper()

	if _, err := appDB.ExecP(`
		INSERT INTO integration_credentials (id, credential_type, ciphertext, key_version, masked_value, value_text, enabled)
		VALUES (?, 'api_key', 'route-api-key', '', '', 'route-api-key', 1)
	`, "route-credential"); err != nil {
		t.Fatalf("insert credential: %v", err)
	}
	if _, err := appDB.ExecP(`
		INSERT INTO integration_channels (
			id, scenario, channel_code, provider_code, adapter_key, environment, enabled, priority, credential_id, config_json
		) VALUES (?, 'llm', 'route-deepseek', 'deepseek', 'llm.deepseek.route-test', 'test', 1, 1, ?, '{"base_url":"https://api.deepseek.com"}')
	`, "route-channel", "route-credential"); err != nil {
		t.Fatalf("insert channel: %v", err)
	}
	if _, err := appDB.ExecP(`
		INSERT INTO integration_model_options (
			id, scenario, channel_id, model_code, provider_model_id, default_params_json, enabled
		) VALUES (?, 'llm', ?, 'route-summary-fast', 'route-provider-model', '{}', 1)
	`, "route-model", "route-channel"); err != nil {
		t.Fatalf("insert model option: %v", err)
	}
	if _, err := appDB.ExecP(`
		INSERT INTO integration_operation_configs (
			id, scenario, operation, channel_code, model_code, enabled
		) VALUES (?, 'llm', 'text_summary', 'route-deepseek', 'route-summary-fast', 1)
	`, "route-text-summary-config"); err != nil {
		t.Fatalf("insert operation config: %v", err)
	}
}
