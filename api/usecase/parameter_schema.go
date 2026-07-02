package usecase

import (
	"encoding/json"
	"net/url"
	"strconv"
	"strings"

	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
	"github.com/tfnick/go-svelte-starter/api/models"
)

const (
	ParameterIntegrationCredentialFormatPlain      = "plain"
	ParameterIntegrationCredentialFormatJSONObject = "json_object"

	ParameterIntegrationSchemaFieldText    = "text"
	ParameterIntegrationSchemaFieldURL     = "url"
	ParameterIntegrationSchemaFieldNumber  = "number"
	ParameterIntegrationSchemaFieldBoolean = "boolean"
	ParameterIntegrationSchemaFieldSecret  = "secret"
)

type ListParameterIntegrationSchemasQry struct {
	Scenario string
}

type ParameterIntegrationAdapterSchemaCo struct {
	Scenario            string
	AdapterKey          string
	Label               string
	Description         string
	ProviderCode        string
	CredentialType      string
	CredentialFormat    string
	ModelDictionaryType string
	AdvancedJSON        bool
	ConfigFields        []ParameterIntegrationSchemaFieldCo
	CredentialFields    []ParameterIntegrationSchemaFieldCo
}

type ParameterIntegrationSchemaFieldCo struct {
	Key            string
	Label          string
	Kind           string
	Required       bool
	Placeholder    string
	HelpText       string
	DefaultValue   string
	DictionaryType string
	Sensitive      bool
	Options        []ParameterIntegrationSchemaOptionCo
}

type ParameterIntegrationSchemaOptionCo struct {
	Value string
	Label string
}

var parameterIntegrationAdapterSchemas = []ParameterIntegrationAdapterSchemaCo{
	{
		Scenario:         models.IntegrationScenarioExternalNotification,
		AdapterKey:       "bot.feishu.webhook",
		Label:            "Feishu Bot Webhook",
		Description:      "Feishu group bot webhook for external event notifications.",
		ProviderCode:     "feishu",
		CredentialType:   "webhook_url",
		CredentialFormat: ParameterIntegrationCredentialFormatJSONObject,
		AdvancedJSON:     true,
		ConfigFields: []ParameterIntegrationSchemaFieldCo{
			{
				Key:          "message_type",
				Label:        "Message Type",
				Kind:         ParameterIntegrationSchemaFieldText,
				Required:     false,
				DefaultValue: "text",
				Options: []ParameterIntegrationSchemaOptionCo{
					{Value: "text", Label: "Text"},
				},
			},
		},
		CredentialFields: []ParameterIntegrationSchemaFieldCo{
			{
				Key:         "webhook_url",
				Label:       "Webhook URL",
				Kind:        ParameterIntegrationSchemaFieldURL,
				Required:    true,
				Sensitive:   true,
				Placeholder: "https://open.feishu.cn/open-apis/bot/v2/hook/...",
			},
			{
				Key:         "signing_secret",
				Label:       "Signing Secret",
				Kind:        ParameterIntegrationSchemaFieldSecret,
				Required:    false,
				Sensitive:   true,
				Placeholder: "Optional Feishu bot signing secret",
			},
		},
	},
	{
		Scenario:         models.IntegrationScenarioExternalNotification,
		AdapterKey:       "bot.discord.webhook",
		Label:            "Discord Webhook",
		Description:      "Discord webhook for external event notifications.",
		ProviderCode:     "discord",
		CredentialType:   "webhook_url",
		CredentialFormat: ParameterIntegrationCredentialFormatJSONObject,
		AdvancedJSON:     true,
		ConfigFields: []ParameterIntegrationSchemaFieldCo{
			{
				Key:          "message_type",
				Label:        "Message Type",
				Kind:         ParameterIntegrationSchemaFieldText,
				Required:     false,
				DefaultValue: "text",
				Options: []ParameterIntegrationSchemaOptionCo{
					{Value: "text", Label: "Text"},
				},
			},
		},
		CredentialFields: []ParameterIntegrationSchemaFieldCo{
			{
				Key:         "webhook_url",
				Label:       "Webhook URL",
				Kind:        ParameterIntegrationSchemaFieldURL,
				Required:    true,
				Sensitive:   true,
				Placeholder: "https://discord.com/api/webhooks/...",
			},
		},
	},
	{
		Scenario:         models.IntegrationScenarioPayment,
		AdapterKey:       "payment.creem.hosted_checkout",
		Label:            "Creem Hosted Checkout",
		Description:      "Hosted checkout channel for Creem payment integration.",
		ProviderCode:     "creem",
		CredentialType:   "payment_bundle",
		CredentialFormat: ParameterIntegrationCredentialFormatJSONObject,
		AdvancedJSON:     true,
		ConfigFields: []ParameterIntegrationSchemaFieldCo{
			{
				Key:          "base_url",
				Label:        "API URL",
				Kind:         ParameterIntegrationSchemaFieldURL,
				Required:     true,
				DefaultValue: "https://test-api.creem.io/v1",
				HelpText:     "Use the Creem test or production API base URL.",
			},
			{
				Key:         "product_id",
				Label:       "Product ID",
				Kind:        ParameterIntegrationSchemaFieldText,
				Required:    true,
				Placeholder: "prod_...",
			},
			{
				Key:         "success_url",
				Label:       "Success URL",
				Kind:        ParameterIntegrationSchemaFieldURL,
				Required:    false,
				Placeholder: "https://example.com/orders/success",
			},
			{
				Key:          "units",
				Label:        "Units",
				Kind:         ParameterIntegrationSchemaFieldNumber,
				Required:     false,
				DefaultValue: "1",
			},
		},
		CredentialFields: []ParameterIntegrationSchemaFieldCo{
			{
				Key:         "api_key",
				Label:       "API Key",
				Kind:        ParameterIntegrationSchemaFieldSecret,
				Required:    true,
				Sensitive:   true,
				Placeholder: "sk_...",
			},
			{
				Key:         "webhook_secret",
				Label:       "Webhook Secret",
				Kind:        ParameterIntegrationSchemaFieldSecret,
				Required:    true,
				Sensitive:   true,
				Placeholder: "whsec_...",
			},
		},
	},
	{
		Scenario:            models.IntegrationScenarioLLM,
		AdapterKey:          "llm.deepseek.openai_compatible",
		Label:               "DeepSeek OpenAI Compatible",
		Description:         "OpenAI-compatible LLM channel for DeepSeek.",
		ProviderCode:        "deepseek",
		CredentialType:      "api_key",
		CredentialFormat:    ParameterIntegrationCredentialFormatPlain,
		ModelDictionaryType: "llm_model_deepseek",
		AdvancedJSON:        true,
		ConfigFields: []ParameterIntegrationSchemaFieldCo{
			{
				Key:          "base_url",
				Label:        "API URL",
				Kind:         ParameterIntegrationSchemaFieldURL,
				Required:     true,
				DefaultValue: "https://api.deepseek.com",
			},
		},
		CredentialFields: []ParameterIntegrationSchemaFieldCo{
			{
				Key:         "api_key",
				Label:       "API Key",
				Kind:        ParameterIntegrationSchemaFieldSecret,
				Required:    true,
				Sensitive:   true,
				Placeholder: "sk_...",
			},
		},
	},
	{
		Scenario:            models.IntegrationScenarioLLM,
		AdapterKey:          "llm.siliconflow.openai_compatible",
		Label:               "SiliconFlow OpenAI Compatible",
		Description:         "OpenAI-compatible LLM channel for SiliconFlow (aggregated models).",
		ProviderCode:        "siliconflow",
		CredentialType:      "api_key",
		CredentialFormat:    ParameterIntegrationCredentialFormatPlain,
		ModelDictionaryType: "llm_model_siliconflow",
		AdvancedJSON:        true,
		ConfigFields: []ParameterIntegrationSchemaFieldCo{
			{
				Key:          "base_url",
				Label:        "API URL",
				Kind:         ParameterIntegrationSchemaFieldURL,
				Required:     true,
				DefaultValue: "https://api.siliconflow.cn/v1",
			},
		},
		CredentialFields: []ParameterIntegrationSchemaFieldCo{
			{
				Key:         "api_key",
				Label:       "API Key",
				Kind:        ParameterIntegrationSchemaFieldSecret,
				Required:    true,
				Sensitive:   true,
				Placeholder: "sk-...",
			},
		},
	},
	{
		Scenario:            models.IntegrationScenarioEmbedding,
		AdapterKey:          "embedding.siliconflow.openai_compatible",
		Label:               "SiliconFlow Embedding",
		Description:         "OpenAI-compatible embedding channel for SiliconFlow Knowledge Base indexing.",
		ProviderCode:        "siliconflow",
		CredentialType:      "api_key",
		CredentialFormat:    ParameterIntegrationCredentialFormatPlain,
		ModelDictionaryType: "embedding_model_siliconflow",
		AdvancedJSON:        true,
		ConfigFields: []ParameterIntegrationSchemaFieldCo{
			{
				Key:          "base_url",
				Label:        "API URL",
				Kind:         ParameterIntegrationSchemaFieldURL,
				Required:     true,
				DefaultValue: "https://api.siliconflow.cn",
			},
			{
				Key:          "dimensions",
				Label:        "Dimensions",
				Kind:         ParameterIntegrationSchemaFieldNumber,
				Required:     false,
				DefaultValue: "64",
				HelpText:     "The current Knowledge Base vector table is 64-dimensional.",
			},
			{
				Key:          "encoding_format",
				Label:        "Encoding Format",
				Kind:         ParameterIntegrationSchemaFieldText,
				Required:     false,
				DefaultValue: "float",
				Options: []ParameterIntegrationSchemaOptionCo{
					{Value: "float", Label: "Float"},
				},
			},
			{
				Key:          "endpoint_path",
				Label:        "Endpoint Path",
				Kind:         ParameterIntegrationSchemaFieldText,
				Required:     false,
				DefaultValue: "/v1/embeddings",
			},
		},
		CredentialFields: []ParameterIntegrationSchemaFieldCo{
			{
				Key:         "api_key",
				Label:       "API Key",
				Kind:        ParameterIntegrationSchemaFieldSecret,
				Required:    true,
				Sensitive:   true,
				Placeholder: "sk_...",
			},
		},
	},
	{
		Scenario:         models.IntegrationScenarioEmbedding,
		AdapterKey:       "embedding.local_hash_64",
		Label:            "Local Hash 64 Embedding",
		Description:      "Local deterministic 64-dimensional embedding provider for development and fallback Knowledge Base indexing.",
		ProviderCode:     "local",
		CredentialType:   "none",
		CredentialFormat: ParameterIntegrationCredentialFormatPlain,
		AdvancedJSON:     true,
		ConfigFields:     []ParameterIntegrationSchemaFieldCo{},
		CredentialFields: []ParameterIntegrationSchemaFieldCo{},
	},
	{
		Scenario:         models.IntegrationScenarioSMS,
		AdapterKey:       "sms.aliyun.adapter",
		Label:            "Aliyun SMS",
		Description:      "Reserved SMS channel parameter schema; provider adapter implementation is out of scope.",
		ProviderCode:     "aliyun",
		CredentialType:   "api_key",
		CredentialFormat: ParameterIntegrationCredentialFormatPlain,
		AdvancedJSON:     true,
		ConfigFields: []ParameterIntegrationSchemaFieldCo{
			{
				Key:         "base_url",
				Label:       "API URL",
				Kind:        ParameterIntegrationSchemaFieldURL,
				Required:    false,
				Placeholder: "https://dysmsapi.aliyuncs.com",
			},
			{
				Key:         "sign_name",
				Label:       "Sign Name",
				Kind:        ParameterIntegrationSchemaFieldText,
				Required:    false,
				Placeholder: "Acme",
			},
			{
				Key:         "template_code",
				Label:       "Template Code",
				Kind:        ParameterIntegrationSchemaFieldText,
				Required:    false,
				Placeholder: "SMS_123456789",
			},
		},
		CredentialFields: []ParameterIntegrationSchemaFieldCo{
			{
				Key:         "api_key",
				Label:       "API Key",
				Kind:        ParameterIntegrationSchemaFieldSecret,
				Required:    true,
				Sensitive:   true,
				Placeholder: "sms credential",
			},
		},
	},
	{
		Scenario:         models.IntegrationScenarioEmail,
		AdapterKey:       "email.aliyun.smtp",
		Label:            "Aliyun SMTP",
		Description:      "SMTP channel for Aliyun enterprise mailbox.",
		ProviderCode:     "aliyun",
		CredentialType:   "smtp_password",
		CredentialFormat: ParameterIntegrationCredentialFormatJSONObject,
		AdvancedJSON:     true,
		ConfigFields: []ParameterIntegrationSchemaFieldCo{
			{
				Key:          "smtp_host",
				Label:        "SMTP Host",
				Kind:         ParameterIntegrationSchemaFieldText,
				Required:     true,
				DefaultValue: "smtp.qiye.aliyun.com",
				HelpText:     "Aliyun enterprise mailbox SMTP server.",
			},
			{
				Key:          "smtp_port",
				Label:        "SMTP Port",
				Kind:         ParameterIntegrationSchemaFieldNumber,
				Required:     true,
				DefaultValue: "465",
				HelpText:     "Port 465 is recommended for SSL SMTP.",
			},
			{
				Key:          "security",
				Label:        "Security",
				Kind:         ParameterIntegrationSchemaFieldText,
				Required:     true,
				DefaultValue: "ssl",
				Options: []ParameterIntegrationSchemaOptionCo{
					{Value: "ssl", Label: "SSL"},
					{Value: "starttls", Label: "STARTTLS"},
					{Value: "none", Label: "None"},
				},
			},
			{
				Key:         "from_email",
				Label:       "From Email",
				Kind:        ParameterIntegrationSchemaFieldText,
				Required:    true,
				Placeholder: "noreply@example.com",
			},
			{
				Key:         "from_name",
				Label:       "From Name",
				Kind:        ParameterIntegrationSchemaFieldText,
				Required:    false,
				Placeholder: "Acme",
			},
		},
		CredentialFields: []ParameterIntegrationSchemaFieldCo{
			{
				Key:         "username",
				Label:       "SMTP Username",
				Kind:        ParameterIntegrationSchemaFieldText,
				Required:    true,
				Placeholder: "noreply@example.com",
			},
			{
				Key:         "password",
				Label:       "SMTP Password",
				Kind:        ParameterIntegrationSchemaFieldSecret,
				Required:    true,
				Sensitive:   true,
				Placeholder: "client authorization password",
				HelpText:    "Use the mailbox client authorization password, not the account login password.",
			},
		},
	},
	{
		Scenario:         models.IntegrationScenarioEmail,
		AdapterKey:       "email.resend.api",
		Label:            "Resend API",
		Description:      "API key channel for Resend email integration.",
		ProviderCode:     "resend",
		CredentialType:   "api_key",
		CredentialFormat: ParameterIntegrationCredentialFormatPlain,
		AdvancedJSON:     true,
		ConfigFields: []ParameterIntegrationSchemaFieldCo{
			{
				Key:          "base_url",
				Label:        "API URL",
				Kind:         ParameterIntegrationSchemaFieldURL,
				Required:     true,
				DefaultValue: "https://api.resend.com",
			},
			{
				Key:         "from_email",
				Label:       "From Email",
				Kind:        ParameterIntegrationSchemaFieldText,
				Required:    true,
				Placeholder: "noreply@example.com",
			},
			{
				Key:         "from_name",
				Label:       "From Name",
				Kind:        ParameterIntegrationSchemaFieldText,
				Required:    false,
				Placeholder: "Acme",
			},
		},
		CredentialFields: []ParameterIntegrationSchemaFieldCo{
			{
				Key:         "api_key",
				Label:       "API Key",
				Kind:        ParameterIntegrationSchemaFieldSecret,
				Required:    true,
				Sensitive:   true,
				Placeholder: "re_...",
			},
		},
	},
	{
		Scenario:         models.IntegrationScenarioOSS,
		AdapterKey:       "oss.cloudflare_r2.s3_compatible",
		Label:            "Cloudflare R2",
		Description:      "S3-compatible object storage channel for Cloudflare R2.",
		ProviderCode:     "cloudflare_r2",
		CredentialType:   "s3_access_key",
		CredentialFormat: ParameterIntegrationCredentialFormatJSONObject,
		AdvancedJSON:     true,
		ConfigFields: []ParameterIntegrationSchemaFieldCo{
			{
				Key:         "endpoint_url",
				Label:       "Endpoint URL",
				Kind:        ParameterIntegrationSchemaFieldURL,
				Required:    true,
				Placeholder: "https://<account_id>.r2.cloudflarestorage.com",
				HelpText:    "Use the S3-compatible R2 endpoint for the account.",
			},
			{
				Key:         "bucket",
				Label:       "Bucket",
				Kind:        ParameterIntegrationSchemaFieldText,
				Required:    true,
				Placeholder: "my-bucket",
			},
			{
				Key:          "region",
				Label:        "Region",
				Kind:         ParameterIntegrationSchemaFieldText,
				Required:     false,
				DefaultValue: "auto",
				HelpText:     "Cloudflare R2 commonly uses the SDK region value auto.",
			},
			{
				Key:          "use_path_style",
				Label:        "Use Path Style",
				Kind:         ParameterIntegrationSchemaFieldBoolean,
				Required:     false,
				DefaultValue: "true",
				HelpText:     "Use AWS SDK S3 path-style addressing for Cloudflare R2.",
			},
			{
				Key:         "public_base_url",
				Label:       "Public Base URL",
				Kind:        ParameterIntegrationSchemaFieldURL,
				Required:    false,
				Placeholder: "https://assets.example.com",
				HelpText:    "Optional public bucket or custom domain base URL for future object URL composition.",
			},
			{
				Key:         "key_prefix",
				Label:       "Key Prefix",
				Kind:        ParameterIntegrationSchemaFieldText,
				Required:    false,
				Placeholder: "uploads/",
			},
		},
		CredentialFields: []ParameterIntegrationSchemaFieldCo{
			{
				Key:         "access_key_id",
				Label:       "Access Key ID",
				Kind:        ParameterIntegrationSchemaFieldSecret,
				Required:    true,
				Sensitive:   true,
				Placeholder: "R2 access key ID",
			},
			{
				Key:         "secret_access_key",
				Label:       "Secret Access Key",
				Kind:        ParameterIntegrationSchemaFieldSecret,
				Required:    true,
				Sensitive:   true,
				Placeholder: "R2 secret access key",
			},
		},
	},
	{
		Scenario:         models.IntegrationScenarioOSS,
		AdapterKey:       "oss.aliyun_oss.s3_compatible",
		Label:            "Aliyun OSS",
		Description:      "S3-compatible object storage channel for Aliyun OSS.",
		ProviderCode:     "aliyun",
		CredentialType:   "s3_access_key",
		CredentialFormat: ParameterIntegrationCredentialFormatJSONObject,
		AdvancedJSON:     true,
		ConfigFields: []ParameterIntegrationSchemaFieldCo{
			{
				Key:         "endpoint_url",
				Label:       "Endpoint URL",
				Kind:        ParameterIntegrationSchemaFieldURL,
				Required:    true,
				Placeholder: "https://oss-cn-hangzhou.aliyuncs.com",
				HelpText:    "Use the Aliyun OSS S3-compatible endpoint for the bucket region.",
			},
			{
				Key:         "bucket",
				Label:       "Bucket",
				Kind:        ParameterIntegrationSchemaFieldText,
				Required:    true,
				Placeholder: "my-bucket",
			},
			{
				Key:         "region",
				Label:       "Region",
				Kind:        ParameterIntegrationSchemaFieldText,
				Required:    false,
				Placeholder: "cn-hangzhou",
				HelpText:    "Optional SDK region value matching the OSS endpoint region.",
			},
			{
				Key:      "use_path_style",
				Label:    "Use Path Style",
				Kind:     ParameterIntegrationSchemaFieldBoolean,
				Required: false,
				HelpText: "Optional AWS SDK S3 path-style addressing override. Aliyun OSS defaults to virtual-hosted addressing.",
			},
			{
				Key:         "public_base_url",
				Label:       "Public Base URL",
				Kind:        ParameterIntegrationSchemaFieldURL,
				Required:    false,
				Placeholder: "https://assets.example.com",
				HelpText:    "Optional public bucket or custom domain base URL for future object URL composition.",
			},
			{
				Key:         "key_prefix",
				Label:       "Key Prefix",
				Kind:        ParameterIntegrationSchemaFieldText,
				Required:    false,
				Placeholder: "uploads/",
			},
		},
		CredentialFields: []ParameterIntegrationSchemaFieldCo{
			{
				Key:         "access_key_id",
				Label:       "Access Key ID",
				Kind:        ParameterIntegrationSchemaFieldSecret,
				Required:    true,
				Sensitive:   true,
				Placeholder: "Aliyun AccessKey ID",
			},
			{
				Key:         "secret_access_key",
				Label:       "Secret Access Key",
				Kind:        ParameterIntegrationSchemaFieldSecret,
				Required:    true,
				Sensitive:   true,
				Placeholder: "Aliyun AccessKey secret",
			},
		},
	},
}

func ListParameterIntegrationSchemas(_ fwusecase.Context, qry ListParameterIntegrationSchemasQry) ([]ParameterIntegrationAdapterSchemaCo, error) {
	scenario := strings.TrimSpace(qry.Scenario)
	if scenario != "" {
		normalized, err := normalizeIntegrationScenario(scenario)
		if err != nil {
			return nil, err
		}
		scenario = normalized
	}

	result := make([]ParameterIntegrationAdapterSchemaCo, 0, len(parameterIntegrationAdapterSchemas))
	for i := range parameterIntegrationAdapterSchemas {
		schema := parameterIntegrationAdapterSchemas[i]
		if scenario == "" || schema.Scenario == scenario {
			result = append(result, schema)
		}
	}
	return result, nil
}

func parameterIntegrationSchemaByAdapterKey(adapterKey string) (ParameterIntegrationAdapterSchemaCo, bool) {
	normalized := strings.TrimSpace(adapterKey)
	for i := range parameterIntegrationAdapterSchemas {
		if parameterIntegrationAdapterSchemas[i].AdapterKey == normalized {
			return parameterIntegrationAdapterSchemas[i], true
		}
	}
	return ParameterIntegrationAdapterSchemaCo{}, false
}

func parameterIntegrationSchemaRequiresCredential(adapterKey string) bool {
	schema, ok := parameterIntegrationSchemaByAdapterKey(adapterKey)
	if !ok {
		return true
	}
	for _, field := range schema.CredentialFields {
		if field.Required {
			return true
		}
	}
	return false
}

func validateParameterIntegrationSchema(input parameterIntegrationChannelInputData, create bool) error {
	schema, ok := parameterIntegrationSchemaByAdapterKey(input.AdapterKey)
	if !ok {
		return nil
	}
	if input.Scenario != schema.Scenario {
		return fwusecase.E(fwusecase.CodeValidation, "adapter schema does not match integration scenario", nil)
	}
	if schema.ProviderCode != "" && input.ProviderCode != schema.ProviderCode {
		return fwusecase.E(fwusecase.CodeValidation, "provider code does not match adapter schema", nil)
	}
	if schema.CredentialType != "" && input.CredentialType != schema.CredentialType {
		return fwusecase.E(fwusecase.CodeValidation, "credential type does not match adapter schema", nil)
	}

	config, err := decodeJSONObject(input.ConfigJSON, "config JSON")
	if err != nil {
		return err
	}
	if err := validateParameterSchemaObjectFields(config, schema.ConfigFields, "config"); err != nil {
		return err
	}

	if create || strings.TrimSpace(input.CredentialValue) != "" {
		if err := validateParameterSchemaCredential(input.CredentialValue, schema); err != nil {
			return err
		}
	}
	return nil
}

func validateParameterSchemaCredential(value string, schema ParameterIntegrationAdapterSchemaCo) error {
	raw := strings.TrimSpace(value)
	if raw == "" {
		if len(schema.CredentialFields) == 0 {
			return nil
		}
		return fwusecase.E(fwusecase.CodeValidation, "credential value is required", nil)
	}

	switch schema.CredentialFormat {
	case ParameterIntegrationCredentialFormatPlain:
		for _, field := range schema.CredentialFields {
			if field.Required && raw == "" {
				return fwusecase.E(fwusecase.CodeValidation, field.Label+" is required", nil)
			}
		}
		return nil
	case ParameterIntegrationCredentialFormatJSONObject:
		credential, err := decodeJSONObject(raw, "credential value")
		if err != nil {
			return err
		}
		return validateParameterSchemaObjectFields(credential, schema.CredentialFields, "credential")
	default:
		return fwusecase.E(fwusecase.CodeValidation, "invalid credential schema format", nil)
	}
}

func validateParameterSchemaObjectFields(value map[string]interface{}, fields []ParameterIntegrationSchemaFieldCo, scope string) error {
	for _, field := range fields {
		item, exists := value[field.Key]
		empty := !exists || parameterSchemaValueIsEmpty(item)
		if field.Required && empty {
			return fwusecase.E(fwusecase.CodeValidation, field.Label+" is required", nil)
		}
		if empty {
			continue
		}
		if err := validateParameterSchemaFieldValue(item, field, scope); err != nil {
			return err
		}
	}
	return nil
}

func validateParameterSchemaFieldValue(value interface{}, field ParameterIntegrationSchemaFieldCo, scope string) error {
	if len(field.Options) > 0 {
		raw, ok := parameterSchemaValueAsString(value)
		if !ok || !parameterSchemaOptionAllows(raw, field.Options) {
			return fwusecase.E(fwusecase.CodeValidation, field.Label+" must be one of the allowed options", nil)
		}
	}

	switch field.Kind {
	case ParameterIntegrationSchemaFieldURL:
		raw, ok := parameterSchemaValueAsString(value)
		if !ok {
			return fwusecase.E(fwusecase.CodeValidation, field.Label+" must be a URL", nil)
		}
		parsed, err := url.Parse(raw)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return fwusecase.E(fwusecase.CodeValidation, field.Label+" must be a URL", err)
		}
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			return fwusecase.E(fwusecase.CodeValidation, field.Label+" must be an HTTP URL", nil)
		}
	case ParameterIntegrationSchemaFieldNumber:
		if _, ok := value.(float64); ok {
			return nil
		}
		raw, ok := parameterSchemaValueAsString(value)
		if !ok {
			return fwusecase.E(fwusecase.CodeValidation, field.Label+" must be a number", nil)
		}
		if _, err := strconv.ParseFloat(raw, 64); err != nil {
			return fwusecase.E(fwusecase.CodeValidation, field.Label+" must be a number", err)
		}
	case ParameterIntegrationSchemaFieldBoolean:
		if _, ok := value.(bool); !ok {
			return fwusecase.E(fwusecase.CodeValidation, field.Label+" must be a boolean", nil)
		}
	case ParameterIntegrationSchemaFieldText, ParameterIntegrationSchemaFieldSecret:
		if _, ok := parameterSchemaValueAsString(value); !ok {
			return fwusecase.E(fwusecase.CodeValidation, field.Label+" must be text", nil)
		}
	default:
		return fwusecase.E(fwusecase.CodeValidation, scope+" field schema is invalid", nil)
	}
	return nil
}

func parameterSchemaOptionAllows(value string, options []ParameterIntegrationSchemaOptionCo) bool {
	for _, option := range options {
		if option.Value == value {
			return true
		}
	}
	return false
}

func parameterSchemaValueIsEmpty(value interface{}) bool {
	if value == nil {
		return true
	}
	if raw, ok := value.(string); ok {
		return strings.TrimSpace(raw) == ""
	}
	return false
}

func parameterSchemaValueAsString(value interface{}) (string, bool) {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed), true
	case json.Number:
		return strings.TrimSpace(typed.String()), true
	default:
		return "", false
	}
}
