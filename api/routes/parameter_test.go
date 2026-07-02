package routes_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/tfnick/go-svelte-starter/api/db"
	"github.com/tfnick/go-svelte-starter/api/routes"
	"github.com/tfnick/sqlx"
)

func TestListParameterIntegrationChannelsReturnsAdminDTO(t *testing.T) {
	setupRouteTestDBs(t)
	appDB, err := db.DefaultManager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}
	seedRouteParameterChannel(t, appDB)

	router := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/parameters/integration-channels?scenario=oss", nil)
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)

	if err := routes.ListParameterIntegrationChannels(c); err != nil {
		t.Fatalf("list channels: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var envelope struct {
		Success bool                                         `json:"success"`
		Data    []routes.ParameterIntegrationChannelResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !envelope.Success || len(envelope.Data) != 1 {
		t.Fatalf("unexpected envelope: %s", rec.Body.String())
	}
	body := rec.Body.String()
	if strings.Contains(body, "ciphertext") || strings.Contains(body, "credential_plaintext") || strings.Contains(body, "credential_masked_value") {
		t.Fatalf("response leaked legacy credential fields: %s", body)
	}
	if envelope.Data[0].CredentialValue != "route-sms-secret" {
		t.Fatalf("unexpected response DTO: %#v", envelope.Data[0])
	}
	if !envelope.Data[0].IsPrimary {
		t.Fatalf("expected primary flag in response DTO: %#v", envelope.Data[0])
	}
}

func TestListParameterIntegrationChannelsReturnsModelSelectionDTO(t *testing.T) {
	setupRouteTestDBs(t)

	router := echo.New()
	body := bytes.NewBufferString(`{
		"scenario":"llm",
		"channel_code":"route-deepseek",
		"provider_code":"deepseek",
		"adapter_key":"llm.deepseek.openai_compatible",
		"environment":"test",
		"enabled":true,
		"priority":10,
		"webhook_enabled":false,
		"config_json":"{\"base_url\":\"https://api.deepseek.com\"}",
		"metadata_json":"{}",
		"credential_type":"api_key",
		"credential_value":"route-deepseek-key",
		"model_code":"deepseek-v4-flash",
		"provider_model_id":"deepseek-v4-flash",
		"operation":"chat.completion"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/api/parameters/integration-channels", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)

	if err := routes.CreateParameterIntegrationChannel(c); err != nil {
		t.Fatalf("create LLM channel: %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusCreated, rec.Code, rec.Body.String())
	}
	var createEnvelope struct {
		Success bool                                       `json:"success"`
		Data    routes.ParameterIntegrationChannelResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &createEnvelope); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if createEnvelope.Data.ModelCode != "deepseek-v4-flash" || createEnvelope.Data.ProviderModelID != "deepseek-v4-flash" {
		t.Fatalf("expected created response model selection, got %s", rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/parameters/integration-channels?scenario=llm", nil)
	rec = httptest.NewRecorder()
	c = router.NewContext(req, rec)

	if err := routes.ListParameterIntegrationChannels(c); err != nil {
		t.Fatalf("list LLM channels: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}
	var listEnvelope struct {
		Success bool                                         `json:"success"`
		Data    []routes.ParameterIntegrationChannelResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listEnvelope); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if !listEnvelope.Success || len(listEnvelope.Data) != 1 {
		t.Fatalf("unexpected list response: %s", rec.Body.String())
	}
	if listEnvelope.Data[0].ModelCode != "deepseek-v4-flash" || listEnvelope.Data[0].ProviderModelID != "deepseek-v4-flash" {
		t.Fatalf("expected listed response model selection, got %s", rec.Body.String())
	}
}

func TestListParameterIntegrationSchemasReturnsDTO(t *testing.T) {
	setupRouteTestDBs(t)

	router := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/parameters/integration-schemas?scenario=payment", nil)
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)

	if err := routes.ListParameterIntegrationSchemas(c); err != nil {
		t.Fatalf("list schemas: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var envelope struct {
		Success bool                                               `json:"success"`
		Data    []routes.ParameterIntegrationAdapterSchemaResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !envelope.Success || len(envelope.Data) != 1 {
		t.Fatalf("unexpected envelope: %s", rec.Body.String())
	}
	if envelope.Data[0].AdapterKey != "payment.creem.hosted_checkout" {
		t.Fatalf("unexpected schema response: %#v", envelope.Data[0])
	}
	if envelope.Data[0].CredentialFormat != "json_object" {
		t.Fatalf("unexpected credential format: %#v", envelope.Data[0])
	}
	if len(envelope.Data[0].ConfigFields) == 0 || len(envelope.Data[0].CredentialFields) == 0 {
		t.Fatalf("expected fields in schema response: %#v", envelope.Data[0])
	}
	baseURLField := envelope.Data[0].ConfigFields[0]
	if baseURLField.Key != "base_url" || baseURLField.Kind != "url" {
		t.Fatalf("expected payment API URL DTO field, got %#v", baseURLField)
	}
	if baseURLField.DictionaryType != "" || len(baseURLField.Options) != 0 {
		t.Fatalf("payment API URL DTO must be a free URL input, got %#v", baseURLField)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/parameters/integration-schemas?scenario=llm", nil)
	rec = httptest.NewRecorder()
	c = router.NewContext(req, rec)
	if err := routes.ListParameterIntegrationSchemas(c); err != nil {
		t.Fatalf("list LLM schemas: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	envelope = struct {
		Success bool                                               `json:"success"`
		Data    []routes.ParameterIntegrationAdapterSchemaResponse `json:"data"`
	}{}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode LLM schema response: %v", err)
	}
	if !envelope.Success || len(envelope.Data) != 2 {
		t.Fatalf("unexpected LLM schema envelope: %s", rec.Body.String())
	}
	if envelope.Data[0].ProviderCode != "deepseek" || envelope.Data[0].ModelDictionaryType != "llm_model_deepseek" {
		t.Fatalf("unexpected DeepSeek LLM schema response: %#v", envelope.Data[0])
	}
	if envelope.Data[1].ProviderCode != "siliconflow" || envelope.Data[1].ModelDictionaryType != "llm_model_siliconflow" {
		t.Fatalf("unexpected SiliconFlow LLM schema response: %#v", envelope.Data[1])
	}

	req = httptest.NewRequest(http.MethodGet, "/api/parameters/integration-schemas?scenario=embedding", nil)
	rec = httptest.NewRecorder()
	c = router.NewContext(req, rec)
	if err := routes.ListParameterIntegrationSchemas(c); err != nil {
		t.Fatalf("list Embedding schemas: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	envelope = struct {
		Success bool                                               `json:"success"`
		Data    []routes.ParameterIntegrationAdapterSchemaResponse `json:"data"`
	}{}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode Embedding schema response: %v", err)
	}
	if !envelope.Success || len(envelope.Data) != 2 {
		t.Fatalf("unexpected Embedding schema envelope: %s", rec.Body.String())
	}
	if envelope.Data[0].ProviderCode != "siliconflow" || envelope.Data[0].ModelDictionaryType != "embedding_model_siliconflow" {
		t.Fatalf("unexpected SiliconFlow Embedding schema response: %#v", envelope.Data[0])
	}
	for _, field := range envelope.Data[0].ConfigFields {
		if field.Key == "model" {
			t.Fatalf("Embedding model must not be duplicated as a config field: %#v", envelope.Data[0])
		}
	}

	req = httptest.NewRequest(http.MethodGet, "/api/parameters/integration-schemas?scenario=email", nil)
	rec = httptest.NewRecorder()
	c = router.NewContext(req, rec)
	if err := routes.ListParameterIntegrationSchemas(c); err != nil {
		t.Fatalf("list Email schemas: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	envelope = struct {
		Success bool                                               `json:"success"`
		Data    []routes.ParameterIntegrationAdapterSchemaResponse `json:"data"`
	}{}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode Email schema response: %v", err)
	}
	if !envelope.Success || len(envelope.Data) != 2 {
		t.Fatalf("unexpected Email schema envelope: %s", rec.Body.String())
	}
	if envelope.Data[0].AdapterKey != "email.aliyun.smtp" || envelope.Data[0].CredentialType != "smtp_password" {
		t.Fatalf("unexpected Aliyun Email schema response: %#v", envelope.Data[0])
	}
	if len(envelope.Data[0].ConfigFields) < 3 || envelope.Data[0].ConfigFields[2].Key != "security" || len(envelope.Data[0].ConfigFields[2].Options) == 0 {
		t.Fatalf("expected Aliyun SMTP security options, got %#v", envelope.Data[0].ConfigFields)
	}
	if envelope.Data[1].AdapterKey != "email.resend.api" || envelope.Data[1].CredentialFormat != "plain" {
		t.Fatalf("unexpected Resend Email schema response: %#v", envelope.Data[1])
	}

	req = httptest.NewRequest(http.MethodGet, "/api/parameters/integration-schemas?scenario=oss", nil)
	rec = httptest.NewRecorder()
	c = router.NewContext(req, rec)
	if err := routes.ListParameterIntegrationSchemas(c); err != nil {
		t.Fatalf("list OSS schemas: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	envelope = struct {
		Success bool                                               `json:"success"`
		Data    []routes.ParameterIntegrationAdapterSchemaResponse `json:"data"`
	}{}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode OSS schema response: %v", err)
	}
	if !envelope.Success || len(envelope.Data) != 2 {
		t.Fatalf("unexpected OSS schema envelope: %s", rec.Body.String())
	}
	if envelope.Data[0].AdapterKey != "oss.cloudflare_r2.s3_compatible" || envelope.Data[0].CredentialType != "s3_access_key" {
		t.Fatalf("unexpected Cloudflare R2 schema response: %#v", envelope.Data[0])
	}
	if len(envelope.Data[0].ConfigFields) == 0 || envelope.Data[0].ConfigFields[0].Key != "endpoint_url" {
		t.Fatalf("expected R2 endpoint URL field, got %#v", envelope.Data[0].ConfigFields)
	}
	if envelope.Data[1].AdapterKey != "oss.aliyun_oss.s3_compatible" || envelope.Data[1].ProviderCode != "aliyun" {
		t.Fatalf("unexpected Aliyun OSS schema response: %#v", envelope.Data[1])
	}
	if len(envelope.Data[1].CredentialFields) != 2 || envelope.Data[1].CredentialFields[1].Key != "secret_access_key" {
		t.Fatalf("expected Aliyun OSS access key credential fields, got %#v", envelope.Data[1].CredentialFields)
	}
}

func TestCreateParameterIntegrationChannelReturnsCreatedDTO(t *testing.T) {
	setupRouteTestDBs(t)

	router := echo.New()
	body := bytes.NewBufferString(`{
		"scenario":"payment",
		"channel_code":"route-creem",
		"provider_code":"creem",
		"adapter_key":"payment.creem.hosted_checkout",
		"environment":"test",
		"enabled":true,
		"priority":10,
		"webhook_enabled":true,
		"is_primary":true,
		"config_json":"{\"base_url\":\"https://api.creem.io\",\"product_id\":\"prod_route\"}",
		"metadata_json":"{\"owner\":\"finance\"}",
		"credential_type":"payment_bundle",
		"credential_value":"{\"api_key\":\"route-key\",\"webhook_secret\":\"route-secret\"}"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/api/parameters/integration-channels", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)

	if err := routes.CreateParameterIntegrationChannel(c); err != nil {
		t.Fatalf("create channel: %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusCreated, rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "ciphertext") || strings.Contains(rec.Body.String(), "credential_masked_value") {
		t.Fatalf("response leaked legacy credential fields: %s", rec.Body.String())
	}
	var envelope struct {
		Success bool                                       `json:"success"`
		Data    routes.ParameterIntegrationChannelResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if envelope.Data.CredentialValue != `{"api_key":"route-key","webhook_secret":"route-secret"}` {
		t.Fatalf("expected credential value in admin DTO, got %s", rec.Body.String())
	}
	if envelope.Data.IsPrimary {
		t.Fatalf("expected non-OSS primary flag to be ignored, got %#v", envelope.Data)
	}
}

func TestSetParameterIntegrationChannelEnabledReturnsUpdatedDTO(t *testing.T) {
	setupRouteTestDBs(t)
	appDB, err := db.DefaultManager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}
	seedRouteParameterChannel(t, appDB)

	router := echo.New()
	req := httptest.NewRequest(http.MethodPatch, "/api/parameters/integration-channels/route-sms-channel/enabled", bytes.NewBufferString(`{"enabled":false}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("route-sms-channel")

	if err := routes.SetParameterIntegrationChannelEnabled(c); err != nil {
		t.Fatalf("set enabled: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var envelope struct {
		Success bool                                       `json:"success"`
		Data    routes.ParameterIntegrationChannelResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !envelope.Success || envelope.Data.Enabled || envelope.Data.IsPrimary {
		t.Fatalf("expected disabled channel response, got %s", rec.Body.String())
	}
}

func seedRouteParameterChannel(t *testing.T, appDB *sqlx.Engine) {
	t.Helper()

	if _, err := appDB.ExecP(`
		INSERT INTO integration_credentials (id, credential_type, ciphertext, key_version, masked_value, value_text, enabled)
		VALUES (?, 'api_key', 'route-sms-secret', '', '', 'route-sms-secret', 1)
	`, "route-sms-credential"); err != nil {
		t.Fatalf("insert credential: %v", err)
	}
	if _, err := appDB.ExecP(`
		INSERT INTO integration_channels (
			id, scenario, channel_code, provider_code, adapter_key, environment, enabled,
			priority, credential_id, webhook_enabled, is_primary, config_json, metadata_json
		) VALUES (?, 'oss', 'route-sms', 'cloudflare_r2', 'oss.cloudflare_r2.s3_compatible', 'test', 1,
			30, ?, 0, 1, '{"endpoint_url":"https://example-account.r2.cloudflarestorage.com","bucket":"assets","region":"auto"}', '{"owner":"ops"}')
	`, "route-sms-channel", "route-sms-credential"); err != nil {
		t.Fatalf("insert channel: %v", err)
	}
}
