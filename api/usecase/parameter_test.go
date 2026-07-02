package usecase_test

import (
	"testing"

	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
	"github.com/tfnick/go-svelte-starter/api/models"
	"github.com/tfnick/go-svelte-starter/api/usecase"
	"github.com/tfnick/sqlx"
)

func TestCreateParameterIntegrationChannelStoresCredentialValueAndReturnsAdminCo(t *testing.T) {
	manager := setupUsecaseOrderTxDB(t)
	appDB, err := manager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}

	credentialValue := `{"api_key":"sk_param","webhook_secret":"whsec_param"}`
	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	channel, err := usecase.CreateParameterIntegrationChannel(ctx, usecase.SaveParameterIntegrationChannelCmd{
		Scenario:        models.IntegrationScenarioPayment,
		ChannelCode:     "param-creem",
		ProviderCode:    "creem",
		AdapterKey:      "payment.creem.hosted_checkout",
		Environment:     "test",
		Enabled:         true,
		Priority:        10,
		WebhookEnabled:  true,
		ConfigJSON:      `{"base_url":"https://api.creem.io","product_id":"prod_1"}`,
		MetadataJSON:    `{"owner":"finance"}`,
		CredentialType:  "payment_bundle",
		CredentialValue: credentialValue,
	})
	if err != nil {
		t.Fatalf("create parameter channel: %v", err)
	}
	if channel.ID == "" || channel.CredentialValue != credentialValue || channel.ConfigJSON == "" {
		t.Fatalf("unexpected channel co: %#v", channel)
	}

	var row struct {
		ValueText   string `db:"value_text"`
		Ciphertext  string `db:"ciphertext"`
		MaskedValue string `db:"masked_value"`
	}
	if err := appDB.GetP(&row, `
		SELECT cred.value_text, cred.ciphertext, cred.masked_value
		FROM integration_credentials cred
		INNER JOIN integration_channels channel ON channel.credential_id = cred.id
		WHERE channel.id = ?
	`, channel.ID); err != nil {
		t.Fatalf("load stored credential: %v", err)
	}
	if row.ValueText != credentialValue {
		t.Fatalf("expected credential value to be stored plainly, got %#v", row)
	}
	if row.Ciphertext != credentialValue || row.MaskedValue != "" {
		t.Fatalf("expected legacy credential columns to be compatibility-only, got %#v", row)
	}
}

func TestUpdateParameterIntegrationChannelPreservesOrUpdatesCredentialValue(t *testing.T) {
	manager := setupUsecaseOrderTxDB(t)
	appDB, err := manager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}
	seedParameterChannel(t, appDB)

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	before := loadParameterCredentialValue(t, appDB, "param-sms-channel")
	updated, err := usecase.UpdateParameterIntegrationChannel(ctx, usecase.SaveParameterIntegrationChannelCmd{
		ID:             "param-sms-channel",
		Scenario:       models.IntegrationScenarioSMS,
		ChannelCode:    "param-sms",
		ProviderCode:   "aliyun",
		AdapterKey:     "sms.aliyun.adapter",
		Environment:    "test",
		Enabled:        true,
		Priority:       40,
		WebhookEnabled: false,
		ConfigJSON:     `{"base_url":"https://sms.example.com"}`,
		MetadataJSON:   `{"owner":"ops"}`,
		CredentialType: "api_key",
	})
	if err != nil {
		t.Fatalf("update without credential value: %v", err)
	}
	if updated.CredentialValue != "sms-secret" {
		t.Fatalf("expected existing credential value, got %#v", updated)
	}
	if after := loadParameterCredentialValue(t, appDB, "param-sms-channel"); after != before {
		t.Fatal("expected credential value to be preserved when request value is empty")
	}

	changed, err := usecase.UpdateParameterIntegrationChannel(ctx, usecase.SaveParameterIntegrationChannelCmd{
		ID:              "param-sms-channel",
		Scenario:        models.IntegrationScenarioSMS,
		ChannelCode:     "param-sms",
		ProviderCode:    "aliyun",
		AdapterKey:      "sms.aliyun.adapter",
		Environment:     "test",
		Enabled:         true,
		Priority:        50,
		WebhookEnabled:  true,
		ConfigJSON:      `{"base_url":"https://sms-rotated.example.com"}`,
		MetadataJSON:    `{"owner":"platform"}`,
		CredentialType:  "api_key",
		CredentialValue: "new-sms-secret",
	})
	if err != nil {
		t.Fatalf("update with credential value: %v", err)
	}
	if changed.CredentialValue != "new-sms-secret" {
		t.Fatalf("expected updated credential value, got %#v", changed)
	}
	if afterChange := loadParameterCredentialValue(t, appDB, "param-sms-channel"); afterChange != "new-sms-secret" {
		t.Fatalf("unexpected stored credential value: %q", afterChange)
	}
}

func TestParameterIntegrationChannelRejectsSensitiveConfigKeys(t *testing.T) {
	setupUsecaseOrderTxDB(t)

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	_, err := usecase.CreateParameterIntegrationChannel(ctx, usecase.SaveParameterIntegrationChannelCmd{
		Scenario:        models.IntegrationScenarioLLM,
		ChannelCode:     "bad-deepseek",
		ProviderCode:    "deepseek",
		AdapterKey:      "llm.deepseek.openai_compatible",
		Enabled:         true,
		ConfigJSON:      `{"api_key":"should-not-be-here"}`,
		MetadataJSON:    "{}",
		CredentialType:  "api_key",
		CredentialValue: "secret",
	})
	if fwusecase.CodeOf(err) != fwusecase.CodeValidation {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestParameterIntegrationChannelRejectsSensitiveConfigKeysInsideArrays(t *testing.T) {
	setupUsecaseOrderTxDB(t)

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	_, err := usecase.CreateParameterIntegrationChannel(ctx, usecase.SaveParameterIntegrationChannelCmd{
		Scenario:        models.IntegrationScenarioSMS,
		ChannelCode:     "bad-sms-array",
		ProviderCode:    "aliyun",
		AdapterKey:      "sms.aliyun.adapter",
		Enabled:         true,
		ConfigJSON:      `{"items":[{"api_key":"should-not-be-here"}]}`,
		MetadataJSON:    "{}",
		CredentialType:  "api_key",
		CredentialValue: "secret",
	})
	if fwusecase.CodeOf(err) != fwusecase.CodeValidation {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestParameterIntegrationChannelRejectsCredentialTypeOutsideDictionary(t *testing.T) {
	setupUsecaseOrderTxDB(t)

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	_, err := usecase.CreateParameterIntegrationChannel(ctx, usecase.SaveParameterIntegrationChannelCmd{
		Scenario:        models.IntegrationScenarioLLM,
		ChannelCode:     "bad-credential-type",
		ProviderCode:    "custom",
		AdapterKey:      "custom.llm.adapter",
		Enabled:         true,
		ConfigJSON:      "{}",
		MetadataJSON:    "{}",
		CredentialType:  "raw_password",
		CredentialValue: "secret",
	})
	if fwusecase.CodeOf(err) != fwusecase.CodeValidation {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestListParameterIntegrationSchemasFiltersByScenario(t *testing.T) {
	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)

	schemas, err := usecase.ListParameterIntegrationSchemas(ctx, usecase.ListParameterIntegrationSchemasQry{
		Scenario: models.IntegrationScenarioPayment,
	})
	if err != nil {
		t.Fatalf("list schemas: %v", err)
	}
	if len(schemas) != 1 {
		t.Fatalf("expected one payment schema, got %#v", schemas)
	}
	if schemas[0].AdapterKey != "payment.creem.hosted_checkout" {
		t.Fatalf("unexpected schema: %#v", schemas[0])
	}
	if schemas[0].CredentialFormat != usecase.ParameterIntegrationCredentialFormatJSONObject {
		t.Fatalf("expected json object credential schema, got %#v", schemas[0])
	}
	if len(schemas[0].ConfigFields) == 0 || len(schemas[0].CredentialFields) == 0 {
		t.Fatalf("expected config and credential fields, got %#v", schemas[0])
	}
	baseURLField := schemas[0].ConfigFields[0]
	if baseURLField.Key != "base_url" || baseURLField.Kind != usecase.ParameterIntegrationSchemaFieldURL {
		t.Fatalf("expected payment API URL field, got %#v", baseURLField)
	}
	if baseURLField.DictionaryType != "" || len(baseURLField.Options) != 0 {
		t.Fatalf("payment API URL must be a free URL input, got %#v", baseURLField)
	}

	llmSchemas, err := usecase.ListParameterIntegrationSchemas(ctx, usecase.ListParameterIntegrationSchemasQry{
		Scenario: models.IntegrationScenarioLLM,
	})
	if err != nil {
		t.Fatalf("list LLM schemas: %v", err)
	}
	if len(llmSchemas) != 2 {
		t.Fatalf("expected two LLM schemas, got %#v", llmSchemas)
	}
	if llmSchemas[0].AdapterKey != "llm.deepseek.openai_compatible" || llmSchemas[0].ProviderCode != "deepseek" {
		t.Fatalf("unexpected DeepSeek LLM schema: %#v", llmSchemas[0])
	}
	if llmSchemas[0].ModelDictionaryType != "llm_model_deepseek" {
		t.Fatalf("expected DeepSeek LLM model dictionary, got %#v", llmSchemas[0])
	}
	if llmSchemas[1].AdapterKey != "llm.siliconflow.openai_compatible" || llmSchemas[1].ProviderCode != "siliconflow" {
		t.Fatalf("unexpected SiliconFlow LLM schema: %#v", llmSchemas[1])
	}
	if llmSchemas[1].ModelDictionaryType != "llm_model_siliconflow" {
		t.Fatalf("expected SiliconFlow LLM model dictionary, got %#v", llmSchemas[1])
	}

	embeddingSchemas, err := usecase.ListParameterIntegrationSchemas(ctx, usecase.ListParameterIntegrationSchemasQry{
		Scenario: models.IntegrationScenarioEmbedding,
	})
	if err != nil {
		t.Fatalf("list Embedding schemas: %v", err)
	}
	if len(embeddingSchemas) != 2 {
		t.Fatalf("expected two Embedding schemas, got %#v", embeddingSchemas)
	}
	if embeddingSchemas[0].AdapterKey != "embedding.siliconflow.openai_compatible" || embeddingSchemas[0].ProviderCode != "siliconflow" {
		t.Fatalf("unexpected default Embedding schema: %#v", embeddingSchemas[0])
	}
	if embeddingSchemas[0].CredentialType != "api_key" || embeddingSchemas[0].CredentialFormat != usecase.ParameterIntegrationCredentialFormatPlain {
		t.Fatalf("unexpected SiliconFlow Embedding credential schema: %#v", embeddingSchemas[0])
	}
	if embeddingSchemas[0].ModelDictionaryType != "embedding_model_siliconflow" {
		t.Fatalf("expected SiliconFlow Embedding model dictionary, got %#v", embeddingSchemas[0])
	}
	embeddingBaseURLField, ok := schemaFieldByKey(embeddingSchemas[0].ConfigFields, "base_url")
	if !ok || embeddingBaseURLField.Kind != usecase.ParameterIntegrationSchemaFieldURL || embeddingBaseURLField.DefaultValue != "https://api.siliconflow.cn" {
		t.Fatalf("unexpected Embedding base_url field: %#v", embeddingBaseURLField)
	}
	if embeddingModelField, ok := schemaFieldByKey(embeddingSchemas[0].ConfigFields, "model"); ok {
		t.Fatalf("Embedding model must use top-level model_code/provider_model_id, got config field: %#v", embeddingModelField)
	}
	embeddingDimensionsField, ok := schemaFieldByKey(embeddingSchemas[0].ConfigFields, "dimensions")
	if !ok || embeddingDimensionsField.Kind != usecase.ParameterIntegrationSchemaFieldNumber || embeddingDimensionsField.DefaultValue != "64" {
		t.Fatalf("unexpected Embedding dimensions field: %#v", embeddingDimensionsField)
	}
	embeddingEncodingField, ok := schemaFieldByKey(embeddingSchemas[0].ConfigFields, "encoding_format")
	if !ok || embeddingEncodingField.DefaultValue != "float" || len(embeddingEncodingField.Options) != 1 {
		t.Fatalf("unexpected Embedding encoding_format field: %#v", embeddingEncodingField)
	}
	embeddingEndpointPathField, ok := schemaFieldByKey(embeddingSchemas[0].ConfigFields, "endpoint_path")
	if !ok || embeddingEndpointPathField.DefaultValue != "/v1/embeddings" {
		t.Fatalf("unexpected Embedding endpoint_path field: %#v", embeddingEndpointPathField)
	}
	if embeddingSchemas[1].AdapterKey != "embedding.local_hash_64" || embeddingSchemas[1].ProviderCode != "local" {
		t.Fatalf("unexpected local Embedding schema: %#v", embeddingSchemas[1])
	}
	if embeddingSchemas[1].CredentialType != "none" || len(embeddingSchemas[1].CredentialFields) != 0 {
		t.Fatalf("unexpected local Embedding credential schema: %#v", embeddingSchemas[1])
	}

	smsSchemas, err := usecase.ListParameterIntegrationSchemas(ctx, usecase.ListParameterIntegrationSchemasQry{
		Scenario: models.IntegrationScenarioSMS,
	})
	if err != nil {
		t.Fatalf("list SMS schemas: %v", err)
	}
	if len(smsSchemas) != 1 {
		t.Fatalf("expected one SMS schema, got %#v", smsSchemas)
	}
	if smsSchemas[0].AdapterKey != "sms.aliyun.adapter" {
		t.Fatalf("unexpected SMS schema: %#v", smsSchemas[0])
	}
	if smsSchemas[0].ProviderCode != "aliyun" || smsSchemas[0].CredentialFormat != usecase.ParameterIntegrationCredentialFormatPlain {
		t.Fatalf("unexpected SMS schema contract: %#v", smsSchemas[0])
	}

	emailSchemas, err := usecase.ListParameterIntegrationSchemas(ctx, usecase.ListParameterIntegrationSchemasQry{
		Scenario: models.IntegrationScenarioEmail,
	})
	if err != nil {
		t.Fatalf("list Email schemas: %v", err)
	}
	if len(emailSchemas) != 2 {
		t.Fatalf("expected two Email schemas, got %#v", emailSchemas)
	}
	if emailSchemas[0].AdapterKey != "email.aliyun.smtp" || emailSchemas[1].AdapterKey != "email.resend.api" {
		t.Fatalf("unexpected Email schemas: %#v", emailSchemas)
	}
	if emailSchemas[0].CredentialType != "smtp_password" || emailSchemas[0].CredentialFormat != usecase.ParameterIntegrationCredentialFormatJSONObject {
		t.Fatalf("unexpected Aliyun Email schema: %#v", emailSchemas[0])
	}
	if emailSchemas[1].CredentialType != "api_key" || emailSchemas[1].CredentialFormat != usecase.ParameterIntegrationCredentialFormatPlain {
		t.Fatalf("unexpected Resend Email schema: %#v", emailSchemas[1])
	}

	ossSchemas, err := usecase.ListParameterIntegrationSchemas(ctx, usecase.ListParameterIntegrationSchemasQry{
		Scenario: models.IntegrationScenarioOSS,
	})
	if err != nil {
		t.Fatalf("list OSS schemas: %v", err)
	}
	if len(ossSchemas) != 2 {
		t.Fatalf("expected two OSS schemas, got %#v", ossSchemas)
	}
	if ossSchemas[0].AdapterKey != "oss.cloudflare_r2.s3_compatible" || ossSchemas[0].ProviderCode != "cloudflare_r2" {
		t.Fatalf("unexpected Cloudflare R2 schema: %#v", ossSchemas[0])
	}
	if ossSchemas[0].CredentialType != "s3_access_key" || ossSchemas[0].CredentialFormat != usecase.ParameterIntegrationCredentialFormatJSONObject {
		t.Fatalf("unexpected Cloudflare R2 credential schema: %#v", ossSchemas[0])
	}
	if len(ossSchemas[0].ConfigFields) < 3 || ossSchemas[0].ConfigFields[0].Key != "endpoint_url" || ossSchemas[0].ConfigFields[2].DefaultValue != "auto" {
		t.Fatalf("unexpected Cloudflare R2 config fields: %#v", ossSchemas[0].ConfigFields)
	}
	r2PathStyleField, ok := schemaFieldByKey(ossSchemas[0].ConfigFields, "use_path_style")
	if !ok || r2PathStyleField.Kind != usecase.ParameterIntegrationSchemaFieldBoolean || r2PathStyleField.DefaultValue != "true" {
		t.Fatalf("unexpected Cloudflare R2 path-style field: %#v", r2PathStyleField)
	}
	if ossSchemas[1].AdapterKey != "oss.aliyun_oss.s3_compatible" || ossSchemas[1].ProviderCode != "aliyun" {
		t.Fatalf("unexpected Aliyun OSS schema: %#v", ossSchemas[1])
	}
	if ossSchemas[1].CredentialType != "s3_access_key" || ossSchemas[1].CredentialFormat != usecase.ParameterIntegrationCredentialFormatJSONObject {
		t.Fatalf("unexpected Aliyun OSS credential schema: %#v", ossSchemas[1])
	}
	if len(ossSchemas[1].ConfigFields) < 3 || ossSchemas[1].ConfigFields[0].Key != "endpoint_url" || ossSchemas[1].ConfigFields[2].Placeholder != "cn-hangzhou" {
		t.Fatalf("unexpected Aliyun OSS config fields: %#v", ossSchemas[1].ConfigFields)
	}
	aliyunPathStyleField, ok := schemaFieldByKey(ossSchemas[1].ConfigFields, "use_path_style")
	if !ok || aliyunPathStyleField.Kind != usecase.ParameterIntegrationSchemaFieldBoolean || aliyunPathStyleField.DefaultValue != "" {
		t.Fatalf("unexpected Aliyun OSS path-style field: %#v", aliyunPathStyleField)
	}
}

func TestParameterIntegrationChannelValidatesAdapterSchema(t *testing.T) {
	setupUsecaseOrderTxDB(t)
	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)

	base := usecase.SaveParameterIntegrationChannelCmd{
		Scenario:        models.IntegrationScenarioPayment,
		ChannelCode:     "schema-creem",
		ProviderCode:    "creem",
		AdapterKey:      "payment.creem.hosted_checkout",
		Environment:     "test",
		Enabled:         true,
		ConfigJSON:      `{"base_url":"https://api.creem.io","product_id":"prod_schema"}`,
		MetadataJSON:    "{}",
		CredentialType:  "payment_bundle",
		CredentialValue: `{"api_key":"sk_schema","webhook_secret":"whsec_schema"}`,
	}

	missingProduct := base
	missingProduct.ChannelCode = "schema-missing-product"
	missingProduct.ConfigJSON = `{"base_url":"https://api.creem.io"}`
	if _, err := usecase.CreateParameterIntegrationChannel(ctx, missingProduct); fwusecase.CodeOf(err) != fwusecase.CodeValidation {
		t.Fatalf("expected missing product_id validation, got %v", err)
	}

	invalidURL := base
	invalidURL.ChannelCode = "schema-invalid-url"
	invalidURL.ConfigJSON = `{"base_url":"creem","product_id":"prod_schema"}`
	if _, err := usecase.CreateParameterIntegrationChannel(ctx, invalidURL); fwusecase.CodeOf(err) != fwusecase.CodeValidation {
		t.Fatalf("expected invalid URL validation, got %v", err)
	}

	missingSecret := base
	missingSecret.ChannelCode = "schema-missing-secret"
	missingSecret.CredentialValue = `{"api_key":"sk_schema"}`
	if _, err := usecase.CreateParameterIntegrationChannel(ctx, missingSecret); fwusecase.CodeOf(err) != fwusecase.CodeValidation {
		t.Fatalf("expected missing webhook secret validation, got %v", err)
	}

	if _, err := usecase.CreateParameterIntegrationChannel(ctx, base); err != nil {
		t.Fatalf("expected schema-valid channel to be created: %v", err)
	}
}

func TestParameterIntegrationChannelAcceptsPlainCredentialSchema(t *testing.T) {
	setupUsecaseOrderTxDB(t)
	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)

	channel, err := usecase.CreateParameterIntegrationChannel(ctx, usecase.SaveParameterIntegrationChannelCmd{
		Scenario:        models.IntegrationScenarioLLM,
		ChannelCode:     "schema-deepseek",
		ProviderCode:    "deepseek",
		AdapterKey:      "llm.deepseek.openai_compatible",
		Environment:     "test",
		Enabled:         true,
		ConfigJSON:      `{"base_url":"https://api.deepseek.com"}`,
		MetadataJSON:    "{}",
		CredentialType:  "api_key",
		CredentialValue: "sk_deepseek",
	})
	if err != nil {
		t.Fatalf("create LLM channel: %v", err)
	}
	if channel.CredentialType != "api_key" || channel.CredentialValue != "sk_deepseek" {
		t.Fatalf("unexpected channel: %#v", channel)
	}

	modelChannel, err := usecase.CreateParameterIntegrationChannel(ctx, usecase.SaveParameterIntegrationChannelCmd{
		Scenario:        models.IntegrationScenarioLLM,
		ChannelCode:     "schema-deepseek-with-model",
		ProviderCode:    "deepseek",
		AdapterKey:      "llm.deepseek.openai_compatible",
		Environment:     "test",
		Enabled:         true,
		ConfigJSON:      `{"base_url":"https://api.deepseek.com"}`,
		MetadataJSON:    "{}",
		CredentialType:  "api_key",
		CredentialValue: "sk_deepseek_model",
		ModelCode:       "deepseek-v4-flash",
		ProviderModelID: "deepseek-v4-flash",
		Operation:       "chat.completion",
	})
	if err != nil {
		t.Fatalf("create LLM channel with model: %v", err)
	}
	if modelChannel.ModelCode != "deepseek-v4-flash" || modelChannel.ProviderModelID != "deepseek-v4-flash" {
		t.Fatalf("expected created channel to return model selection, got %#v", modelChannel)
	}
	modelChannels, err := usecase.ListParameterIntegrationChannels(ctx, usecase.ListParameterIntegrationChannelsQry{
		Scenario: models.IntegrationScenarioLLM,
	})
	if err != nil {
		t.Fatalf("list LLM channels: %v", err)
	}
	listedModelChannel := parameterChannelByCode(modelChannels, "schema-deepseek-with-model")
	if listedModelChannel.ModelCode != "deepseek-v4-flash" || listedModelChannel.ProviderModelID != "deepseek-v4-flash" {
		t.Fatalf("expected listed channel to return model selection, got %#v", listedModelChannel)
	}
	updatedModelChannel, err := usecase.UpdateParameterIntegrationChannel(ctx, usecase.SaveParameterIntegrationChannelCmd{
		ID:              modelChannel.ID,
		Scenario:        models.IntegrationScenarioLLM,
		ChannelCode:     "schema-deepseek-with-model",
		ProviderCode:    "deepseek",
		AdapterKey:      "llm.deepseek.openai_compatible",
		Environment:     "test",
		Enabled:         true,
		ConfigJSON:      `{"base_url":"https://api.deepseek.com"}`,
		MetadataJSON:    "{}",
		CredentialType:  "api_key",
		CredentialValue: "",
		ModelCode:       "deepseek-v4-pro",
		ProviderModelID: "deepseek-v4-pro",
		Operation:       "chat.completion",
	})
	if err != nil {
		t.Fatalf("update LLM channel model: %v", err)
	}
	if updatedModelChannel.ModelCode != "deepseek-v4-pro" || updatedModelChannel.ProviderModelID != "deepseek-v4-pro" {
		t.Fatalf("expected updated channel to return new model selection, got %#v", updatedModelChannel)
	}

	embeddingChannel, err := usecase.CreateParameterIntegrationChannel(ctx, usecase.SaveParameterIntegrationChannelCmd{
		Scenario:        models.IntegrationScenarioEmbedding,
		ChannelCode:     "schema-siliconflow-embedding",
		ProviderCode:    "siliconflow",
		AdapterKey:      "embedding.siliconflow.openai_compatible",
		Environment:     "test",
		Enabled:         true,
		ConfigJSON:      `{"base_url":"https://api.siliconflow.cn","dimensions":64,"encoding_format":"float","endpoint_path":"/v1/embeddings"}`,
		MetadataJSON:    "{}",
		CredentialType:  "api_key",
		CredentialValue: "sk_embedding",
		ModelCode:       "Qwen/Qwen3-Embedding-4B",
		ProviderModelID: "Qwen/Qwen3-Embedding-4B",
		Operation:       "embedding_create",
	})
	if err != nil {
		t.Fatalf("create Embedding channel: %v", err)
	}
	if embeddingChannel.Scenario != models.IntegrationScenarioEmbedding || embeddingChannel.CredentialValue != "sk_embedding" {
		t.Fatalf("unexpected Embedding channel: %#v", embeddingChannel)
	}
	if embeddingChannel.ModelCode != "qwen3-embedding-4b" || embeddingChannel.ProviderModelID != "Qwen/Qwen3-Embedding-4B" {
		t.Fatalf("expected Embedding model selection to be normalized and returned, got %#v", embeddingChannel)
	}
}

func TestParameterIntegrationChannelAcceptsLocalEmbeddingWithoutCredential(t *testing.T) {
	setupUsecaseOrderTxDB(t)
	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)

	channel, err := usecase.CreateParameterIntegrationChannel(ctx, usecase.SaveParameterIntegrationChannelCmd{
		Scenario:        models.IntegrationScenarioEmbedding,
		ChannelCode:     "schema-local-embedding",
		ProviderCode:    "local",
		AdapterKey:      "embedding.local_hash_64",
		Environment:     "test",
		Enabled:         true,
		ConfigJSON:      "{}",
		MetadataJSON:    "{}",
		CredentialType:  "none",
		CredentialValue: "",
	})
	if err != nil {
		t.Fatalf("create local Embedding channel without credential: %v", err)
	}
	if channel.Scenario != models.IntegrationScenarioEmbedding || channel.CredentialType != "none" || channel.CredentialValue != "" {
		t.Fatalf("unexpected local Embedding channel: %#v", channel)
	}
}

func TestParameterIntegrationChannelClearsCredentialWhenUpdatedToLocalEmbedding(t *testing.T) {
	setupUsecaseOrderTxDB(t)
	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)

	channel, err := usecase.CreateParameterIntegrationChannel(ctx, usecase.SaveParameterIntegrationChannelCmd{
		Scenario:        models.IntegrationScenarioEmbedding,
		ChannelCode:     "schema-embedding-switch",
		ProviderCode:    "siliconflow",
		AdapterKey:      "embedding.siliconflow.openai_compatible",
		Environment:     "test",
		Enabled:         true,
		ConfigJSON:      `{"base_url":"https://api.siliconflow.cn","model":"Qwen/Qwen3-Embedding-0.6B","dimensions":64,"encoding_format":"float","endpoint_path":"/v1/embeddings"}`,
		MetadataJSON:    "{}",
		CredentialType:  "api_key",
		CredentialValue: "sk_embedding",
	})
	if err != nil {
		t.Fatalf("create external Embedding channel: %v", err)
	}

	updated, err := usecase.UpdateParameterIntegrationChannel(ctx, usecase.SaveParameterIntegrationChannelCmd{
		ID:              channel.ID,
		Scenario:        models.IntegrationScenarioEmbedding,
		ChannelCode:     channel.ChannelCode,
		ProviderCode:    "local",
		AdapterKey:      "embedding.local_hash_64",
		Environment:     channel.Environment,
		Enabled:         true,
		Priority:        channel.Priority,
		ConfigJSON:      "{}",
		MetadataJSON:    "{}",
		CredentialType:  "none",
		CredentialValue: "",
	})
	if err != nil {
		t.Fatalf("update Embedding channel to local: %v", err)
	}
	if updated.CredentialType != "none" || updated.CredentialValue != "" {
		t.Fatalf("expected local Embedding update to clear credential, got %#v", updated)
	}
}

func TestParameterIntegrationChannelAcceptsEmailSMTPJSONCredentialSchema(t *testing.T) {
	setupUsecaseOrderTxDB(t)
	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)

	channel, err := usecase.CreateParameterIntegrationChannel(ctx, usecase.SaveParameterIntegrationChannelCmd{
		Scenario:       models.IntegrationScenarioEmail,
		ChannelCode:    "aliyun-email",
		ProviderCode:   "aliyun",
		AdapterKey:     "email.aliyun.smtp",
		Environment:    "production",
		Enabled:        true,
		ConfigJSON:     `{"smtp_host":"smtp.qiye.aliyun.com","smtp_port":465,"security":"ssl","from_email":"noreply@example.com"}`,
		MetadataJSON:   "{}",
		CredentialType: "smtp_password",
		CredentialValue: `{
			"username":"noreply@example.com",
			"password":"mailbox-secret"
		}`,
	})
	if err != nil {
		t.Fatalf("create Aliyun Email channel: %v", err)
	}
	if channel.Scenario != models.IntegrationScenarioEmail || channel.CredentialType != "smtp_password" {
		t.Fatalf("unexpected Email channel: %#v", channel)
	}

	badSecurity := usecase.SaveParameterIntegrationChannelCmd{
		Scenario:        models.IntegrationScenarioEmail,
		ChannelCode:     "bad-email-security",
		ProviderCode:    "aliyun",
		AdapterKey:      "email.aliyun.smtp",
		Environment:     "production",
		Enabled:         true,
		ConfigJSON:      `{"smtp_host":"smtp.qiye.aliyun.com","smtp_port":465,"security":"invalid","from_email":"noreply@example.com"}`,
		MetadataJSON:    "{}",
		CredentialType:  "smtp_password",
		CredentialValue: `{"username":"noreply@example.com","password":"mailbox-secret"}`,
	}
	if _, err := usecase.CreateParameterIntegrationChannel(ctx, badSecurity); fwusecase.CodeOf(err) != fwusecase.CodeValidation {
		t.Fatalf("expected invalid SMTP security validation, got %v", err)
	}
}

func TestParameterIntegrationChannelAcceptsEmailResendPlainCredentialSchema(t *testing.T) {
	setupUsecaseOrderTxDB(t)
	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)

	channel, err := usecase.CreateParameterIntegrationChannel(ctx, usecase.SaveParameterIntegrationChannelCmd{
		Scenario:        models.IntegrationScenarioEmail,
		ChannelCode:     "resend-email",
		ProviderCode:    "resend",
		AdapterKey:      "email.resend.api",
		Environment:     "production",
		Enabled:         true,
		ConfigJSON:      `{"base_url":"https://api.resend.com","from_email":"noreply@example.com"}`,
		MetadataJSON:    "{}",
		CredentialType:  "api_key",
		CredentialValue: "re_secret",
	})
	if err != nil {
		t.Fatalf("create Resend Email channel: %v", err)
	}
	if channel.CredentialType != "api_key" || channel.CredentialValue != "re_secret" {
		t.Fatalf("unexpected Resend Email channel: %#v", channel)
	}
}

func TestParameterIntegrationChannelAcceptsCloudflareR2CredentialSchema(t *testing.T) {
	setupUsecaseOrderTxDB(t)
	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)

	channel, err := usecase.CreateParameterIntegrationChannel(ctx, usecase.SaveParameterIntegrationChannelCmd{
		Scenario:       models.IntegrationScenarioOSS,
		ChannelCode:    "r2-assets",
		ProviderCode:   "cloudflare_r2",
		AdapterKey:     "oss.cloudflare_r2.s3_compatible",
		Environment:    "production",
		Enabled:        true,
		ConfigJSON:     `{"endpoint_url":"https://example-account.r2.cloudflarestorage.com","bucket":"assets","region":"auto","key_prefix":"uploads/"}`,
		MetadataJSON:   "{}",
		CredentialType: "s3_access_key",
		CredentialValue: `{
			"access_key_id":"r2-access-key",
			"secret_access_key":"r2-secret-key"
		}`,
	})
	if err != nil {
		t.Fatalf("create Cloudflare R2 channel: %v", err)
	}
	if channel.Scenario != models.IntegrationScenarioOSS || channel.CredentialType != "s3_access_key" {
		t.Fatalf("unexpected Cloudflare R2 channel: %#v", channel)
	}

	missingBucket := usecase.SaveParameterIntegrationChannelCmd{
		Scenario:        models.IntegrationScenarioOSS,
		ChannelCode:     "r2-missing-bucket",
		ProviderCode:    "cloudflare_r2",
		AdapterKey:      "oss.cloudflare_r2.s3_compatible",
		Environment:     "production",
		Enabled:         true,
		ConfigJSON:      `{"endpoint_url":"https://example-account.r2.cloudflarestorage.com","region":"auto"}`,
		MetadataJSON:    "{}",
		CredentialType:  "s3_access_key",
		CredentialValue: `{"access_key_id":"r2-access-key","secret_access_key":"r2-secret-key"}`,
	}
	if _, err := usecase.CreateParameterIntegrationChannel(ctx, missingBucket); fwusecase.CodeOf(err) != fwusecase.CodeValidation {
		t.Fatalf("expected missing bucket validation, got %v", err)
	}

	missingSecret := usecase.SaveParameterIntegrationChannelCmd{
		Scenario:        models.IntegrationScenarioOSS,
		ChannelCode:     "r2-missing-secret",
		ProviderCode:    "cloudflare_r2",
		AdapterKey:      "oss.cloudflare_r2.s3_compatible",
		Environment:     "production",
		Enabled:         true,
		ConfigJSON:      `{"endpoint_url":"https://example-account.r2.cloudflarestorage.com","bucket":"assets","region":"auto"}`,
		MetadataJSON:    "{}",
		CredentialType:  "s3_access_key",
		CredentialValue: `{"access_key_id":"r2-access-key"}`,
	}
	if _, err := usecase.CreateParameterIntegrationChannel(ctx, missingSecret); fwusecase.CodeOf(err) != fwusecase.CodeValidation {
		t.Fatalf("expected missing secret validation, got %v", err)
	}
}

func TestParameterIntegrationChannelAcceptsAliyunOSSCredentialSchema(t *testing.T) {
	setupUsecaseOrderTxDB(t)
	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)

	channel, err := usecase.CreateParameterIntegrationChannel(ctx, usecase.SaveParameterIntegrationChannelCmd{
		Scenario:       models.IntegrationScenarioOSS,
		ChannelCode:    "aliyun-assets",
		ProviderCode:   "aliyun",
		AdapterKey:     "oss.aliyun_oss.s3_compatible",
		Environment:    "production",
		Enabled:        true,
		ConfigJSON:     `{"endpoint_url":"https://oss-cn-hangzhou.aliyuncs.com","bucket":"assets","region":"cn-hangzhou","key_prefix":"uploads/"}`,
		MetadataJSON:   "{}",
		CredentialType: "s3_access_key",
		CredentialValue: `{
			"access_key_id":"aliyun-access-key",
			"secret_access_key":"aliyun-secret-key"
		}`,
	})
	if err != nil {
		t.Fatalf("create Aliyun OSS channel: %v", err)
	}
	if channel.Scenario != models.IntegrationScenarioOSS || channel.ProviderCode != "aliyun" || channel.CredentialType != "s3_access_key" {
		t.Fatalf("unexpected Aliyun OSS channel: %#v", channel)
	}

	missingEndpoint := usecase.SaveParameterIntegrationChannelCmd{
		Scenario:        models.IntegrationScenarioOSS,
		ChannelCode:     "aliyun-missing-endpoint",
		ProviderCode:    "aliyun",
		AdapterKey:      "oss.aliyun_oss.s3_compatible",
		Environment:     "production",
		Enabled:         true,
		ConfigJSON:      `{"bucket":"assets","region":"cn-hangzhou"}`,
		MetadataJSON:    "{}",
		CredentialType:  "s3_access_key",
		CredentialValue: `{"access_key_id":"aliyun-access-key","secret_access_key":"aliyun-secret-key"}`,
	}
	if _, err := usecase.CreateParameterIntegrationChannel(ctx, missingEndpoint); fwusecase.CodeOf(err) != fwusecase.CodeValidation {
		t.Fatalf("expected missing endpoint validation, got %v", err)
	}

	missingSecret := usecase.SaveParameterIntegrationChannelCmd{
		Scenario:        models.IntegrationScenarioOSS,
		ChannelCode:     "aliyun-missing-secret",
		ProviderCode:    "aliyun",
		AdapterKey:      "oss.aliyun_oss.s3_compatible",
		Environment:     "production",
		Enabled:         true,
		ConfigJSON:      `{"endpoint_url":"https://oss-cn-hangzhou.aliyuncs.com","bucket":"assets","region":"cn-hangzhou"}`,
		MetadataJSON:    "{}",
		CredentialType:  "s3_access_key",
		CredentialValue: `{"access_key_id":"aliyun-access-key"}`,
	}
	if _, err := usecase.CreateParameterIntegrationChannel(ctx, missingSecret); fwusecase.CodeOf(err) != fwusecase.CodeValidation {
		t.Fatalf("expected missing secret validation, got %v", err)
	}
}

func TestOSSPrimaryProviderIsAtMostOneAndCanBeEmpty(t *testing.T) {
	setupUsecaseOrderTxDB(t)
	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)

	first, err := usecase.CreateParameterIntegrationChannel(ctx, ossParameterChannelCmd("primary-r2", "cloudflare_r2", "oss.cloudflare_r2.s3_compatible", true))
	if err != nil {
		t.Fatalf("create first primary OSS channel: %v", err)
	}
	if !first.IsPrimary {
		t.Fatalf("expected first channel to be primary, got %#v", first)
	}

	second, err := usecase.CreateParameterIntegrationChannel(ctx, ossParameterChannelCmd("primary-aliyun", "aliyun", "oss.aliyun_oss.s3_compatible", true))
	if err != nil {
		t.Fatalf("create second primary OSS channel: %v", err)
	}
	if !second.IsPrimary {
		t.Fatalf("expected second channel to be primary, got %#v", second)
	}

	channels, err := usecase.ListParameterIntegrationChannels(ctx, usecase.ListParameterIntegrationChannelsQry{
		Scenario: models.IntegrationScenarioOSS,
	})
	if err != nil {
		t.Fatalf("list OSS channels: %v", err)
	}
	if countPrimaryChannels(channels) != 1 || primaryChannel(channels).ID != second.ID {
		t.Fatalf("expected only second channel to be primary, got %#v", channels)
	}

	updatedFirst, err := usecase.UpdateParameterIntegrationChannel(ctx, ossParameterChannelUpdateCmd(first.ID, "primary-r2", "cloudflare_r2", "oss.cloudflare_r2.s3_compatible", true))
	if err != nil {
		t.Fatalf("promote first channel: %v", err)
	}
	if !updatedFirst.IsPrimary {
		t.Fatalf("expected first channel to become primary, got %#v", updatedFirst)
	}
	channels, err = usecase.ListParameterIntegrationChannels(ctx, usecase.ListParameterIntegrationChannelsQry{
		Scenario: models.IntegrationScenarioOSS,
	})
	if err != nil {
		t.Fatalf("list OSS channels after promote: %v", err)
	}
	if countPrimaryChannels(channels) != 1 || primaryChannel(channels).ID != first.ID {
		t.Fatalf("expected only first channel to be primary, got %#v", channels)
	}

	noPrimary, err := usecase.UpdateParameterIntegrationChannel(ctx, ossParameterChannelUpdateCmd(first.ID, "primary-r2", "cloudflare_r2", "oss.cloudflare_r2.s3_compatible", false))
	if err != nil {
		t.Fatalf("unset first channel primary: %v", err)
	}
	if noPrimary.IsPrimary {
		t.Fatalf("expected first channel to stop being primary, got %#v", noPrimary)
	}
	channels, err = usecase.ListParameterIntegrationChannels(ctx, usecase.ListParameterIntegrationChannelsQry{
		Scenario: models.IntegrationScenarioOSS,
	})
	if err != nil {
		t.Fatalf("list OSS channels after unset: %v", err)
	}
	if countPrimaryChannels(channels) != 0 {
		t.Fatalf("expected zero primary channels to be valid, got %#v", channels)
	}

	rePrimary, err := usecase.UpdateParameterIntegrationChannel(ctx, ossParameterChannelUpdateCmd(first.ID, "primary-r2", "cloudflare_r2", "oss.cloudflare_r2.s3_compatible", true))
	if err != nil {
		t.Fatalf("re-promote first channel: %v", err)
	}
	if !rePrimary.IsPrimary {
		t.Fatalf("expected first channel to be primary again, got %#v", rePrimary)
	}
	disabled, err := usecase.SetParameterIntegrationChannelEnabled(ctx, usecase.SetParameterIntegrationChannelEnabledCmd{
		ID:      first.ID,
		Enabled: false,
	})
	if err != nil {
		t.Fatalf("disable primary channel: %v", err)
	}
	if disabled.Enabled || disabled.IsPrimary {
		t.Fatalf("expected disabling primary channel to clear primary, got %#v", disabled)
	}
}

func TestNonModelScenarioIgnoresPrimaryFlag(t *testing.T) {
	setupUsecaseOrderTxDB(t)
	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)

	channel, err := usecase.CreateParameterIntegrationChannel(ctx, usecase.SaveParameterIntegrationChannelCmd{
		Scenario:        models.IntegrationScenarioPayment,
		ChannelCode:     "primary-ignored-payment",
		ProviderCode:    "creem",
		AdapterKey:      "payment.creem.hosted_checkout",
		Environment:     "test",
		Enabled:         true,
		IsPrimary:       true,
		ConfigJSON:      `{"base_url":"https://test-api.creem.io/v1","product_id":"prod_test"}`,
		MetadataJSON:    "{}",
		CredentialType:  "payment_bundle",
		CredentialValue: `{"api_key":"sk_test","webhook_secret":"whsec_test"}`,
	})
	if err != nil {
		t.Fatalf("create payment channel with primary flag: %v", err)
	}
	if channel.IsPrimary {
		t.Fatalf("expected non-model scenario channel primary flag to be ignored, got %#v", channel)
	}
}

func TestSetParameterIntegrationChannelEnabled(t *testing.T) {
	manager := setupUsecaseOrderTxDB(t)
	appDB, err := manager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}
	seedParameterChannel(t, appDB)

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	channel, err := usecase.SetParameterIntegrationChannelEnabled(ctx, usecase.SetParameterIntegrationChannelEnabledCmd{
		ID:      "param-sms-channel",
		Enabled: false,
	})
	if err != nil {
		t.Fatalf("disable channel: %v", err)
	}
	if channel.Enabled {
		t.Fatalf("expected disabled channel, got %#v", channel)
	}
}

func ossParameterChannelCmd(channelCode string, providerCode string, adapterKey string, isPrimary bool) usecase.SaveParameterIntegrationChannelCmd {
	return usecase.SaveParameterIntegrationChannelCmd{
		Scenario:        models.IntegrationScenarioOSS,
		ChannelCode:     channelCode,
		ProviderCode:    providerCode,
		AdapterKey:      adapterKey,
		Environment:     "production",
		Enabled:         true,
		Priority:        10,
		IsPrimary:       isPrimary,
		ConfigJSON:      ossConfigJSON(providerCode),
		MetadataJSON:    "{}",
		CredentialType:  "s3_access_key",
		CredentialValue: `{"access_key_id":"oss-access-key","secret_access_key":"oss-secret-key"}`,
	}
}

func ossParameterChannelUpdateCmd(id string, channelCode string, providerCode string, adapterKey string, isPrimary bool) usecase.SaveParameterIntegrationChannelCmd {
	cmd := ossParameterChannelCmd(channelCode, providerCode, adapterKey, isPrimary)
	cmd.ID = id
	cmd.CredentialValue = ""
	return cmd
}

func ossConfigJSON(providerCode string) string {
	if providerCode == "aliyun" {
		return `{"endpoint_url":"https://oss-cn-hangzhou.aliyuncs.com","bucket":"assets","region":"cn-hangzhou"}`
	}
	return `{"endpoint_url":"https://example-account.r2.cloudflarestorage.com","bucket":"assets","region":"auto"}`
}

func countPrimaryChannels(channels []usecase.ParameterIntegrationChannelCo) int {
	count := 0
	for i := range channels {
		if channels[i].IsPrimary {
			count++
		}
	}
	return count
}

func primaryChannel(channels []usecase.ParameterIntegrationChannelCo) usecase.ParameterIntegrationChannelCo {
	for i := range channels {
		if channels[i].IsPrimary {
			return channels[i]
		}
	}
	return usecase.ParameterIntegrationChannelCo{}
}

func parameterChannelByCode(channels []usecase.ParameterIntegrationChannelCo, channelCode string) usecase.ParameterIntegrationChannelCo {
	for i := range channels {
		if channels[i].ChannelCode == channelCode {
			return channels[i]
		}
	}
	return usecase.ParameterIntegrationChannelCo{}
}

func seedParameterChannel(t *testing.T, appDB *sqlx.Engine) {
	t.Helper()

	if _, err := appDB.ExecP(`
		INSERT INTO integration_credentials (id, credential_type, ciphertext, key_version, masked_value, value_text, enabled)
		VALUES (?, 'api_key', 'sms-secret', '', '', 'sms-secret', 1)
	`, "param-sms-credential"); err != nil {
		t.Fatalf("insert credential: %v", err)
	}
	if _, err := appDB.ExecP(`
		INSERT INTO integration_channels (
			id, scenario, channel_code, provider_code, adapter_key, environment, enabled,
			priority, credential_id, webhook_enabled, config_json, metadata_json
		) VALUES (?, 'sms', 'param-sms', 'aliyun', 'sms.aliyun.adapter', 'test', 1,
			30, ?, 0, '{"base_url":"https://sms.example.com"}', '{"owner":"ops"}')
	`, "param-sms-channel", "param-sms-credential"); err != nil {
		t.Fatalf("insert channel: %v", err)
	}
}

func loadParameterCredentialValue(t *testing.T, appDB *sqlx.Engine, channelID string) string {
	t.Helper()

	var value string
	if err := appDB.GetP(&value, `
		SELECT cred.value_text
		FROM integration_credentials cred
		INNER JOIN integration_channels channel ON channel.credential_id = cred.id
		WHERE channel.id = ?
	`, channelID); err != nil {
		t.Fatalf("load credential value: %v", err)
	}
	return value
}

func schemaFieldByKey(fields []usecase.ParameterIntegrationSchemaFieldCo, key string) (usecase.ParameterIntegrationSchemaFieldCo, bool) {
	for i := range fields {
		if fields[i].Key == key {
			return fields[i], true
		}
	}
	return usecase.ParameterIntegrationSchemaFieldCo{}, false
}
