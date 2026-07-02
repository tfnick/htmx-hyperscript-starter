package models

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/tfnick/go-svelte-starter/api/db"
	"github.com/tfnick/go-svelte-starter/api/framework/data/modelerror"
	"github.com/tfnick/go-svelte-starter/api/framework/timefmt"
)

const (
	IntegrationScenarioLLM                  = "llm"
	IntegrationScenarioPayment              = "payment"
	IntegrationScenarioSMS                  = "sms"
	IntegrationScenarioEmail                = "email"
	IntegrationScenarioOSS                  = "oss"
	IntegrationScenarioEmbedding            = "embedding"
	IntegrationScenarioExternalNotification = "external_notification"

	deepSeekOpenAICompatibleAdapterKey             = "llm.deepseek.openai_compatible"
	siliconFlowEmbeddingOpenAICompatibleAdapterKey = "embedding.siliconflow.openai_compatible"
	localHashEmbeddingAdapterKey                   = "embedding.local_hash_64"
	defaultDeepSeekModelCode                       = "deepseek-chat"
	defaultSiliconFlowEmbeddingModelCode           = "qwen3-embedding-0.6b"
	defaultSiliconFlowEmbeddingProviderModelID     = "Qwen/Qwen3-Embedding-0.6B"
	defaultLocalHashEmbeddingModelCode             = "local-hash-64"

	IntegrationInvocationStatusStarted   = "started"
	IntegrationInvocationStatusSucceeded = "succeeded"
	IntegrationInvocationStatusFailed    = "failed"

	IntegrationWebhookReceiptStatusReceived   = "received"
	IntegrationWebhookReceiptStatusQueued     = "queued"
	IntegrationWebhookReceiptStatusProcessing = "processing"
	IntegrationWebhookReceiptStatusSucceeded  = "succeeded"
	IntegrationWebhookReceiptStatusFailed     = "failed"
	IntegrationWebhookReceiptStatusIgnored    = "ignored"
)

var ErrIntegrationChannelConflict = errors.New("integration channel conflict")

type IntegrationCredential struct {
	ID             string `db:"id"`
	CredentialType string `db:"credential_type"`
	ValueText      string `db:"value_text"`
	Enabled        int    `db:"enabled"`
	RotatedAt      string `db:"rotated_at"`
	CreatedAt      string `db:"created_at"`
	UpdatedAt      string `db:"updated_at"`
}

type IntegrationChannel struct {
	ID             string `db:"id"`
	Scenario       string `db:"scenario"`
	ChannelCode    string `db:"channel_code"`
	ProviderCode   string `db:"provider_code"`
	AdapterKey     string `db:"adapter_key"`
	Environment    string `db:"environment"`
	Enabled        int    `db:"enabled"`
	Priority       int    `db:"priority"`
	CredentialID   string `db:"credential_id"`
	PolicyID       string `db:"policy_id"`
	WebhookEnabled int    `db:"webhook_enabled"`
	IsPrimary      int    `db:"is_primary"`
	ConfigJSON     string `db:"config_json"`
	MetadataJSON   string `db:"metadata_json"`
	CreatedAt      string `db:"created_at"`
	UpdatedAt      string `db:"updated_at"`
}

type IntegrationPolicy struct {
	ID            string `db:"id"`
	Scenario      string `db:"scenario"`
	Name          string `db:"name"`
	AllowlistJSON string `db:"allowlist_json"`
	DenylistJSON  string `db:"denylist_json"`
	RateLimitJSON string `db:"rate_limit_json"`
	RiskRulesJSON string `db:"risk_rules_json"`
	Enabled       int    `db:"enabled"`
	CreatedAt     string `db:"created_at"`
	UpdatedAt     string `db:"updated_at"`
}

type IntegrationModelOption struct {
	ID                string `db:"id"`
	Scenario          string `db:"scenario"`
	ChannelID         string `db:"channel_id"`
	ModelCode         string `db:"model_code"`
	ProviderModelID   string `db:"provider_model_id"`
	CapabilitiesJSON  string `db:"capabilities_json"`
	DefaultParamsJSON string `db:"default_params_json"`
	CostPolicyJSON    string `db:"cost_policy_json"`
	Enabled           int    `db:"enabled"`
	CreatedAt         string `db:"created_at"`
	UpdatedAt         string `db:"updated_at"`
}

type IntegrationOperationConfig struct {
	ID          string `db:"id"`
	Scenario    string `db:"scenario"`
	Operation   string `db:"operation"`
	ChannelCode string `db:"channel_code"`
	ModelCode   string `db:"model_code"`
	Enabled     int    `db:"enabled"`
	ConfigJSON  string `db:"config_json"`
	CreatedAt   string `db:"created_at"`
	UpdatedAt   string `db:"updated_at"`
}

type IntegrationInvocation struct {
	ID                string `db:"id"`
	Scenario          string `db:"scenario"`
	ChannelID         string `db:"channel_id"`
	ChannelCode       string `db:"channel_code"`
	ProviderCode      string `db:"provider_code"`
	Operation         string `db:"operation"`
	IdempotencyKey    string `db:"idempotency_key"`
	ModelCode         string `db:"model_code"`
	ProviderRequestID string `db:"provider_request_id"`
	Status            string `db:"status"`
	ErrorCategory     string `db:"error_category"`
	Retryable         int    `db:"retryable"`
	UsageJSON         string `db:"usage_json"`
	DurationMS        int64  `db:"duration_ms"`
	CreatedAt         string `db:"created_at"`
	UpdatedAt         string `db:"updated_at"`
}

type IntegrationWebhookReceipt struct {
	ID                string `db:"id"`
	Scenario          string `db:"scenario"`
	ChannelID         string `db:"channel_id"`
	ChannelCode       string `db:"channel_code"`
	ProviderCode      string `db:"provider_code"`
	ProviderEventID   string `db:"provider_event_id"`
	IdempotencyKey    string `db:"idempotency_key"`
	PayloadHash       string `db:"payload_hash"`
	PayloadCiphertext string `db:"payload_ciphertext"`
	SafeSnapshotJSON  string `db:"safe_snapshot_json"`
	HeadersHash       string `db:"headers_hash"`
	Status            string `db:"status"`
	Attempts          int    `db:"attempts"`
	LastErrorCode     string `db:"last_error_code"`
	ReceivedAt        string `db:"received_at"`
	ProcessedAt       string `db:"processed_at"`
	MessageID         string `db:"message_id"`
	CreatedAt         string `db:"created_at"`
	UpdatedAt         string `db:"updated_at"`
}

type IntegrationLLMConfig struct {
	Channel         IntegrationChannel
	Credential      IntegrationCredential
	Policy          IntegrationPolicy
	Model           IntegrationModelOption
	OperationConfig IntegrationOperationConfig
}

type IntegrationPaymentConfig struct {
	Channel         IntegrationChannel
	Credential      IntegrationCredential
	Policy          IntegrationPolicy
	OperationConfig IntegrationOperationConfig
}

type IntegrationChannelConfig struct {
	ID              string `db:"id"`
	Scenario        string `db:"scenario"`
	ChannelCode     string `db:"channel_code"`
	ProviderCode    string `db:"provider_code"`
	AdapterKey      string `db:"adapter_key"`
	Environment     string `db:"environment"`
	Enabled         int    `db:"enabled"`
	Priority        int    `db:"priority"`
	CredentialID    string `db:"credential_id"`
	CredentialType  string `db:"credential_type"`
	CredentialValue string `db:"credential_value"`
	PolicyID        string `db:"policy_id"`
	WebhookEnabled  int    `db:"webhook_enabled"`
	IsPrimary       int    `db:"is_primary"`
	ConfigJSON      string `db:"config_json"`
	MetadataJSON    string `db:"metadata_json"`
	ModelCode       string `db:"model_code"`
	ProviderModelID string `db:"provider_model_id"`
	CreatedAt       string `db:"created_at"`
	UpdatedAt       string `db:"updated_at"`
}

type LLMConfigQuery struct {
	Scenario    string
	Operation   string
	ChannelCode string
	ModelCode   string
}

type PaymentConfigQuery struct {
	Scenario    string
	Operation   string
	ChannelCode string
}

type IntegrationEmbeddingConfig struct {
	Channel         IntegrationChannel
	Credential      IntegrationCredential
	Policy          IntegrationPolicy
	Model           IntegrationModelOption
	OperationConfig IntegrationOperationConfig
}

type EmbeddingConfigQuery struct {
	Scenario    string
	Operation   string
	ChannelCode string
	ModelCode   string
}

func GetEnabledEmbeddingConfig(ctx context.Context, qry EmbeddingConfigQuery) (IntegrationEmbeddingConfig, error) {
	operationConfig, err := findEnabledOperationConfig(ctx, qry.Scenario, qry.Operation)
	if err != nil {
		return IntegrationEmbeddingConfig{}, err
	}

	channelCode := qry.ChannelCode
	if channelCode == "" {
		channelCode = operationConfig.ChannelCode
	}
	modelCode := qry.ModelCode
	if modelCode == "" {
		modelCode = operationConfig.ModelCode
	}

	channel, err := findEnabledChannel(ctx, qry.Scenario, channelCode)
	if err != nil {
		return IntegrationEmbeddingConfig{}, err
	}

	credential, err := getEnabledCredentialByID(ctx, channel.CredentialID)
	if err != nil {
		return IntegrationEmbeddingConfig{}, err
	}

	policy := IntegrationPolicy{}
	if channel.PolicyID != "" {
		policy, err = getEnabledPolicyByID(ctx, channel.PolicyID)
		if err != nil {
			return IntegrationEmbeddingConfig{}, err
		}
	}

	model, err := findEnabledModelOption(ctx, qry.Scenario, channel.ID, modelCode)
	if err != nil {
		fallback, ok := defaultEmbeddingModelOptionForChannel(qry.Scenario, channel, modelCode, err)
		if !ok {
			return IntegrationEmbeddingConfig{}, err
		}
		model = fallback
	}

	return IntegrationEmbeddingConfig{
		Channel:         channel,
		Credential:      credential,
		Policy:          policy,
		Model:           model,
		OperationConfig: operationConfig,
	}, nil
}

func GetEnabledPrimaryOSSChannelConfig(ctx context.Context) (IntegrationChannelConfig, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return IntegrationChannelConfig{}, fmt.Errorf("database unavailable: %w", err)
	}

	query := integrationChannelConfigSelectSQL() + `
		WHERE c.scenario = ? AND c.enabled = 1 AND c.is_primary = 1 AND cred.enabled = 1
		LIMIT 1
	`
	var channel IntegrationChannelConfig
	if err := d.GetP(&channel, query, IntegrationScenarioOSS); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return IntegrationChannelConfig{}, fmt.Errorf("primary OSS channel not found: %w", modelerror.ErrNotFound)
		}
		return IntegrationChannelConfig{}, fmt.Errorf("load primary OSS channel failed: %w", err)
	}
	return channel, nil
}

func GetOSSChannelConfigByCodeAndAdapter(ctx context.Context, channelCode string, adapterKey string) (IntegrationChannelConfig, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return IntegrationChannelConfig{}, fmt.Errorf("database unavailable: %w", err)
	}

	query := integrationChannelConfigSelectSQL() + `
		WHERE c.scenario = ? AND c.channel_code = ? AND c.adapter_key = ? AND cred.enabled = 1
		ORDER BY c.created_at DESC, c.id DESC
		LIMIT 1
	`
	var channel IntegrationChannelConfig
	if err := d.GetP(&channel, query, IntegrationScenarioOSS, channelCode, adapterKey); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return IntegrationChannelConfig{}, fmt.Errorf("OSS channel not found: %w", modelerror.ErrNotFound)
		}
		return IntegrationChannelConfig{}, fmt.Errorf("load OSS channel failed: %w", err)
	}
	return channel, nil
}

func GetEnabledLLMConfig(ctx context.Context, qry LLMConfigQuery) (IntegrationLLMConfig, error) {
	operationConfig, err := findEnabledOperationConfig(ctx, qry.Scenario, qry.Operation)
	if err != nil {
		return IntegrationLLMConfig{}, err
	}

	channelCode := qry.ChannelCode
	if channelCode == "" {
		channelCode = operationConfig.ChannelCode
	}
	modelCode := qry.ModelCode
	if modelCode == "" {
		modelCode = operationConfig.ModelCode
	}

	channel, err := findEnabledChannel(ctx, qry.Scenario, channelCode)
	if err != nil {
		return IntegrationLLMConfig{}, err
	}

	credential, err := getEnabledCredentialByID(ctx, channel.CredentialID)
	if err != nil {
		return IntegrationLLMConfig{}, err
	}

	policy := IntegrationPolicy{}
	if channel.PolicyID != "" {
		policy, err = getEnabledPolicyByID(ctx, channel.PolicyID)
		if err != nil {
			return IntegrationLLMConfig{}, err
		}
	}

	model, err := findEnabledModelOption(ctx, qry.Scenario, channel.ID, modelCode)
	if err != nil {
		fallback, ok := defaultLLMModelOptionForChannel(qry.Scenario, channel, modelCode, err)
		if !ok {
			return IntegrationLLMConfig{}, err
		}
		model = fallback
	}

	return IntegrationLLMConfig{
		Channel:         channel,
		Credential:      credential,
		Policy:          policy,
		Model:           model,
		OperationConfig: operationConfig,
	}, nil
}

func GetEnabledPaymentConfig(ctx context.Context, qry PaymentConfigQuery) (IntegrationPaymentConfig, error) {
	operationConfig, err := findEnabledOperationConfig(ctx, qry.Scenario, qry.Operation)
	if err != nil {
		return IntegrationPaymentConfig{}, err
	}

	channelCode := qry.ChannelCode
	if channelCode == "" {
		channelCode = operationConfig.ChannelCode
	}

	channel, err := findEnabledChannel(ctx, qry.Scenario, channelCode)
	if err != nil {
		return IntegrationPaymentConfig{}, err
	}

	credential, err := getEnabledCredentialByID(ctx, channel.CredentialID)
	if err != nil {
		return IntegrationPaymentConfig{}, err
	}

	policy := IntegrationPolicy{}
	if channel.PolicyID != "" {
		policy, err = getEnabledPolicyByID(ctx, channel.PolicyID)
		if err != nil {
			return IntegrationPaymentConfig{}, err
		}
	}

	return IntegrationPaymentConfig{
		Channel:         channel,
		Credential:      credential,
		Policy:          policy,
		OperationConfig: operationConfig,
	}, nil
}

func findEnabledOperationConfig(ctx context.Context, scenario string, operation string) (IntegrationOperationConfig, error) {
	if operation == "" {
		return IntegrationOperationConfig{}, nil
	}

	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return IntegrationOperationConfig{}, fmt.Errorf("database unavailable: %w", err)
	}

	query := `
		SELECT id, scenario, operation, channel_code, model_code, enabled, config_json, created_at, updated_at
		FROM integration_operation_configs
		WHERE scenario = ? AND operation = ? AND enabled = 1
		LIMIT 1
	`
	var config IntegrationOperationConfig
	if err := d.GetP(&config, query, scenario, operation); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return IntegrationOperationConfig{}, nil
		}
		return IntegrationOperationConfig{}, fmt.Errorf("load integration operation config failed: %w", err)
	}
	return config, nil
}

func ListIntegrationChannelConfigs(ctx context.Context, scenario string) ([]IntegrationChannelConfig, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return nil, fmt.Errorf("database unavailable: %w", err)
	}

	query := integrationChannelConfigSelectSQL() + `
		WHERE c.scenario = ?
		ORDER BY c.priority ASC, c.channel_code ASC, c.created_at DESC
	`
	var channels []IntegrationChannelConfig
	if err := d.SelectP(&channels, query, scenario); err != nil {
		return nil, fmt.Errorf("list integration channels failed: %w", err)
	}
	return channels, nil
}

func GetIntegrationChannelConfigByID(ctx context.Context, id string) (IntegrationChannelConfig, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return IntegrationChannelConfig{}, fmt.Errorf("database unavailable: %w", err)
	}

	query := integrationChannelConfigSelectSQL() + `
		WHERE c.id = ?
		LIMIT 1
	`
	var channel IntegrationChannelConfig
	if err := d.GetP(&channel, query, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return IntegrationChannelConfig{}, fmt.Errorf("integration channel not found: %w", modelerror.ErrNotFound)
		}
		return IntegrationChannelConfig{}, fmt.Errorf("load integration channel failed: %w", err)
	}
	return channel, nil
}

type CreateIntegrationCredentialCmd struct {
	CredentialType string
	ValueText      string
	Enabled        bool
}

func CreateIntegrationCredential(ctx context.Context, cmd CreateIntegrationCredentialCmd) (IntegrationCredential, error) {
	now := timefmt.NowSQLiteDateTime()
	credential := IntegrationCredential{
		ID:             uuid.Must(uuid.NewV7()).String(),
		CredentialType: cmd.CredentialType,
		ValueText:      cmd.ValueText,
		Enabled:        boolToInt(cmd.Enabled),
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return IntegrationCredential{}, fmt.Errorf("database unavailable: %w", err)
	}
	if _, err := d.ExecNamed(`
		INSERT INTO integration_credentials (
			id, credential_type, ciphertext, key_version, masked_value, value_text, enabled, created_at, updated_at
		) VALUES (
			:id, :credential_type, :value_text, '', '', :value_text, :enabled, :created_at, :updated_at
		)
	`, credential); err != nil {
		return IntegrationCredential{}, fmt.Errorf("create integration credential failed: %w", err)
	}
	return credential, nil
}

type UpdateIntegrationCredentialCmd struct {
	ID             string
	CredentialType string
	ValueText      string
	Enabled        bool
	UpdateValue    bool
}

func UpdateIntegrationCredential(ctx context.Context, cmd UpdateIntegrationCredentialCmd) (IntegrationCredential, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return IntegrationCredential{}, fmt.Errorf("database unavailable: %w", err)
	}

	if cmd.UpdateValue {
		query := `
			UPDATE integration_credentials SET
				credential_type = ?,
				ciphertext = ?,
				key_version = '',
				masked_value = '',
				value_text = ?,
				enabled = ?,
				rotated_at = ?,
				updated_at = ?
			WHERE id = ?
		`
		now := timefmt.NowSQLiteDateTime()
		result, err := d.ExecP(query, cmd.CredentialType, cmd.ValueText, cmd.ValueText, boolToInt(cmd.Enabled), now, now, cmd.ID)
		if err != nil {
			return IntegrationCredential{}, fmt.Errorf("update integration credential failed: %w", err)
		}
		if err := requireRowsAffected(result, "integration credential not found"); err != nil {
			return IntegrationCredential{}, err
		}
		return getIntegrationCredentialByID(ctx, cmd.ID)
	}

	query := `
		UPDATE integration_credentials SET
			credential_type = ?,
			enabled = ?,
			updated_at = ?
		WHERE id = ?
	`
	result, err := d.ExecP(query, cmd.CredentialType, boolToInt(cmd.Enabled), timefmt.NowSQLiteDateTime(), cmd.ID)
	if err != nil {
		return IntegrationCredential{}, fmt.Errorf("update integration credential failed: %w", err)
	}
	if err := requireRowsAffected(result, "integration credential not found"); err != nil {
		return IntegrationCredential{}, err
	}
	return getIntegrationCredentialByID(ctx, cmd.ID)
}

type CreateIntegrationChannelCmd struct {
	Scenario       string
	ChannelCode    string
	ProviderCode   string
	AdapterKey     string
	Environment    string
	Enabled        bool
	Priority       int
	CredentialID   string
	PolicyID       string
	WebhookEnabled bool
	IsPrimary      bool
	ConfigJSON     string
	MetadataJSON   string
}

func CreateIntegrationChannel(ctx context.Context, cmd CreateIntegrationChannelCmd) (IntegrationChannelConfig, error) {
	now := timefmt.NowSQLiteDateTime()
	channel := IntegrationChannel{
		ID:             uuid.Must(uuid.NewV7()).String(),
		Scenario:       cmd.Scenario,
		ChannelCode:    cmd.ChannelCode,
		ProviderCode:   cmd.ProviderCode,
		AdapterKey:     cmd.AdapterKey,
		Environment:    cmd.Environment,
		Enabled:        boolToInt(cmd.Enabled),
		Priority:       cmd.Priority,
		CredentialID:   cmd.CredentialID,
		PolicyID:       cmd.PolicyID,
		WebhookEnabled: boolToInt(cmd.WebhookEnabled),
		IsPrimary:      boolToInt(cmd.IsPrimary),
		ConfigJSON:     cmd.ConfigJSON,
		MetadataJSON:   cmd.MetadataJSON,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if channel.Environment == "" {
		channel.Environment = "production"
	}
	if channel.ConfigJSON == "" {
		channel.ConfigJSON = "{}"
	}
	if channel.MetadataJSON == "" {
		channel.MetadataJSON = "{}"
	}
	if !integrationScenarioAllowsPrimary(channel.Scenario) || channel.Enabled == 0 {
		channel.IsPrimary = 0
	}

	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return IntegrationChannelConfig{}, fmt.Errorf("database unavailable: %w", err)
	}
	if _, err := d.ExecNamed(`
		INSERT INTO integration_channels (
			id, scenario, channel_code, provider_code, adapter_key, environment, enabled,
			priority, credential_id, policy_id, webhook_enabled, is_primary, config_json, metadata_json,
			created_at, updated_at
		) VALUES (
			:id, :scenario, :channel_code, :provider_code, :adapter_key, :environment, :enabled,
			:priority, :credential_id, NULLIF(:policy_id, ''), :webhook_enabled, :is_primary, :config_json, :metadata_json,
			:created_at, :updated_at
		)
	`, channel); err != nil {
		if isSQLiteUniqueConstraint(err) {
			return IntegrationChannelConfig{}, fmt.Errorf("integration channel already exists: %w", ErrIntegrationChannelConflict)
		}
		return IntegrationChannelConfig{}, fmt.Errorf("create integration channel failed: %w", err)
	}
	return GetIntegrationChannelConfigByID(ctx, channel.ID)
}

type UpdateIntegrationChannelCmd struct {
	ID             string
	Scenario       string
	ChannelCode    string
	ProviderCode   string
	AdapterKey     string
	Environment    string
	Enabled        bool
	Priority       int
	PolicyID       string
	WebhookEnabled bool
	IsPrimary      bool
	ConfigJSON     string
	MetadataJSON   string
}

func UpdateIntegrationChannel(ctx context.Context, cmd UpdateIntegrationChannelCmd) (IntegrationChannelConfig, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return IntegrationChannelConfig{}, fmt.Errorf("database unavailable: %w", err)
	}
	isPrimary := boolToInt(cmd.IsPrimary)
	if !integrationScenarioAllowsPrimary(cmd.Scenario) || !cmd.Enabled {
		isPrimary = 0
	}

	query := `
		UPDATE integration_channels SET
			scenario = ?,
			channel_code = ?,
			provider_code = ?,
			adapter_key = ?,
			environment = ?,
			enabled = ?,
			priority = ?,
			policy_id = NULLIF(?, ''),
			webhook_enabled = ?,
			is_primary = ?,
			config_json = ?,
			metadata_json = ?,
			updated_at = ?
		WHERE id = ?
	`
	result, err := d.ExecP(
		query,
		cmd.Scenario,
		cmd.ChannelCode,
		cmd.ProviderCode,
		cmd.AdapterKey,
		cmd.Environment,
		boolToInt(cmd.Enabled),
		cmd.Priority,
		cmd.PolicyID,
		boolToInt(cmd.WebhookEnabled),
		isPrimary,
		cmd.ConfigJSON,
		cmd.MetadataJSON,
		timefmt.NowSQLiteDateTime(),
		cmd.ID,
	)
	if err != nil {
		if isSQLiteUniqueConstraint(err) {
			return IntegrationChannelConfig{}, fmt.Errorf("integration channel already exists: %w", ErrIntegrationChannelConflict)
		}
		return IntegrationChannelConfig{}, fmt.Errorf("update integration channel failed: %w", err)
	}
	if err := requireRowsAffected(result, "integration channel not found"); err != nil {
		return IntegrationChannelConfig{}, err
	}
	return GetIntegrationChannelConfigByID(ctx, cmd.ID)
}

func SetIntegrationChannelEnabled(ctx context.Context, id string, enabled bool) (IntegrationChannelConfig, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return IntegrationChannelConfig{}, fmt.Errorf("database unavailable: %w", err)
	}
	query := `
		UPDATE integration_channels
		SET enabled = ?,
			is_primary = CASE WHEN ? = 0 THEN 0 ELSE is_primary END,
			updated_at = ?
		WHERE id = ?
	`
	enabledInt := boolToInt(enabled)
	result, err := d.ExecP(query, enabledInt, enabledInt, timefmt.NowSQLiteDateTime(), id)
	if err != nil {
		return IntegrationChannelConfig{}, fmt.Errorf("set integration channel enabled failed: %w", err)
	}
	if err := requireRowsAffected(result, "integration channel not found"); err != nil {
		return IntegrationChannelConfig{}, err
	}
	return GetIntegrationChannelConfigByID(ctx, id)
}

func ClearIntegrationChannelPrimary(ctx context.Context, scenario string) error {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return fmt.Errorf("database unavailable: %w", err)
	}
	query := `
		UPDATE integration_channels
		SET is_primary = 0, updated_at = ?
		WHERE scenario = ? AND is_primary = 1
	`
	if _, err := d.ExecP(query, timefmt.NowSQLiteDateTime(), scenario); err != nil {
		return fmt.Errorf("clear integration channel primary failed: %w", err)
	}
	return nil
}

func GetEnabledPrimaryIntegrationChannelConfig(ctx context.Context, scenario string) (IntegrationChannelConfig, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return IntegrationChannelConfig{}, fmt.Errorf("database unavailable: %w", err)
	}

	query := integrationChannelConfigSelectSQL() + `
		WHERE c.scenario = ? AND c.enabled = 1 AND c.is_primary = 1 AND cred.enabled = 1
		ORDER BY c.priority ASC, c.created_at DESC, c.id DESC
		LIMIT 1
	`
	var channel IntegrationChannelConfig
	if err := d.GetP(&channel, query, scenario); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return IntegrationChannelConfig{}, fmt.Errorf("primary integration channel not found: %w", modelerror.ErrNotFound)
		}
		return IntegrationChannelConfig{}, fmt.Errorf("load primary integration channel failed: %w", err)
	}
	return channel, nil
}

func GetEnabledPrimaryOrDefaultIntegrationChannelConfig(ctx context.Context, scenario string) (IntegrationChannelConfig, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return IntegrationChannelConfig{}, fmt.Errorf("database unavailable: %w", err)
	}

	query := integrationChannelConfigSelectSQL() + `
		WHERE c.scenario = ? AND c.enabled = 1 AND cred.enabled = 1
		ORDER BY c.is_primary DESC, c.priority ASC, c.created_at DESC, c.id DESC
		LIMIT 1
	`
	var channel IntegrationChannelConfig
	if err := d.GetP(&channel, query, scenario); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return IntegrationChannelConfig{}, fmt.Errorf("enabled integration channel not found: %w", modelerror.ErrNotFound)
		}
		return IntegrationChannelConfig{}, fmt.Errorf("load enabled integration channel failed: %w", err)
	}
	return channel, nil
}

func integrationScenarioAllowsPrimary(scenario string) bool {
	switch scenario {
	case IntegrationScenarioOSS, IntegrationScenarioLLM, IntegrationScenarioEmbedding, IntegrationScenarioExternalNotification:
		return true
	default:
		return false
	}
}

func findEnabledChannel(ctx context.Context, scenario string, channelCode string) (IntegrationChannel, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return IntegrationChannel{}, fmt.Errorf("database unavailable: %w", err)
	}

	query := `
		SELECT id, scenario, channel_code, provider_code, adapter_key, environment, enabled, priority,
			credential_id, COALESCE(policy_id, '') AS policy_id, webhook_enabled, is_primary, config_json, metadata_json, created_at, updated_at
		FROM integration_channels
		WHERE scenario = ? AND enabled = 1
	`
	args := []interface{}{scenario}
	if channelCode != "" {
		query += " AND channel_code = ?"
		args = append(args, channelCode)
	}
	query += " ORDER BY is_primary DESC, priority ASC, channel_code ASC LIMIT 1"

	var channel IntegrationChannel
	if err := d.GetP(&channel, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return IntegrationChannel{}, fmt.Errorf("integration channel not found: %w", modelerror.ErrNotFound)
		}
		return IntegrationChannel{}, fmt.Errorf("load integration channel failed: %w", err)
	}
	return channel, nil
}

func getEnabledCredentialByID(ctx context.Context, id string) (IntegrationCredential, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return IntegrationCredential{}, fmt.Errorf("database unavailable: %w", err)
	}

	query := `
		SELECT id, credential_type, COALESCE(NULLIF(value_text, ''), ciphertext) AS value_text, enabled,
			COALESCE(rotated_at, '') AS rotated_at, created_at, updated_at
		FROM integration_credentials
		WHERE id = ? AND enabled = 1
	`
	var credential IntegrationCredential
	if err := d.GetP(&credential, query, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return IntegrationCredential{}, fmt.Errorf("integration credential not found: %w", modelerror.ErrNotFound)
		}
		return IntegrationCredential{}, fmt.Errorf("load integration credential failed: %w", err)
	}
	return credential, nil
}

func getIntegrationCredentialByID(ctx context.Context, id string) (IntegrationCredential, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return IntegrationCredential{}, fmt.Errorf("database unavailable: %w", err)
	}

	query := `
		SELECT id, credential_type, COALESCE(NULLIF(value_text, ''), ciphertext) AS value_text, enabled,
			COALESCE(rotated_at, '') AS rotated_at, created_at, updated_at
		FROM integration_credentials
		WHERE id = ?
	`
	var credential IntegrationCredential
	if err := d.GetP(&credential, query, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return IntegrationCredential{}, fmt.Errorf("integration credential not found: %w", modelerror.ErrNotFound)
		}
		return IntegrationCredential{}, fmt.Errorf("load integration credential failed: %w", err)
	}
	return credential, nil
}

func getEnabledPolicyByID(ctx context.Context, id string) (IntegrationPolicy, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return IntegrationPolicy{}, fmt.Errorf("database unavailable: %w", err)
	}

	query := `
		SELECT id, scenario, name, allowlist_json, denylist_json, rate_limit_json,
			risk_rules_json, enabled, created_at, updated_at
		FROM integration_policies
		WHERE id = ? AND enabled = 1
	`
	var policy IntegrationPolicy
	if err := d.GetP(&policy, query, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return IntegrationPolicy{}, fmt.Errorf("integration policy not found: %w", modelerror.ErrNotFound)
		}
		return IntegrationPolicy{}, fmt.Errorf("load integration policy failed: %w", err)
	}
	return policy, nil
}

func findEnabledModelOption(ctx context.Context, scenario string, channelID string, modelCode string) (IntegrationModelOption, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return IntegrationModelOption{}, fmt.Errorf("database unavailable: %w", err)
	}

	query := `
		SELECT id, scenario, channel_id, model_code, provider_model_id, capabilities_json,
			default_params_json, cost_policy_json, enabled, created_at, updated_at
		FROM integration_model_options
		WHERE scenario = ? AND channel_id = ? AND enabled = 1
	`
	args := []interface{}{scenario, channelID}
	if modelCode != "" {
		query += " AND model_code = ?"
		args = append(args, modelCode)
	}
	query += " ORDER BY model_code ASC LIMIT 1"

	var model IntegrationModelOption
	if err := d.GetP(&model, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return IntegrationModelOption{}, fmt.Errorf("integration model option not found: %w", modelerror.ErrNotFound)
		}
		return IntegrationModelOption{}, fmt.Errorf("load integration model option failed: %w", err)
	}
	return model, nil
}

func defaultLLMModelOptionForChannel(scenario string, channel IntegrationChannel, modelCode string, err error) (IntegrationModelOption, bool) {
	if !errors.Is(err, modelerror.ErrNotFound) {
		return IntegrationModelOption{}, false
	}
	if scenario != IntegrationScenarioLLM || channel.AdapterKey != deepSeekOpenAICompatibleAdapterKey {
		return IntegrationModelOption{}, false
	}

	normalizedModelCode := strings.TrimSpace(modelCode)
	if normalizedModelCode != "" && normalizedModelCode != defaultDeepSeekModelCode {
		return IntegrationModelOption{}, false
	}

	return IntegrationModelOption{
		Scenario:          IntegrationScenarioLLM,
		ChannelID:         channel.ID,
		ModelCode:         defaultDeepSeekModelCode,
		ProviderModelID:   defaultDeepSeekModelCode,
		CapabilitiesJSON:  "{}",
		DefaultParamsJSON: "{}",
		CostPolicyJSON:    "{}",
		Enabled:           1,
	}, true
}

func defaultEmbeddingModelOptionForChannel(scenario string, channel IntegrationChannel, modelCode string, err error) (IntegrationModelOption, bool) {
	if !errors.Is(err, modelerror.ErrNotFound) {
		return IntegrationModelOption{}, false
	}
	if scenario != IntegrationScenarioEmbedding {
		return IntegrationModelOption{}, false
	}

	normalizedModelCode := strings.TrimSpace(modelCode)

	switch channel.AdapterKey {
	case localHashEmbeddingAdapterKey:
		if normalizedModelCode != "" && normalizedModelCode != defaultLocalHashEmbeddingModelCode {
			return IntegrationModelOption{}, false
		}
		return IntegrationModelOption{
			Scenario:          IntegrationScenarioEmbedding,
			ChannelID:         channel.ID,
			ModelCode:         defaultLocalHashEmbeddingModelCode,
			ProviderModelID:   defaultLocalHashEmbeddingModelCode,
			CapabilitiesJSON:  "{}",
			DefaultParamsJSON: `{"dimensions":64}`,
			CostPolicyJSON:    "{}",
			Enabled:           1,
		}, true
	case siliconFlowEmbeddingOpenAICompatibleAdapterKey:
		if normalizedModelCode != "" && normalizedModelCode != defaultSiliconFlowEmbeddingModelCode {
			return IntegrationModelOption{}, false
		}
	default:
		return IntegrationModelOption{}, false
	}

	return IntegrationModelOption{
		Scenario:          IntegrationScenarioEmbedding,
		ChannelID:         channel.ID,
		ModelCode:         defaultSiliconFlowEmbeddingModelCode,
		ProviderModelID:   defaultSiliconFlowEmbeddingProviderModelID,
		CapabilitiesJSON:  "{}",
		DefaultParamsJSON: `{"dimensions":64,"encoding_format":"float"}`,
		CostPolicyJSON:    "{}",
		Enabled:           1,
	}, true
}

type CreateIntegrationModelOptionCmd struct {
	Scenario          string
	ChannelID         string
	ModelCode         string
	ProviderModelID   string
	CapabilitiesJSON  string
	DefaultParamsJSON string
	CostPolicyJSON    string
	Enabled           bool
}

func CreateIntegrationModelOption(ctx context.Context, cmd CreateIntegrationModelOptionCmd) (IntegrationModelOption, error) {
	now := timefmt.NowSQLiteDateTime()
	capabilitiesJSON := cmd.CapabilitiesJSON
	if capabilitiesJSON == "" {
		capabilitiesJSON = "{}"
	}
	defaultParamsJSON := cmd.DefaultParamsJSON
	if defaultParamsJSON == "" {
		defaultParamsJSON = "{}"
	}
	costPolicyJSON := cmd.CostPolicyJSON
	if costPolicyJSON == "" {
		costPolicyJSON = "{}"
	}

	model := IntegrationModelOption{
		ID:                uuid.Must(uuid.NewV7()).String(),
		Scenario:          cmd.Scenario,
		ChannelID:         cmd.ChannelID,
		ModelCode:         cmd.ModelCode,
		ProviderModelID:   cmd.ProviderModelID,
		CapabilitiesJSON:  capabilitiesJSON,
		DefaultParamsJSON: defaultParamsJSON,
		CostPolicyJSON:    costPolicyJSON,
		Enabled:           boolToInt(cmd.Enabled),
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return IntegrationModelOption{}, fmt.Errorf("database unavailable: %w", err)
	}
	query := `
			INSERT INTO integration_model_options (
				id, scenario, channel_id, model_code, provider_model_id, capabilities_json,
				default_params_json, cost_policy_json, enabled, created_at, updated_at
			) VALUES (
				:id, :scenario, :channel_id, :model_code, :provider_model_id, :capabilities_json,
				:default_params_json, :cost_policy_json, :enabled, :created_at, :updated_at
			)
		`
	if _, err := d.ExecNamed(query, model); err != nil {
		return IntegrationModelOption{}, fmt.Errorf("create integration model option failed: %w", err)
	}
	return model, nil
}

func SetIntegrationChannelModelOption(ctx context.Context, cmd CreateIntegrationModelOptionCmd) (IntegrationModelOption, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return IntegrationModelOption{}, fmt.Errorf("database unavailable: %w", err)
	}

	now := timefmt.NowSQLiteDateTime()
	disableQuery := `
		UPDATE integration_model_options
		SET enabled = 0, updated_at = ?
		WHERE scenario = ? AND channel_id = ? AND enabled = 1 AND model_code <> ?
	`
	if _, err := d.ExecP(disableQuery, now, cmd.Scenario, cmd.ChannelID, cmd.ModelCode); err != nil {
		return IntegrationModelOption{}, fmt.Errorf("disable old integration model options failed: %w", err)
	}

	enableQuery := `
		UPDATE integration_model_options
		SET provider_model_id = ?, enabled = 1, updated_at = ?
		WHERE scenario = ? AND channel_id = ? AND model_code = ?
	`
	result, err := d.ExecP(enableQuery, cmd.ProviderModelID, now, cmd.Scenario, cmd.ChannelID, cmd.ModelCode)
	if err != nil {
		return IntegrationModelOption{}, fmt.Errorf("update integration model option failed: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return IntegrationModelOption{}, fmt.Errorf("read integration model option affected rows failed: %w", err)
	}
	if rows == 0 {
		return CreateIntegrationModelOption(ctx, cmd)
	}
	return findEnabledModelOption(ctx, cmd.Scenario, cmd.ChannelID, cmd.ModelCode)
}

type CreateIntegrationOperationConfigCmd struct {
	Scenario    string
	Operation   string
	ChannelCode string
	ModelCode   string
	Enabled     bool
	ConfigJSON  string
}

func CreateIntegrationOperationConfig(ctx context.Context, cmd CreateIntegrationOperationConfigCmd) (IntegrationOperationConfig, error) {
	now := timefmt.NowSQLiteDateTime()
	configJSON := cmd.ConfigJSON
	if configJSON == "" {
		configJSON = "{}"
	}

	config := IntegrationOperationConfig{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Scenario:    cmd.Scenario,
		Operation:   cmd.Operation,
		ChannelCode: cmd.ChannelCode,
		ModelCode:   cmd.ModelCode,
		Enabled:     boolToInt(cmd.Enabled),
		ConfigJSON:  configJSON,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return IntegrationOperationConfig{}, fmt.Errorf("database unavailable: %w", err)
	}
	query := `
			INSERT INTO integration_operation_configs (
				id, scenario, operation, channel_code, model_code, enabled, config_json, created_at, updated_at
			) VALUES (
				:id, :scenario, :operation, :channel_code, :model_code, :enabled, :config_json, :created_at, :updated_at
			)
		`
	if _, err := d.ExecNamed(query, config); err != nil {
		return IntegrationOperationConfig{}, fmt.Errorf("create integration operation config failed: %w", err)
	}
	return config, nil
}

func UpsertIntegrationOperationConfig(ctx context.Context, cmd CreateIntegrationOperationConfigCmd) (IntegrationOperationConfig, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return IntegrationOperationConfig{}, fmt.Errorf("database unavailable: %w", err)
	}

	configJSON := cmd.ConfigJSON
	if configJSON == "" {
		configJSON = "{}"
	}
	query := `
		UPDATE integration_operation_configs
		SET channel_code = ?, model_code = ?, enabled = ?, config_json = ?, updated_at = ?
		WHERE scenario = ? AND operation = ?
	`
	result, err := d.ExecP(query, cmd.ChannelCode, cmd.ModelCode, boolToInt(cmd.Enabled), configJSON, timefmt.NowSQLiteDateTime(), cmd.Scenario, cmd.Operation)
	if err != nil {
		return IntegrationOperationConfig{}, fmt.Errorf("update integration operation config failed: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return IntegrationOperationConfig{}, fmt.Errorf("read integration operation config affected rows failed: %w", err)
	}
	if rows == 0 {
		return CreateIntegrationOperationConfig(ctx, cmd)
	}
	return findEnabledOperationConfig(ctx, cmd.Scenario, cmd.Operation)
}

type CreateIntegrationInvocationCmd struct {
	Scenario       string
	ChannelID      string
	ChannelCode    string
	ProviderCode   string
	Operation      string
	IdempotencyKey string
	ModelCode      string
	Status         string
}

func CreateIntegrationInvocation(ctx context.Context, cmd CreateIntegrationInvocationCmd) (IntegrationInvocation, error) {
	now := timefmt.NowSQLiteDateTime()
	invocation := IntegrationInvocation{
		ID:             uuid.Must(uuid.NewV7()).String(),
		Scenario:       cmd.Scenario,
		ChannelID:      cmd.ChannelID,
		ChannelCode:    cmd.ChannelCode,
		ProviderCode:   cmd.ProviderCode,
		Operation:      cmd.Operation,
		IdempotencyKey: cmd.IdempotencyKey,
		ModelCode:      cmd.ModelCode,
		Status:         cmd.Status,
		UsageJSON:      "{}",
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if invocation.Status == "" {
		invocation.Status = IntegrationInvocationStatusStarted
	}

	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return IntegrationInvocation{}, fmt.Errorf("database unavailable: %w", err)
	}
	query := `
		INSERT INTO integration_invocations (
			id, scenario, channel_id, channel_code, provider_code, operation, idempotency_key,
			model_code, status, usage_json, created_at, updated_at
		) VALUES (
			:id, :scenario, :channel_id, :channel_code, :provider_code, :operation, :idempotency_key,
			:model_code, :status, :usage_json, :created_at, :updated_at
		)
	`
	if _, err := d.ExecNamed(query, invocation); err != nil {
		return IntegrationInvocation{}, fmt.Errorf("create integration invocation failed: %w", err)
	}
	return invocation, nil
}

type CompleteIntegrationInvocationCmd struct {
	ID                string
	Status            string
	ProviderRequestID string
	ErrorCategory     string
	Retryable         bool
	UsageJSON         string
	DurationMS        int64
}

func CompleteIntegrationInvocation(ctx context.Context, cmd CompleteIntegrationInvocationCmd) error {
	retryable := 0
	if cmd.Retryable {
		retryable = 1
	}
	usageJSON := cmd.UsageJSON
	if usageJSON == "" {
		usageJSON = "{}"
	}

	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return fmt.Errorf("database unavailable: %w", err)
	}
	query := `
		UPDATE integration_invocations SET
			status = ?,
			provider_request_id = ?,
			error_category = ?,
			retryable = ?,
			usage_json = ?,
			duration_ms = ?,
			updated_at = ?
		WHERE id = ?
	`
	result, err := d.ExecP(
		query,
		cmd.Status,
		cmd.ProviderRequestID,
		cmd.ErrorCategory,
		retryable,
		usageJSON,
		cmd.DurationMS,
		timefmt.NowSQLiteDateTime(),
		cmd.ID,
	)
	if err != nil {
		return fmt.Errorf("complete integration invocation failed: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read affected rows failed: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("integration invocation not found: %w", modelerror.ErrNotFound)
	}
	return nil
}

func GetIntegrationInvocationByProviderRequestID(ctx context.Context, scenario string, providerRequestID string) (IntegrationInvocation, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return IntegrationInvocation{}, fmt.Errorf("database unavailable: %w", err)
	}

	query := `
		SELECT id, scenario, channel_id, channel_code, provider_code, operation, idempotency_key,
			model_code, provider_request_id, status, error_category, retryable, usage_json,
			duration_ms, created_at, updated_at
		FROM integration_invocations
		WHERE scenario = ? AND provider_request_id = ?
		ORDER BY created_at DESC, id DESC
		LIMIT 1
	`
	var invocation IntegrationInvocation
	if err := d.GetP(&invocation, query, scenario, providerRequestID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return IntegrationInvocation{}, fmt.Errorf("integration invocation not found: %w", modelerror.ErrNotFound)
		}
		return IntegrationInvocation{}, fmt.Errorf("load integration invocation failed: %w", err)
	}
	return invocation, nil
}

type CreateIntegrationWebhookReceiptCmd struct {
	Scenario          string
	ChannelID         string
	ChannelCode       string
	ProviderCode      string
	ProviderEventID   string
	IdempotencyKey    string
	PayloadHash       string
	PayloadCiphertext string
	SafeSnapshotJSON  string
	HeadersHash       string
	Status            string
}

func CreateIntegrationWebhookReceipt(ctx context.Context, cmd CreateIntegrationWebhookReceiptCmd) (IntegrationWebhookReceipt, bool, error) {
	if existing, err := GetIntegrationWebhookReceiptByIdempotencyKey(ctx, cmd.Scenario, cmd.IdempotencyKey); err == nil {
		return existing, false, nil
	} else if !errors.Is(err, modelerror.ErrNotFound) {
		return IntegrationWebhookReceipt{}, false, err
	}

	now := timefmt.NowSQLiteDateTime()
	receipt := IntegrationWebhookReceipt{
		ID:                uuid.Must(uuid.NewV7()).String(),
		Scenario:          cmd.Scenario,
		ChannelID:         cmd.ChannelID,
		ChannelCode:       cmd.ChannelCode,
		ProviderCode:      cmd.ProviderCode,
		ProviderEventID:   cmd.ProviderEventID,
		IdempotencyKey:    cmd.IdempotencyKey,
		PayloadHash:       cmd.PayloadHash,
		PayloadCiphertext: cmd.PayloadCiphertext,
		SafeSnapshotJSON:  cmd.SafeSnapshotJSON,
		HeadersHash:       cmd.HeadersHash,
		Status:            cmd.Status,
		ReceivedAt:        now,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if receipt.Status == "" {
		receipt.Status = IntegrationWebhookReceiptStatusReceived
	}
	if receipt.SafeSnapshotJSON == "" {
		receipt.SafeSnapshotJSON = "{}"
	}

	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return IntegrationWebhookReceipt{}, false, fmt.Errorf("database unavailable: %w", err)
	}
	query := `
		INSERT INTO integration_webhook_receipts (
			id, scenario, channel_id, channel_code, provider_code, provider_event_id,
			idempotency_key, payload_hash, payload_ciphertext, safe_snapshot_json,
			headers_hash, status, received_at, created_at, updated_at
		) VALUES (
			:id, :scenario, :channel_id, :channel_code, :provider_code, :provider_event_id,
			:idempotency_key, :payload_hash, :payload_ciphertext, :safe_snapshot_json,
			:headers_hash, :status, :received_at, :created_at, :updated_at
		) ON CONFLICT(scenario, idempotency_key) DO NOTHING
	`
	result, err := d.ExecNamed(query, receipt)
	if err != nil {
		return IntegrationWebhookReceipt{}, false, fmt.Errorf("create integration webhook receipt failed: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return IntegrationWebhookReceipt{}, false, fmt.Errorf("read affected rows failed: %w", err)
	}
	if rows == 0 {
		existing, err := GetIntegrationWebhookReceiptByIdempotencyKey(ctx, cmd.Scenario, cmd.IdempotencyKey)
		if err != nil {
			return IntegrationWebhookReceipt{}, false, err
		}
		return existing, false, nil
	}
	return receipt, true, nil
}

func GetIntegrationWebhookReceiptByIdempotencyKey(ctx context.Context, scenario string, idempotencyKey string) (IntegrationWebhookReceipt, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return IntegrationWebhookReceipt{}, fmt.Errorf("database unavailable: %w", err)
	}

	query := integrationWebhookReceiptSelectSQL() + `
		WHERE scenario = ? AND idempotency_key = ?
		LIMIT 1
	`
	var receipt IntegrationWebhookReceipt
	if err := d.GetP(&receipt, query, scenario, idempotencyKey); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return IntegrationWebhookReceipt{}, fmt.Errorf("integration webhook receipt not found: %w", modelerror.ErrNotFound)
		}
		return IntegrationWebhookReceipt{}, fmt.Errorf("load integration webhook receipt failed: %w", err)
	}
	return receipt, nil
}

func GetIntegrationWebhookReceiptByID(ctx context.Context, id string) (IntegrationWebhookReceipt, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return IntegrationWebhookReceipt{}, fmt.Errorf("database unavailable: %w", err)
	}

	query := integrationWebhookReceiptSelectSQL() + `
		WHERE id = ?
		LIMIT 1
	`
	var receipt IntegrationWebhookReceipt
	if err := d.GetP(&receipt, query, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return IntegrationWebhookReceipt{}, fmt.Errorf("integration webhook receipt not found: %w", modelerror.ErrNotFound)
		}
		return IntegrationWebhookReceipt{}, fmt.Errorf("load integration webhook receipt failed: %w", err)
	}
	return receipt, nil
}

func MarkIntegrationWebhookReceiptQueued(ctx context.Context, id string, messageID string) error {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return fmt.Errorf("database unavailable: %w", err)
	}
	query := `
		UPDATE integration_webhook_receipts
		SET status = ?, message_id = ?, updated_at = ?
		WHERE id = ?
	`
	return execReceiptUpdate(d, query, IntegrationWebhookReceiptStatusQueued, messageID, timefmt.NowSQLiteDateTime(), id)
}

func MarkIntegrationWebhookReceiptProcessing(ctx context.Context, id string) error {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return fmt.Errorf("database unavailable: %w", err)
	}
	query := `
		UPDATE integration_webhook_receipts
		SET status = ?, attempts = attempts + 1, updated_at = ?
		WHERE id = ?
	`
	return execReceiptUpdate(d, query, IntegrationWebhookReceiptStatusProcessing, timefmt.NowSQLiteDateTime(), id)
}

func MarkIntegrationWebhookReceiptSucceeded(ctx context.Context, id string) error {
	return markIntegrationWebhookReceiptDone(ctx, id, IntegrationWebhookReceiptStatusSucceeded, "")
}

func MarkIntegrationWebhookReceiptFailed(ctx context.Context, id string, errorCode string) error {
	return markIntegrationWebhookReceiptDone(ctx, id, IntegrationWebhookReceiptStatusFailed, errorCode)
}

func MarkIntegrationWebhookReceiptIgnored(ctx context.Context, id string, errorCode string) error {
	return markIntegrationWebhookReceiptDone(ctx, id, IntegrationWebhookReceiptStatusIgnored, errorCode)
}

func markIntegrationWebhookReceiptDone(ctx context.Context, id string, status string, errorCode string) error {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return fmt.Errorf("database unavailable: %w", err)
	}
	now := timefmt.NowSQLiteDateTime()
	query := `
		UPDATE integration_webhook_receipts SET
			status = ?,
			last_error_code = ?,
			processed_at = ?,
			updated_at = ?
		WHERE id = ?
	`
	return execReceiptUpdate(d, query, status, errorCode, now, now, id)
}

type receiptExecutor interface {
	ExecP(string, ...interface{}) (sql.Result, error)
}

func execReceiptUpdate(d receiptExecutor, query string, args ...interface{}) error {
	result, err := d.ExecP(query, args...)
	if err != nil {
		return fmt.Errorf("update integration webhook receipt failed: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read affected rows failed: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("integration webhook receipt not found: %w", modelerror.ErrNotFound)
	}
	return nil
}

func integrationWebhookReceiptSelectSQL() string {
	return `
		SELECT id, scenario, channel_id, channel_code, provider_code, provider_event_id,
			idempotency_key, payload_hash, payload_ciphertext, safe_snapshot_json,
			headers_hash, status, attempts, last_error_code, received_at,
			COALESCE(processed_at, '') AS processed_at, message_id, created_at, updated_at
		FROM integration_webhook_receipts
	`
}

func integrationChannelConfigSelectSQL() string {
	return `
		SELECT c.id, c.scenario, c.channel_code, c.provider_code, c.adapter_key, c.environment,
			c.enabled, c.priority, c.credential_id,
			cred.credential_type AS credential_type,
			COALESCE(NULLIF(cred.value_text, ''), cred.ciphertext) AS credential_value,
			COALESCE(c.policy_id, '') AS policy_id,
			c.webhook_enabled, c.is_primary, c.config_json, c.metadata_json,
			COALESCE(model.model_code, '') AS model_code,
			COALESCE(model.provider_model_id, '') AS provider_model_id,
			c.created_at, c.updated_at
		FROM integration_channels c
		INNER JOIN integration_credentials cred ON cred.id = c.credential_id
		LEFT JOIN integration_model_options model
			ON model.id = (
				SELECT m.id
				FROM integration_model_options m
				WHERE m.scenario = c.scenario AND m.channel_id = c.id AND m.enabled = 1
				ORDER BY m.model_code ASC, m.created_at DESC
				LIMIT 1
			)
	`
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func requireRowsAffected(result sql.Result, message string) error {
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read affected rows failed: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("%s: %w", message, modelerror.ErrNotFound)
	}
	return nil
}

func isSQLiteUniqueConstraint(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	return strings.Contains(message, "constraint failed") || strings.Contains(message, "UNIQUE constraint failed")
}
