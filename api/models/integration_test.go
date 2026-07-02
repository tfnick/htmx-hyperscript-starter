package models_test

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/tfnick/go-svelte-starter/api/db"
	"github.com/tfnick/go-svelte-starter/api/framework/data/modelerror"
	"github.com/tfnick/go-svelte-starter/api/models"
	"github.com/tfnick/sqlx"
)

func TestGetEnabledLLMConfigLoadsChannelCredentialAndModel(t *testing.T) {
	manager := setupModelsTestDB(t)
	appDB, err := manager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}
	seedIntegrationConfig(t, appDB, "models-deepseek", "models-summary-fast", "models-provider-model", "models-api-key")

	config, err := models.GetEnabledLLMConfig(t.Context(), models.LLMConfigQuery{
		Scenario:  models.IntegrationScenarioLLM,
		Operation: "text_summary",
	})
	if err != nil {
		t.Fatalf("get enabled llm config: %v", err)
	}

	if config.Channel.ChannelCode != "models-deepseek" {
		t.Fatalf("unexpected channel: %#v", config.Channel)
	}
	if config.Model.ModelCode != "models-summary-fast" || config.Model.ProviderModelID != "models-provider-model" {
		t.Fatalf("unexpected model option: %#v", config.Model)
	}
	if config.Credential.ValueText != "models-api-key" {
		t.Fatalf("unexpected credential value: %q", config.Credential.ValueText)
	}
}

func TestGetEnabledLLMConfigFallsBackToDefaultDeepSeekModelForChannelOnlyConfig(t *testing.T) {
	setupModelsTestDB(t)

	credential, err := models.CreateIntegrationCredential(t.Context(), models.CreateIntegrationCredentialCmd{
		CredentialType: "api_key",
		ValueText:      "models-default-api-key",
		Enabled:        true,
	})
	if err != nil {
		t.Fatalf("create credential: %v", err)
	}
	channel, err := models.CreateIntegrationChannel(t.Context(), models.CreateIntegrationChannelCmd{
		Scenario:     models.IntegrationScenarioLLM,
		ChannelCode:  "models-deepseek-channel-only",
		ProviderCode: "deepseek",
		AdapterKey:   "llm.deepseek.openai_compatible",
		Environment:  "test",
		Enabled:      true,
		Priority:     1,
		CredentialID: credential.ID,
		ConfigJSON:   `{"base_url":"https://api.deepseek.com"}`,
		MetadataJSON: "{}",
	})
	if err != nil {
		t.Fatalf("create channel: %v", err)
	}

	config, err := models.GetEnabledLLMConfig(t.Context(), models.LLMConfigQuery{
		Scenario:  models.IntegrationScenarioLLM,
		Operation: "text_summary",
	})
	if err != nil {
		t.Fatalf("get enabled llm config: %v", err)
	}

	if config.Channel.ID != channel.ID {
		t.Fatalf("unexpected channel: %#v", config.Channel)
	}
	if config.Model.ModelCode != "deepseek-chat" || config.Model.ProviderModelID != "deepseek-chat" {
		t.Fatalf("unexpected fallback model option: %#v", config.Model)
	}
	if config.Credential.ValueText != "models-default-api-key" {
		t.Fatalf("unexpected credential value: %q", config.Credential.ValueText)
	}
}

func TestGetEnabledEmbeddingConfigFallsBackToDefaultSiliconFlowModelForChannelOnlyConfig(t *testing.T) {
	setupModelsTestDB(t)

	credential, err := models.CreateIntegrationCredential(t.Context(), models.CreateIntegrationCredentialCmd{
		CredentialType: "api_key",
		ValueText:      "models-embedding-api-key",
		Enabled:        true,
	})
	if err != nil {
		t.Fatalf("create credential: %v", err)
	}
	channel, err := models.CreateIntegrationChannel(t.Context(), models.CreateIntegrationChannelCmd{
		Scenario:     models.IntegrationScenarioEmbedding,
		ChannelCode:  "models-embedding-channel-only",
		ProviderCode: "siliconflow",
		AdapterKey:   "embedding.siliconflow.openai_compatible",
		Environment:  "test",
		Enabled:      true,
		Priority:     1,
		CredentialID: credential.ID,
		ConfigJSON:   `{"base_url":"https://api.siliconflow.cn","endpoint_path":"/v1/embeddings"}`,
		MetadataJSON: "{}",
	})
	if err != nil {
		t.Fatalf("create embedding channel: %v", err)
	}

	config, err := models.GetEnabledEmbeddingConfig(t.Context(), models.EmbeddingConfigQuery{
		Scenario:    models.IntegrationScenarioEmbedding,
		ChannelCode: channel.ChannelCode,
	})
	if err != nil {
		t.Fatalf("get enabled embedding config: %v", err)
	}

	if config.Channel.ID != channel.ID {
		t.Fatalf("unexpected channel: %#v", config.Channel)
	}
	if config.Model.ModelCode != "qwen3-embedding-0.6b" || config.Model.ProviderModelID != "Qwen/Qwen3-Embedding-0.6B" {
		t.Fatalf("unexpected fallback embedding model option: %#v", config.Model)
	}
	if config.Model.DefaultParamsJSON != `{"dimensions":64,"encoding_format":"float"}` {
		t.Fatalf("unexpected fallback embedding params: %q", config.Model.DefaultParamsJSON)
	}
	if config.Credential.ValueText != "models-embedding-api-key" {
		t.Fatalf("unexpected credential value: %q", config.Credential.ValueText)
	}
}

func TestGetEnabledEmbeddingConfigUsesDefaultSiliconFlowProvider(t *testing.T) {
	setupModelsTestDB(t)

	config, err := models.GetEnabledEmbeddingConfig(t.Context(), models.EmbeddingConfigQuery{
		Scenario:  models.IntegrationScenarioEmbedding,
		Operation: "embedding_create",
	})
	if err != nil {
		t.Fatalf("get enabled embedding config: %v", err)
	}

	if config.Channel.ChannelCode != "siliconflow-qwen3-embedding" || config.Channel.AdapterKey != "embedding.siliconflow.openai_compatible" {
		t.Fatalf("unexpected default embedding channel: %#v", config.Channel)
	}
	if config.Model.ModelCode != "qwen3-embedding-0.6b" || config.Model.ProviderModelID != "Qwen/Qwen3-Embedding-0.6B" {
		t.Fatalf("unexpected default embedding model option: %#v", config.Model)
	}
	if config.Model.DefaultParamsJSON != `{"dimensions":64,"encoding_format":"float"}` {
		t.Fatalf("unexpected fallback embedding params: %q", config.Model.DefaultParamsJSON)
	}
	if config.Credential.CredentialType != "api_key" || config.Credential.ValueText != "" {
		t.Fatalf("unexpected SiliconFlow embedding credential: %#v", config.Credential)
	}
}

func TestCreateIntegrationWebhookReceiptReturnsExistingOnDuplicateIdempotencyKey(t *testing.T) {
	manager := setupModelsTestDB(t)
	appDB, err := manager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}
	seedIntegrationConfig(t, appDB, "models-creem", "", "", "models-api-key")

	cmd := models.CreateIntegrationWebhookReceiptCmd{
		Scenario:          models.IntegrationScenarioPayment,
		ChannelID:         "models-creem-channel",
		ChannelCode:       "models-creem",
		ProviderCode:      "creem",
		ProviderEventID:   "evt_models",
		IdempotencyKey:    "evt_models",
		PayloadHash:       "payload-hash",
		PayloadCiphertext: "ciphertext",
		SafeSnapshotJSON:  `{"event_type":"checkout.completed"}`,
		HeadersHash:       "headers-hash",
		Status:            models.IntegrationWebhookReceiptStatusReceived,
	}

	first, created, err := models.CreateIntegrationWebhookReceipt(t.Context(), cmd)
	if err != nil {
		t.Fatalf("create receipt: %v", err)
	}
	if !created || first.ID == "" {
		t.Fatalf("expected first create, got created=%v receipt=%#v", created, first)
	}

	secondCmd := cmd
	secondCmd.ProviderEventID = "evt_models_changed"
	secondCmd.PayloadHash = "payload-hash-changed"
	second, created, err := models.CreateIntegrationWebhookReceipt(t.Context(), secondCmd)
	if err != nil {
		t.Fatalf("create duplicate receipt: %v", err)
	}
	if created {
		t.Fatalf("expected duplicate create=false, got receipt=%#v", second)
	}
	if second.ID != first.ID || second.ProviderEventID != first.ProviderEventID || second.PayloadHash != first.PayloadHash {
		t.Fatalf("expected existing receipt, first=%#v second=%#v", first, second)
	}
}

func TestIntegrationChannelConfigCRUDReturnsAdminCredentialValue(t *testing.T) {
	manager := setupModelsTestDB(t)
	appDB, err := manager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}

	credential, err := models.CreateIntegrationCredential(t.Context(), models.CreateIntegrationCredentialCmd{
		CredentialType: "api_key",
		ValueText:      "model-secret",
		Enabled:        true,
	})
	if err != nil {
		t.Fatalf("create credential: %v", err)
	}
	channel, err := models.CreateIntegrationChannel(t.Context(), models.CreateIntegrationChannelCmd{
		Scenario:       models.IntegrationScenarioSMS,
		ChannelCode:    "model-sms",
		ProviderCode:   "aliyun",
		AdapterKey:     "sms.aliyun.model-test",
		Environment:    "test",
		Enabled:        true,
		Priority:       20,
		CredentialID:   credential.ID,
		WebhookEnabled: false,
		ConfigJSON:     `{"base_url":"https://sms.example.com"}`,
		MetadataJSON:   `{"owner":"ops"}`,
	})
	if err != nil {
		t.Fatalf("create channel: %v", err)
	}
	if channel.CredentialValue != "model-secret" || channel.CredentialType != "api_key" {
		t.Fatalf("expected admin credential fields, got %#v", channel)
	}

	channels, err := models.ListIntegrationChannelConfigs(t.Context(), models.IntegrationScenarioSMS)
	if err != nil {
		t.Fatalf("list channels: %v", err)
	}
	if len(channels) != 1 || channels[0].ID != channel.ID {
		t.Fatalf("unexpected channels: %#v", channels)
	}

	if _, err := models.SetIntegrationChannelEnabled(t.Context(), channel.ID, false); err != nil {
		t.Fatalf("disable channel: %v", err)
	}
	updated, err := models.UpdateIntegrationChannel(t.Context(), models.UpdateIntegrationChannelCmd{
		ID:             channel.ID,
		Scenario:       models.IntegrationScenarioSMS,
		ChannelCode:    "model-sms-renamed",
		ProviderCode:   "aliyun",
		AdapterKey:     "sms.aliyun.model-test",
		Environment:    "test",
		Enabled:        true,
		Priority:       30,
		WebhookEnabled: true,
		ConfigJSON:     `{"base_url":"https://sms-renamed.example.com"}`,
		MetadataJSON:   `{"owner":"platform"}`,
	})
	if err != nil {
		t.Fatalf("update channel: %v", err)
	}
	if updated.ChannelCode != "model-sms-renamed" || !boolFromIntForTest(updated.WebhookEnabled) {
		t.Fatalf("unexpected updated channel: %#v", updated)
	}

	var storedValue string
	if err := appDB.GetP(&storedValue, `SELECT value_text FROM integration_credentials WHERE id = ?`, credential.ID); err != nil {
		t.Fatalf("load stored credential: %v", err)
	}
	if storedValue != "model-secret" {
		t.Fatalf("expected credential value in database, got %q", storedValue)
	}
}

func TestCreateIntegrationChannelReturnsConflictOnDuplicateCodeAndEnvironment(t *testing.T) {
	manager := setupModelsTestDB(t)
	appDB, err := manager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}
	seedIntegrationConfig(t, appDB, "models-duplicate", "", "", "models-api-key")

	_, err = models.CreateIntegrationChannel(t.Context(), models.CreateIntegrationChannelCmd{
		Scenario:     models.IntegrationScenarioPayment,
		ChannelCode:  "models-duplicate",
		ProviderCode: "creem",
		AdapterKey:   "payment.creem.models-test",
		Environment:  "test",
		Enabled:      true,
		Priority:     10,
		CredentialID: "models-duplicate-credential",
		ConfigJSON:   "{}",
		MetadataJSON: "{}",
	})
	if !errors.Is(err, models.ErrIntegrationChannelConflict) {
		t.Fatalf("expected conflict error, got %v", err)
	}
}

func TestOSSPrimaryChannelUniquenessAndDisable(t *testing.T) {
	setupModelsTestDB(t)

	firstCredential, err := models.CreateIntegrationCredential(t.Context(), models.CreateIntegrationCredentialCmd{
		CredentialType: "s3_access_key",
		ValueText:      `{"access_key_id":"ak1","secret_access_key":"sk1"}`,
		Enabled:        true,
	})
	if err != nil {
		t.Fatalf("create first credential: %v", err)
	}
	first, err := models.CreateIntegrationChannel(t.Context(), models.CreateIntegrationChannelCmd{
		Scenario:     models.IntegrationScenarioOSS,
		ChannelCode:  "models-r2",
		ProviderCode: "cloudflare_r2",
		AdapterKey:   "oss.cloudflare_r2.s3_compatible",
		Environment:  "production",
		Enabled:      true,
		Priority:     10,
		CredentialID: firstCredential.ID,
		IsPrimary:    true,
		ConfigJSON:   `{"endpoint_url":"https://r2.example.com","bucket":"assets","region":"auto"}`,
		MetadataJSON: "{}",
	})
	if err != nil {
		t.Fatalf("create first primary OSS channel: %v", err)
	}
	if first.IsPrimary != 1 {
		t.Fatalf("expected first channel to be primary, got %#v", first)
	}

	secondCredential, err := models.CreateIntegrationCredential(t.Context(), models.CreateIntegrationCredentialCmd{
		CredentialType: "s3_access_key",
		ValueText:      `{"access_key_id":"ak2","secret_access_key":"sk2"}`,
		Enabled:        true,
	})
	if err != nil {
		t.Fatalf("create second credential: %v", err)
	}
	_, err = models.CreateIntegrationChannel(t.Context(), models.CreateIntegrationChannelCmd{
		Scenario:     models.IntegrationScenarioOSS,
		ChannelCode:  "models-aliyun-conflict",
		ProviderCode: "aliyun",
		AdapterKey:   "oss.aliyun_oss.s3_compatible",
		Environment:  "production",
		Enabled:      true,
		Priority:     20,
		CredentialID: secondCredential.ID,
		IsPrimary:    true,
		ConfigJSON:   `{"endpoint_url":"https://oss-cn-hangzhou.aliyuncs.com","bucket":"assets","region":"cn-hangzhou"}`,
		MetadataJSON: "{}",
	})
	if !errors.Is(err, models.ErrIntegrationChannelConflict) {
		t.Fatalf("expected primary uniqueness conflict, got %v", err)
	}

	second, err := models.CreateIntegrationChannel(t.Context(), models.CreateIntegrationChannelCmd{
		Scenario:     models.IntegrationScenarioOSS,
		ChannelCode:  "models-aliyun",
		ProviderCode: "aliyun",
		AdapterKey:   "oss.aliyun_oss.s3_compatible",
		Environment:  "production",
		Enabled:      true,
		Priority:     20,
		CredentialID: secondCredential.ID,
		IsPrimary:    false,
		ConfigJSON:   `{"endpoint_url":"https://oss-cn-hangzhou.aliyuncs.com","bucket":"assets","region":"cn-hangzhou"}`,
		MetadataJSON: "{}",
	})
	if err != nil {
		t.Fatalf("create second non-primary OSS channel: %v", err)
	}
	if second.IsPrimary != 0 {
		t.Fatalf("expected second channel to be non-primary, got %#v", second)
	}

	disabled, err := models.SetIntegrationChannelEnabled(t.Context(), first.ID, false)
	if err != nil {
		t.Fatalf("disable primary channel: %v", err)
	}
	if disabled.Enabled != 0 || disabled.IsPrimary != 0 {
		t.Fatalf("expected disabling channel to clear primary flag, got %#v", disabled)
	}

	updatedSecond, err := models.UpdateIntegrationChannel(t.Context(), models.UpdateIntegrationChannelCmd{
		ID:             second.ID,
		Scenario:       models.IntegrationScenarioOSS,
		ChannelCode:    second.ChannelCode,
		ProviderCode:   second.ProviderCode,
		AdapterKey:     second.AdapterKey,
		Environment:    second.Environment,
		Enabled:        true,
		Priority:       second.Priority,
		WebhookEnabled: false,
		IsPrimary:      true,
		ConfigJSON:     second.ConfigJSON,
		MetadataJSON:   second.MetadataJSON,
	})
	if err != nil {
		t.Fatalf("promote second channel: %v", err)
	}
	if updatedSecond.IsPrimary != 1 {
		t.Fatalf("expected second channel to become primary, got %#v", updatedSecond)
	}
}

func TestGetEnabledPrimaryOSSChannelConfig(t *testing.T) {
	setupModelsTestDB(t)

	_, err := models.GetEnabledPrimaryOSSChannelConfig(t.Context())
	if !errors.Is(err, modelerror.ErrNotFound) {
		t.Fatalf("expected missing primary OSS not found error, got %v", err)
	}

	credential, err := models.CreateIntegrationCredential(t.Context(), models.CreateIntegrationCredentialCmd{
		CredentialType: "s3_access_key",
		ValueText:      `{"access_key_id":"ak","secret_access_key":"sk"}`,
		Enabled:        true,
	})
	if err != nil {
		t.Fatalf("create OSS credential: %v", err)
	}
	channel, err := models.CreateIntegrationChannel(t.Context(), models.CreateIntegrationChannelCmd{
		Scenario:     models.IntegrationScenarioOSS,
		ChannelCode:  "models-primary-oss",
		ProviderCode: "cloudflare_r2",
		AdapterKey:   "oss.cloudflare_r2.s3_compatible",
		Environment:  "test",
		Enabled:      true,
		Priority:     10,
		CredentialID: credential.ID,
		IsPrimary:    true,
		ConfigJSON:   `{"endpoint_url":"https://r2.example.com","bucket":"assets","region":"auto"}`,
		MetadataJSON: "{}",
	})
	if err != nil {
		t.Fatalf("create primary OSS channel: %v", err)
	}

	config, err := models.GetEnabledPrimaryOSSChannelConfig(t.Context())
	if err != nil {
		t.Fatalf("get primary OSS channel config: %v", err)
	}
	if config.ID != channel.ID || config.CredentialValue != `{"access_key_id":"ak","secret_access_key":"sk"}` {
		t.Fatalf("unexpected primary OSS config: %#v", config)
	}

	byMetadata, err := models.GetOSSChannelConfigByCodeAndAdapter(t.Context(), "models-primary-oss", "oss.cloudflare_r2.s3_compatible")
	if err != nil {
		t.Fatalf("get OSS channel config by code and adapter: %v", err)
	}
	if byMetadata.ID != channel.ID || byMetadata.CredentialValue != config.CredentialValue {
		t.Fatalf("unexpected OSS metadata config: %#v", byMetadata)
	}
}

func setupModelsTestDB(t *testing.T) *db.DBManager {
	t.Helper()

	previous := db.DefaultManager
	manager := db.NewDBManager()
	db.DefaultManager = manager

	dir := t.TempDir()
	t.Cleanup(func() {
		_ = manager.Close()
		db.DefaultManager = previous
	})

	if err := manager.Open("app", "sqlite", filepath.Join(dir, "app.db")); err != nil {
		t.Fatalf("open app db: %v", err)
	}
	if err := manager.AutoMigrate("app"); err != nil {
		t.Fatalf("migrate app db: %v", err)
	}
	return manager
}

func boolFromIntForTest(value int) bool {
	return value == 1
}

func seedIntegrationConfig(t *testing.T, appDB *sqlx.Engine, channelCode string, modelCode string, providerModelID string, apiKey string) {
	t.Helper()

	credentialID := channelCode + "-credential"
	channelID := channelCode + "-channel"
	modelID := modelCode + "-model"

	if _, err := appDB.ExecP(`
		INSERT INTO integration_credentials (id, credential_type, ciphertext, key_version, masked_value, value_text, enabled)
		VALUES (?, 'api_key', ?, '', '', ?, 1)
	`, credentialID, apiKey, apiKey); err != nil {
		t.Fatalf("insert integration credential: %v", err)
	}
	if _, err := appDB.ExecP(`
		INSERT INTO integration_channels (
			id, scenario, channel_code, provider_code, adapter_key, environment, enabled, priority, credential_id, config_json
		) VALUES (?, ?, ?, ?, ?, 'test', 1, 1, ?, '{"base_url":"https://api.deepseek.com"}')
	`, channelID, integrationScenarioForSeed(modelCode), channelCode, providerCodeForSeed(modelCode), adapterKeyForSeed(modelCode), credentialID); err != nil {
		t.Fatalf("insert integration channel: %v", err)
	}
	if modelCode == "" {
		return
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

func integrationScenarioForSeed(modelCode string) string {
	if modelCode == "" {
		return models.IntegrationScenarioPayment
	}
	return models.IntegrationScenarioLLM
}

func providerCodeForSeed(modelCode string) string {
	if modelCode == "" {
		return "creem"
	}
	return "deepseek"
}

func adapterKeyForSeed(modelCode string) string {
	if modelCode == "" {
		return "payment.creem.models-test"
	}
	return "llm.deepseek.openai_compatible"
}
