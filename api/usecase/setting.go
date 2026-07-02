package usecase

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/tfnick/go-svelte-starter/api/framework/cache"
	"github.com/tfnick/go-svelte-starter/api/framework/data/modelerror"
	"github.com/tfnick/go-svelte-starter/api/framework/integrations/providererror"
	"github.com/tfnick/go-svelte-starter/api/framework/logging"
	"github.com/tfnick/go-svelte-starter/api/framework/timefmt"
	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
	"github.com/tfnick/go-svelte-starter/api/models"
	"github.com/tfnick/go-svelte-starter/api/usecase/integrations/oss"
)

const (
	defaultSiteLogoURL    = "/logo.png"
	siteLogoSettingKey    = "site.logo"
	maxSiteLogoBytes      = 2 * 1024 * 1024
	workerLimitSettingKey = "heavy_task.worker_limit"
	defaultWorkerLimit    = 1
	minWorkerLimit        = 1
	maxWorkerLimit        = 10
	claritySettingKey         = "site.clarity"
	clarityScriptSettingKey   = "site.clarity_script"
	clarityScriptTTL          = 365 * 24 * time.Hour
	pageViewEnabledSettingKey  = "site.page_view_enabled"
	pageViewEnabledCacheTTL    = 365 * 24 * time.Hour
)

var clarityProjectIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

type SiteSettingsQry struct{}

type SaveSiteLogoCmd struct {
	Filename    string
	ContentType string
	Size        int64
	Body        io.Reader
}

type SiteLogoObjectQry struct{}

type ClaritySettingsQry struct{}

type SaveClaritySettingsCmd struct {
	Enabled   bool
	ProjectID string
}

type SiteSettingsCo struct {
	LogoURL                     string
	LogoConfigured              bool
	LogoUpdatedAt               string
	LogoUploadAvailable         bool
	LogoUploadUnavailableReason string
}

type SiteLogoObjectCo struct {
	Body        io.ReadCloser
	ContentType string
	Size        int64
}

type ClaritySettingsCo struct {
	Enabled   bool
	ProjectID string
	UpdatedAt string
}

type siteLogoMetadata struct {
	ObjectKey    string `json:"object_key"`
	ContentType  string `json:"content_type"`
	Size         int64  `json:"size"`
	UpdatedAt    string `json:"updated_at"`
	ChannelCode  string `json:"channel_code"`
	ProviderCode string `json:"provider_code"`
	AdapterKey   string `json:"adapter_key"`
}

type claritySettingsJSON struct {
	Enabled   bool   `json:"enabled"`
	ProjectID string `json:"project_id"`
	UpdatedAt string `json:"updated_at"`
}

func GetSiteSettings(ctx fwusecase.Context, _ SiteSettingsQry) (SiteSettingsCo, error) {
	meta, found, err := loadSiteLogoMetadata(ctx)
	if err != nil {
		return SiteSettingsCo{}, err
	}
	uploadState := resolveSiteLogoUploadState(ctx)
	if !found {
		return withSiteLogoUploadState(defaultSiteSettings(), uploadState), nil
	}
	return withSiteLogoUploadState(siteSettingsFromLogoMetadata(meta), uploadState), nil
}

func SaveSiteLogo(ctx fwusecase.Context, cmd SaveSiteLogoCmd) (SiteSettingsCo, error) {
	if cmd.Body == nil {
		return SiteSettingsCo{}, fwusecase.E(fwusecase.CodeValidation, "logo file is required", nil)
	}
	if cmd.Size > maxSiteLogoBytes {
		return SiteSettingsCo{}, fwusecase.E(fwusecase.CodeValidation, "logo file is too large", nil)
	}

	payload, err := io.ReadAll(io.LimitReader(cmd.Body, maxSiteLogoBytes+1))
	if err != nil {
		return SiteSettingsCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to read logo file", err)
	}
	if len(payload) == 0 {
		return SiteSettingsCo{}, fwusecase.E(fwusecase.CodeValidation, "logo file is required", nil)
	}
	if len(payload) > maxSiteLogoBytes {
		return SiteSettingsCo{}, fwusecase.E(fwusecase.CodeValidation, "logo file is too large", nil)
	}

	contentType, err := normalizeSiteLogoContentType(cmd.ContentType, payload)
	if err != nil {
		return SiteSettingsCo{}, err
	}

	provider, err := primarySiteLogoProvider(ctx)
	if err != nil {
		return SiteSettingsCo{}, err
	}
	adapter, ok := registeredOSSAdapter(provider.Config.AdapterKey)
	if !ok {
		return SiteSettingsCo{}, fwusecase.E(fwusecase.CodeInternal, "logo storage is not configured", fmt.Errorf("OSS adapter not registered: %s", provider.Config.AdapterKey))
	}

	objectKey := "settings/site-logo" + siteLogoExtension(contentType)
	result, err := adapter.PutObject(ctx.Std(), provider.Config, oss.PutObjectRequest{
		Key:         objectKey,
		Body:        bytes.NewReader(payload),
		Size:        int64(len(payload)),
		ContentType: contentType,
		Metadata: map[string]string{
			"filename": strings.TrimSpace(cmd.Filename),
		},
	})
	if err != nil {
		return SiteSettingsCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to store logo", err)
	}
	if strings.TrimSpace(result.Key) != "" {
		objectKey = result.Key
	}

	meta := siteLogoMetadata{
		ObjectKey:    objectKey,
		ContentType:  contentType,
		Size:         int64(len(payload)),
		UpdatedAt:    timefmt.RFC3339Nano(timefmt.NowUTC()),
		ChannelCode:  provider.Config.ChannelCode,
		ProviderCode: provider.Config.ProviderCode,
		AdapterKey:   provider.Config.AdapterKey,
	}
	encoded, err := json.Marshal(meta)
	if err != nil {
		return SiteSettingsCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to encode logo settings", err)
	}
	if _, err := models.UpsertAppSetting(ctx.Std(), siteLogoSettingKey, string(encoded)); err != nil {
		return SiteSettingsCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to save logo settings", err)
	}

	return withSiteLogoUploadState(siteSettingsFromLogoMetadata(meta), siteLogoUploadAvailability{Available: true}), nil
}

func GetSiteLogoObject(ctx fwusecase.Context, _ SiteLogoObjectQry) (SiteLogoObjectCo, error) {
	meta, found, err := loadSiteLogoMetadata(ctx)
	if err != nil {
		return SiteLogoObjectCo{}, err
	}
	if !found {
		return SiteLogoObjectCo{}, fwusecase.E(fwusecase.CodeNotFound, "site logo is not configured", nil)
	}

	provider, err := siteLogoProviderFromMetadata(ctx, meta)
	if err != nil {
		return SiteLogoObjectCo{}, err
	}
	adapter, ok := registeredOSSAdapter(provider.Config.AdapterKey)
	if !ok {
		return SiteLogoObjectCo{}, fwusecase.E(fwusecase.CodeInternal, "logo storage is not configured", fmt.Errorf("OSS adapter not registered: %s", provider.Config.AdapterKey))
	}

	result, err := adapter.GetObject(ctx.Std(), provider.Config, oss.GetObjectRequest{Key: meta.ObjectKey})
	if err != nil {
		if providerErr, ok := providererror.From(err); ok && providerErr.Category == providererror.CategoryPermanent {
			return SiteLogoObjectCo{}, fwusecase.E(fwusecase.CodeNotFound, "site logo is not configured", err)
		}
		return SiteLogoObjectCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to load logo", err)
	}
	if result.Body == nil {
		return SiteLogoObjectCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to load logo", fmt.Errorf("OSS object body is empty"))
	}

	contentType := strings.TrimSpace(result.ContentType)
	if contentType == "" {
		contentType = meta.ContentType
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	return SiteLogoObjectCo{
		Body:        result.Body,
		ContentType: contentType,
		Size:        result.Size,
	}, nil
}

func GetClaritySettings(ctx fwusecase.Context, _ ClaritySettingsQry) (ClaritySettingsCo, error) {
	settings, _, err := loadClaritySettings(ctx)
	if err != nil {
		return ClaritySettingsCo{}, err
	}
	return claritySettingsFromStored(settings), nil
}

func SaveClaritySettings(ctx fwusecase.Context, cmd SaveClaritySettingsCmd) (ClaritySettingsCo, error) {
	settings, err := normalizeClaritySettings(cmd)
	if err != nil {
		return ClaritySettingsCo{}, err
	}

	encoded, err := json.Marshal(settings)
	if err != nil {
		return ClaritySettingsCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to encode Clarity settings", err)
	}

	script := buildClarityScript(settings.ProjectID, settings.Enabled)

	if err := fwusecase.WithAppTx(ctx, func(txCtx fwusecase.Context) error {
		if _, err := models.UpsertAppSetting(txCtx.Std(), claritySettingKey, string(encoded)); err != nil {
			return fwusecase.E(fwusecase.CodeInternal, "failed to save Clarity settings", err)
		}
		if _, err := models.UpsertAppSetting(txCtx.Std(), clarityScriptSettingKey, script); err != nil {
			return fwusecase.E(fwusecase.CodeInternal, "failed to save Clarity script", err)
		}
		return nil
	}); err != nil {
		return ClaritySettingsCo{}, err
	}

	if err := cache.SharedStore().Set(ctx.Std(), "app_setting", clarityScriptSettingKey, []byte(script), clarityScriptTTL); err != nil {
		return ClaritySettingsCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to cache Clarity script", err)
	}

	return claritySettingsFromStored(settings), nil
}

func CachedClarityScript(ctx fwusecase.Context) string {
	script, err := CachedClarityScriptWithError(ctx)
	if err != nil {
		logger := logging.For("settings")
		logger.Warn().Err(err).Msg("failed to load Clarity script")
		return ""
	}
	return script
}

func CachedClarityScriptWithError(ctx fwusecase.Context) (string, error) {
	const ns = "app_setting"

	store := cache.SharedStore()
	raw, ok, err := store.Get(ctx.Std(), ns, clarityScriptSettingKey)
	if err != nil {
		return "", fwusecase.E(fwusecase.CodeInternal, "failed to load Clarity cache", err)
	}
	if ok {
		return string(raw), nil
	}

	setting, err := models.GetAppSetting(ctx.Std(), clarityScriptSettingKey)
	if errors.Is(err, modelerror.ErrNotFound) {
		return "", nil
	}
	if err != nil {
		return "", fwusecase.E(fwusecase.CodeInternal, "failed to load Clarity script", err)
	}

	if err := store.Set(ctx.Std(), ns, clarityScriptSettingKey, []byte(setting.ValueJSON), clarityScriptTTL); err != nil {
		return "", fwusecase.E(fwusecase.CodeInternal, "failed to cache Clarity script", err)
	}

	return setting.ValueJSON, nil
}

func loadSiteLogoMetadata(ctx fwusecase.Context) (siteLogoMetadata, bool, error) {
	setting, err := cache.Cached(ctx.Std(), "app_setting", siteLogoSettingKey, 30*time.Minute, func() (models.AppSetting, error) {
		return models.GetAppSetting(ctx.Std(), siteLogoSettingKey)
	})
	if err != nil {
		if errors.Is(err, modelerror.ErrNotFound) {
			return siteLogoMetadata{}, false, nil
		}
		return siteLogoMetadata{}, false, fwusecase.E(fwusecase.CodeInternal, "failed to load site settings", err)
	}

	var meta siteLogoMetadata
	if err := json.Unmarshal([]byte(setting.ValueJSON), &meta); err != nil {
		return siteLogoMetadata{}, false, fwusecase.E(fwusecase.CodeInternal, "failed to parse site settings", err)
	}
	if strings.TrimSpace(meta.ObjectKey) == "" {
		return siteLogoMetadata{}, false, nil
	}
	return meta, true, nil
}

func loadClaritySettings(ctx fwusecase.Context) (claritySettingsJSON, bool, error) {
	setting, err := cache.Cached(ctx.Std(), "app_setting", claritySettingKey, 30*time.Minute, func() (models.AppSetting, error) {
		return models.GetAppSetting(ctx.Std(), claritySettingKey)
	})
	if err != nil {
		if errors.Is(err, modelerror.ErrNotFound) {
			return claritySettingsJSON{}, false, nil
		}
		return claritySettingsJSON{}, false, fwusecase.E(fwusecase.CodeInternal, "failed to load Clarity settings", err)
	}

	var stored claritySettingsJSON
	if err := json.Unmarshal([]byte(setting.ValueJSON), &stored); err != nil {
		return claritySettingsJSON{}, false, fwusecase.E(fwusecase.CodeInternal, "failed to parse Clarity settings", err)
	}
	stored.ProjectID = strings.TrimSpace(stored.ProjectID)
	if stored.ProjectID == "" {
		stored.Enabled = false
	}
	if stored.UpdatedAt == "" {
		stored.UpdatedAt = setting.UpdatedAt
	}
	return stored, true, nil
}

func normalizeClaritySettings(cmd SaveClaritySettingsCmd) (claritySettingsJSON, error) {
	projectID := strings.TrimSpace(cmd.ProjectID)
	if projectID != "" && !clarityProjectIDPattern.MatchString(projectID) {
		return claritySettingsJSON{}, fwusecase.E(fwusecase.CodeValidation, "Clarity project ID is invalid", nil)
	}
	if cmd.Enabled && projectID == "" {
		return claritySettingsJSON{}, fwusecase.E(fwusecase.CodeValidation, "Clarity project ID is required", nil)
	}

	return claritySettingsJSON{
		Enabled:   cmd.Enabled,
		ProjectID: projectID,
		UpdatedAt: timefmt.RFC3339Nano(timefmt.NowUTC()),
	}, nil
}

func claritySettingsFromStored(settings claritySettingsJSON) ClaritySettingsCo {
	return ClaritySettingsCo{
		Enabled:   settings.Enabled && strings.TrimSpace(settings.ProjectID) != "",
		ProjectID: strings.TrimSpace(settings.ProjectID),
		UpdatedAt: settings.UpdatedAt,
	}
}

func buildClarityScript(projectID string, enabled bool) string {
	projectID = strings.TrimSpace(projectID)
	if !enabled || projectID == "" || !clarityProjectIDPattern.MatchString(projectID) {
		return ""
	}

	encodedID, err := json.Marshal(projectID)
	if err != nil {
		return ""
	}
	return `<script type="text/javascript">
(function(c,l,a,r,i,t,y){
c[a]=c[a]||function(){(c[a].q=c[a].q||[]).push(arguments)};
t=l.createElement(r);t.async=1;t.src="https://www.clarity.ms/tag/"+i;
y=l.getElementsByTagName(r)[0];y.parentNode.insertBefore(t,y);
})(window, document, "clarity", "script", ` + string(encodedID) + `);
</script>`
}

func defaultSiteSettings() SiteSettingsCo {
	return SiteSettingsCo{
		LogoURL:        defaultSiteLogoURL,
		LogoConfigured: false,
	}
}

func siteSettingsFromLogoMetadata(meta siteLogoMetadata) SiteSettingsCo {
	logoURL := "/api/public/settings/logo"
	if meta.UpdatedAt != "" {
		logoURL += "?v=" + url.QueryEscape(meta.UpdatedAt)
	}
	return SiteSettingsCo{
		LogoURL:        logoURL,
		LogoConfigured: true,
		LogoUpdatedAt:  meta.UpdatedAt,
	}
}

type siteLogoUploadAvailability struct {
	Available bool
	Reason    string
}

type siteLogoProvider struct {
	Config oss.ProviderConfig
}

type ossChannelConfigJSON struct {
	EndpointURL   string `json:"endpoint_url"`
	Bucket        string `json:"bucket"`
	Region        string `json:"region"`
	PublicBaseURL string `json:"public_base_url"`
	KeyPrefix     string `json:"key_prefix"`
	UsePathStyle  *bool  `json:"use_path_style"`
}

type ossChannelCredentialJSON struct {
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
}

func withSiteLogoUploadState(settings SiteSettingsCo, state siteLogoUploadAvailability) SiteSettingsCo {
	settings.LogoUploadAvailable = state.Available
	settings.LogoUploadUnavailableReason = state.Reason
	return settings
}

func resolveSiteLogoUploadState(ctx fwusecase.Context) siteLogoUploadAvailability {
	provider, err := primarySiteLogoProvider(ctx)
	if err != nil {
		return siteLogoUploadAvailability{Available: false, Reason: "Primary OSS provider is not configured"}
	}
	if _, ok := registeredOSSAdapter(provider.Config.AdapterKey); !ok {
		return siteLogoUploadAvailability{Available: false, Reason: "Primary OSS provider adapter is not available"}
	}
	return siteLogoUploadAvailability{Available: true}
}

func primarySiteLogoProvider(ctx fwusecase.Context) (siteLogoProvider, error) {
	channel, err := cache.Cached(ctx.Std(), "config.oss", "primary_enabled", 5*time.Minute, func() (models.IntegrationChannelConfig, error) {
		return models.GetEnabledPrimaryOSSChannelConfig(ctx.Std())
	})
	if err != nil {
		if errors.Is(err, modelerror.ErrNotFound) {
			return siteLogoProvider{}, fwusecase.E(fwusecase.CodeValidation, "primary OSS provider is not configured", err)
		}
		return siteLogoProvider{}, fwusecase.E(fwusecase.CodeInternal, "failed to load primary OSS provider", err)
	}
	return siteLogoProviderFromChannel(channel)
}

func siteLogoProviderFromMetadata(ctx fwusecase.Context, meta siteLogoMetadata) (siteLogoProvider, error) {
	if strings.TrimSpace(meta.AdapterKey) == "" || strings.TrimSpace(meta.ChannelCode) == "" {
		return primarySiteLogoProvider(ctx)
	}

	channel, err := models.GetOSSChannelConfigByCodeAndAdapter(ctx.Std(), strings.TrimSpace(meta.ChannelCode), strings.TrimSpace(meta.AdapterKey))
	if err != nil {
		if errors.Is(err, modelerror.ErrNotFound) {
			return siteLogoProvider{}, fwusecase.E(fwusecase.CodeInternal, "logo storage is not configured", err)
		}
		return siteLogoProvider{}, fwusecase.E(fwusecase.CodeInternal, "failed to load OSS provider", err)
	}
	return siteLogoProviderFromChannel(channel)
}

func siteLogoProviderFromChannel(channel models.IntegrationChannelConfig) (siteLogoProvider, error) {
	var cfg ossChannelConfigJSON
	if err := json.Unmarshal([]byte(strings.TrimSpace(channel.ConfigJSON)), &cfg); err != nil {
		return siteLogoProvider{}, fwusecase.E(fwusecase.CodeInternal, "OSS provider config is invalid", err)
	}
	var credential ossChannelCredentialJSON
	if err := json.Unmarshal([]byte(strings.TrimSpace(channel.CredentialValue)), &credential); err != nil {
		return siteLogoProvider{}, fwusecase.E(fwusecase.CodeInternal, "OSS provider credential is invalid", err)
	}

	providerCfg := oss.ProviderConfig{
		ChannelCode:     channel.ChannelCode,
		ProviderCode:    channel.ProviderCode,
		AdapterKey:      channel.AdapterKey,
		EndpointURL:     strings.TrimSpace(cfg.EndpointURL),
		Bucket:          strings.TrimSpace(cfg.Bucket),
		Region:          strings.TrimSpace(cfg.Region),
		PublicBaseURL:   strings.TrimRight(strings.TrimSpace(cfg.PublicBaseURL), "/"),
		KeyPrefix:       strings.Trim(strings.TrimSpace(cfg.KeyPrefix), "/"),
		UsePathStyle:    cfg.UsePathStyle,
		AccessKeyID:     strings.TrimSpace(credential.AccessKeyID),
		SecretAccessKey: strings.TrimSpace(credential.SecretAccessKey),
	}
	if providerCfg.EndpointURL == "" || providerCfg.Bucket == "" {
		return siteLogoProvider{}, fwusecase.E(fwusecase.CodeInternal, "OSS provider config is invalid", fmt.Errorf("endpoint_url and bucket are required"))
	}
	if providerCfg.AccessKeyID == "" || providerCfg.SecretAccessKey == "" {
		return siteLogoProvider{}, fwusecase.E(fwusecase.CodeInternal, "OSS provider credential is invalid", fmt.Errorf("access key id and secret access key are required"))
	}
	return siteLogoProvider{Config: providerCfg}, nil
}

func normalizeSiteLogoContentType(_ string, payload []byte) (string, error) {
	if len(payload) >= 8 && bytes.Equal(payload[:8], []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}) {
		return "image/png", nil
	}
	if len(payload) >= 3 && payload[0] == 0xff && payload[1] == 0xd8 && payload[2] == 0xff {
		return "image/jpeg", nil
	}
	if len(payload) >= 12 && string(payload[:4]) == "RIFF" && string(payload[8:12]) == "WEBP" {
		return "image/webp", nil
	}

	return "", fwusecase.E(fwusecase.CodeValidation, "logo image type is not supported", nil)
}

func GetWorkerLimit(ctx fwusecase.Context) (int, error) {
	setting, err := cache.Cached(ctx.Std(), "app_setting", workerLimitSettingKey, 30*time.Minute, func() (models.AppSetting, error) {
		return models.GetAppSetting(ctx.Std(), workerLimitSettingKey)
	})
	if err != nil {
		if errors.Is(err, modelerror.ErrNotFound) {
			return defaultWorkerLimit, nil
		}
		return 0, fwusecase.E(fwusecase.CodeInternal, "failed to load worker limit setting", err)
	}

	var limit int
	if err := json.Unmarshal([]byte(setting.ValueJSON), &limit); err != nil {
		return defaultWorkerLimit, nil
	}
	if limit < minWorkerLimit {
		return minWorkerLimit, nil
	}
	if limit > maxWorkerLimit {
		return maxWorkerLimit, nil
	}
	return limit, nil
}

func SaveWorkerLimit(ctx fwusecase.Context, limit int) (int, error) {
	if limit < minWorkerLimit || limit > maxWorkerLimit {
		return 0, fwusecase.E(fwusecase.CodeValidation, fmt.Sprintf("worker limit must be between %d and %d", minWorkerLimit, maxWorkerLimit), nil)
	}

	encoded, err := json.Marshal(limit)
	if err != nil {
		return 0, fwusecase.E(fwusecase.CodeInternal, "failed to encode worker limit", err)
	}

	if _, err := models.UpsertAppSetting(ctx.Std(), workerLimitSettingKey, string(encoded)); err != nil {
		return 0, fwusecase.E(fwusecase.CodeInternal, "failed to save worker limit setting", err)
	}

	return limit, nil
}

const (
	marketingDomainSettingKey = "site.marketing_domain"
	defaultMarketingDomain    = "example.com"
)

type MarketingDomainQry struct{}

type SaveMarketingDomainCmd struct {
	Domain string `json:"domain"`
}

type MarketingDomainCo struct {
	Domain string `json:"domain"`
}

func GetPageViewEnabled(ctx fwusecase.Context) (bool, error) {
	enabled, err := cache.Cached(ctx.Std(), "app_setting", pageViewEnabledSettingKey, pageViewEnabledCacheTTL, func() (bool, error) {
		setting, err := models.GetAppSetting(ctx.Std(), pageViewEnabledSettingKey)
		if errors.Is(err, modelerror.ErrNotFound) {
			return true, nil
		}
		if err != nil {
			return false, err
		}
		var enabled bool
		if err := json.Unmarshal([]byte(setting.ValueJSON), &enabled); err != nil {
			return true, nil
		}
		return enabled, nil
	})
	if err != nil {
		return true, fwusecase.E(fwusecase.CodeInternal, "failed to load page view setting", err)
	}
	return enabled, nil
}

func SavePageViewEnabled(ctx fwusecase.Context, enabled bool) (bool, error) {
	encoded, err := json.Marshal(enabled)
	if err != nil {
		return false, fwusecase.E(fwusecase.CodeInternal, "failed to encode page view setting", err)
	}
	if _, err := models.UpsertAppSetting(ctx.Std(), pageViewEnabledSettingKey, string(encoded)); err != nil {
		return false, fwusecase.E(fwusecase.CodeInternal, "failed to save page view setting", err)
	}
	_ = cache.SharedStore().Delete(ctx.Std(), "app_setting", pageViewEnabledSettingKey)
	return enabled, nil
}

func GetMarketingDomain(ctx fwusecase.Context, _ MarketingDomainQry) (MarketingDomainCo, error) {
	setting, err := cache.Cached(ctx.Std(), "app_setting", marketingDomainSettingKey, 30*time.Minute, func() (models.AppSetting, error) {
		return models.GetAppSetting(ctx.Std(), marketingDomainSettingKey)
	})
	if err != nil {
		if errors.Is(err, modelerror.ErrNotFound) {
			return MarketingDomainCo{Domain: defaultMarketingDomain}, nil
		}
		return MarketingDomainCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to load marketing domain setting", err)
	}

	var domain string
	if err := json.Unmarshal([]byte(setting.ValueJSON), &domain); err != nil {
		return MarketingDomainCo{Domain: defaultMarketingDomain}, nil
	}
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return MarketingDomainCo{Domain: defaultMarketingDomain}, nil
	}
	return MarketingDomainCo{Domain: domain}, nil
}

func SaveMarketingDomain(ctx fwusecase.Context, cmd SaveMarketingDomainCmd) (MarketingDomainCo, error) {
	domain := strings.TrimSpace(cmd.Domain)
	if domain == "" {
		return MarketingDomainCo{}, fwusecase.E(fwusecase.CodeValidation, "domain is required", nil)
	}
	if strings.ContainsAny(domain, " \t\n\r/") {
		return MarketingDomainCo{}, fwusecase.E(fwusecase.CodeValidation, "domain is invalid", nil)
	}

	encoded, err := json.Marshal(domain)
	if err != nil {
		return MarketingDomainCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to encode marketing domain", err)
	}

	if _, err := models.UpsertAppSetting(ctx.Std(), marketingDomainSettingKey, string(encoded)); err != nil {
		return MarketingDomainCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to save marketing domain setting", err)
	}

	return MarketingDomainCo{Domain: domain}, nil
}

type MarketingChannelCo struct {
	Code string `json:"code"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

type MarketingChannelsCo struct {
	Domain   string               `json:"domain"`
	Channels []MarketingChannelCo `json:"channels"`
}

func GetMarketingChannels(ctx fwusecase.Context) (MarketingChannelsCo, error) {
	domain, err := GetMarketingDomain(ctx, MarketingDomainQry{})
	if err != nil {
		return MarketingChannelsCo{}, err
	}

	dictionaries, err := cache.Cached(ctx.Std(), "dictionary.options", "utm_channel", 30*time.Minute, func() (map[string][]models.DictionaryValue, error) {
		return models.ListDictionaryOptions(ctx.Std(), []string{"utm_channel"})
	})
	if err != nil {
		return MarketingChannelsCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to load marketing channels", err)
	}

	channels := make([]MarketingChannelCo, 0, len(dictionaries["utm_channel"]))
	for _, dv := range dictionaries["utm_channel"] {
		channels = append(channels, MarketingChannelCo{
			Code: dv.ValueCode,
			Name: dv.Label,
			URL:  fmt.Sprintf("https://%s/?utm_source=%s", domain.Domain, dv.ValueCode),
		})
	}

	return MarketingChannelsCo{
		Domain:   domain.Domain,
		Channels: channels,
	}, nil
}

func siteLogoExtension(contentType string) string {
	switch contentType {
	case "image/jpeg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	default:
		return ".png"
	}
}
