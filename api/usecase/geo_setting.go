package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/tfnick/go-svelte-starter/api/framework/data/modelerror"
	"github.com/tfnick/go-svelte-starter/api/framework/timefmt"
	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
	"github.com/tfnick/go-svelte-starter/api/models"
)

const (
	geoIP2RegionSettingKey = "site.geo.ip2region"

	GeoXDBVersionV4 = "v4"
	GeoXDBVersionV6 = "v6"

	defaultGeoV4XDBPath       = "geo/ip2region_v4.xdb"
	defaultGeoV6XDBPath       = "geo/ip2region_v6.xdb"
	defaultGeoCachePolicy     = "vectorIndex"
	defaultGeoSearcherPool    = 20
	minGeoSearcherPool        = 1
	maxGeoSearcherPool        = 100
	geoXDBDownloadHTTPTimeout = 5 * time.Minute
)

var defaultGeoXDBDownloadSources = map[string]string{
	GeoXDBVersionV4: "https://raw.githubusercontent.com/lionsoul2014/ip2region/master/data/ip2region_v4.xdb",
	GeoXDBVersionV6: "https://raw.githubusercontent.com/lionsoul2014/ip2region/master/data/ip2region_v6.xdb",
}

type GeoSettingsQry struct{}

type SaveGeoSettingsCmd struct {
	Enabled          bool
	V4XDB            string
	V6XDB            string
	CachePolicy      string
	SearcherPoolSize int
}

type CheckGeoXDBCmd struct{}

type EnqueueGeoXDBDownloadCmd struct {
	UserID  string
	Version string
}

type GeoSettingsCo struct {
	Enabled          bool
	V4XDB            string
	V6XDB            string
	CachePolicy      string
	SearcherPoolSize int
	XDBChecked       bool
	UpdatedAt        string
}

type GeoXDBCheckCo struct {
	Valid    bool
	Message  string
	Settings GeoSettingsCo
}

type GeoXDBDownloadCo struct {
	TaskID  string
	Version string
	Message string
}

type GeoIP2RegionResolverConfig struct {
	V4XDBPath       string
	V6XDBPath       string
	CachePolicy     string
	PoolSize        int
	SettingsVersion string
}

type GeoIP2RegionResolverFactory func(GeoIP2RegionResolverConfig) (RegistrationGeoResolver, func(), error)

type GeoIP2RegionXDBValidatorConfig struct {
	Version     string
	XDBPath     string
	CachePolicy string
}

type GeoIP2RegionXDBValidator func(GeoIP2RegionXDBValidatorConfig) error

type geoSettingsJSON struct {
	Enabled          bool   `json:"enabled"`
	V4XDB            string `json:"v4_xdb"`
	V6XDB            string `json:"v6_xdb"`
	CachePolicy      string `json:"cache_policy"`
	SearcherPoolSize int    `json:"searcher_pool_size"`
	XDBChecked       bool   `json:"xdb_checked"`
	UpdatedAt        string `json:"updated_at"`
}

type geoXDBDownloadPayload struct {
	Version          string `json:"version"`
	TargetPath       string `json:"target_path"`
	CachePolicy      string `json:"cache_policy"`
	SearcherPoolSize int    `json:"searcher_pool_size"`
	RequestedAt      string `json:"requested_at"`
}

type geoXDBDownloadResult struct {
	Version  string `json:"version"`
	Target   string `json:"target"`
	Size     int64  `json:"size"`
	Source   string `json:"source"`
	Finished string `json:"finished_at"`
}

type geoXDBDownloaderConfig struct {
	client  *http.Client
	sources map[string]string
}

var (
	geoResolverFactoryMu sync.RWMutex
	geoResolverFactory   GeoIP2RegionResolverFactory
	geoXDBValidator      GeoIP2RegionXDBValidator

	geoXDBDownloaderMu sync.RWMutex
	geoXDBDownloader   = geoXDBDownloaderConfig{
		client:  &http.Client{Timeout: geoXDBDownloadHTTPTimeout},
		sources: cloneGeoXDBSources(defaultGeoXDBDownloadSources),
	}
)

func RegisterGeoIP2RegionResolverFactory(factory GeoIP2RegionResolverFactory) func() {
	geoResolverFactoryMu.Lock()
	previous := geoResolverFactory
	geoResolverFactory = factory
	geoResolverFactoryMu.Unlock()

	return func() {
		geoResolverFactoryMu.Lock()
		geoResolverFactory = previous
		geoResolverFactoryMu.Unlock()
	}
}

func RegisterGeoIP2RegionXDBValidator(validator GeoIP2RegionXDBValidator) func() {
	geoResolverFactoryMu.Lock()
	previous := geoXDBValidator
	geoXDBValidator = validator
	geoResolverFactoryMu.Unlock()

	return func() {
		geoResolverFactoryMu.Lock()
		geoXDBValidator = previous
		geoResolverFactoryMu.Unlock()
	}
}

func ConfigureGeoXDBDownloader(client *http.Client, sources map[string]string) func() {
	geoXDBDownloaderMu.Lock()
	previous := geoXDBDownloader
	if client == nil {
		client = &http.Client{Timeout: geoXDBDownloadHTTPTimeout}
	}
	geoXDBDownloader = geoXDBDownloaderConfig{
		client:  client,
		sources: cloneGeoXDBSources(sources),
	}
	if len(geoXDBDownloader.sources) == 0 {
		geoXDBDownloader.sources = cloneGeoXDBSources(defaultGeoXDBDownloadSources)
	}
	geoXDBDownloaderMu.Unlock()

	return func() {
		geoXDBDownloaderMu.Lock()
		geoXDBDownloader = previous
		geoXDBDownloaderMu.Unlock()
	}
}

func GetGeoSettings(ctx fwusecase.Context, _ GeoSettingsQry) (GeoSettingsCo, error) {
	settings, _, err := loadGeoSettings(ctx)
	if err != nil {
		return GeoSettingsCo{}, err
	}
	return geoSettingsFromStored(settings), nil
}

func SaveGeoSettings(ctx fwusecase.Context, cmd SaveGeoSettingsCmd) (GeoSettingsCo, error) {
	previous, found, err := loadGeoSettings(ctx)
	if err != nil {
		return GeoSettingsCo{}, err
	}

	settings, err := normalizeGeoSettingsForSave(cmd, previous, found)
	if err != nil {
		return GeoSettingsCo{}, err
	}
	if err := saveGeoSettings(ctx, settings); err != nil {
		return GeoSettingsCo{}, err
	}
	return geoSettingsFromStored(settings), nil
}

func CheckGeoXDB(ctx fwusecase.Context, _ CheckGeoXDBCmd) (GeoXDBCheckCo, error) {
	settings, _, err := loadGeoSettings(ctx)
	if err != nil {
		return GeoXDBCheckCo{}, err
	}

	valid := true
	message := "ip2region xdb files are ready"
	if err := validateGeoXDBFiles(settings); err != nil {
		valid = false
		message = "ip2region xdb files are not ready"
	}

	settings.XDBChecked = valid
	settings.UpdatedAt = timefmt.RFC3339Nano(timefmt.NowUTC())
	if err := saveGeoSettings(ctx, settings); err != nil {
		return GeoXDBCheckCo{}, err
	}

	return GeoXDBCheckCo{
		Valid:    valid,
		Message:  message,
		Settings: geoSettingsFromStored(settings),
	}, nil
}

func EnqueueGeoXDBDownload(ctx fwusecase.Context, cmd EnqueueGeoXDBDownloadCmd) (GeoXDBDownloadCo, error) {
	userID := strings.TrimSpace(cmd.UserID)
	if userID == "" {
		return GeoXDBDownloadCo{}, fwusecase.E(fwusecase.CodeValidation, "user ID is required", nil)
	}

	version, err := normalizeGeoXDBVersion(cmd.Version)
	if err != nil {
		return GeoXDBDownloadCo{}, err
	}

	settings, _, err := loadGeoSettings(ctx)
	if err != nil {
		return GeoXDBDownloadCo{}, err
	}
	targetPath := settings.V4XDB
	if version == GeoXDBVersionV6 {
		targetPath = settings.V6XDB
	}
	if _, err := geoXDBRuntimePath(targetPath); err != nil {
		return GeoXDBDownloadCo{}, err
	}

	payload := geoXDBDownloadPayload{
		Version:          version,
		TargetPath:       targetPath,
		CachePolicy:      settings.CachePolicy,
		SearcherPoolSize: settings.SearcherPoolSize,
		RequestedAt:      timefmt.RFC3339Nano(timefmt.NowUTC()),
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return GeoXDBDownloadCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to encode geo download task", err)
	}

	settings.XDBChecked = false
	settings.UpdatedAt = timefmt.RFC3339Nano(timefmt.NowUTC())

	var result EnqueueHeavyTaskResult
	if err := fwusecase.WithAppTx(ctx, func(txCtx fwusecase.Context) error {
		if err := saveGeoSettings(txCtx, settings); err != nil {
			return err
		}
		task, err := EnqueueHeavyTask(txCtx, EnqueueHeavyTaskCmd{
			UserID:      userID,
			TaskType:    HeavyTaskTypeGeoXDBDownload,
			PayloadJSON: string(encoded),
		})
		if err != nil {
			return err
		}
		result = task
		return nil
	}); err != nil {
		return GeoXDBDownloadCo{}, err
	}

	return GeoXDBDownloadCo{
		TaskID:  result.TaskID,
		Version: version,
		Message: "geo xdb download task enqueued",
	}, nil
}

type SettingsBackedRegistrationGeoResolver struct {
	mu        sync.Mutex
	signature string
	resolver  RegistrationGeoResolver
	closeFn   func()
}

func NewSettingsBackedRegistrationGeoResolver() *SettingsBackedRegistrationGeoResolver {
	return &SettingsBackedRegistrationGeoResolver{}
}

func (r *SettingsBackedRegistrationGeoResolver) ResolveRegistrationGeo(ctx fwusecase.Context, ip string) (RegistrationGeo, error) {
	settings, _, err := loadGeoSettings(ctx)
	if err != nil {
		return RegistrationGeo{}, err
	}
	if !settings.Enabled || !settings.XDBChecked {
		r.clearCachedResolver()
		return RegistrationGeo{}, nil
	}

	config, err := geoResolverConfigFromSettings(settings)
	if err != nil {
		return RegistrationGeo{}, err
	}
	resolver, err := r.resolverFor(config)
	if err != nil {
		return RegistrationGeo{}, err
	}
	return resolver.ResolveRegistrationGeo(ctx, ip)
}

func (r *SettingsBackedRegistrationGeoResolver) Close() {
	r.clearCachedResolver()
}

func (r *SettingsBackedRegistrationGeoResolver) resolverFor(config GeoIP2RegionResolverConfig) (RegistrationGeoResolver, error) {
	signature := geoResolverSignature(config)

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.resolver != nil && r.signature == signature {
		return r.resolver, nil
	}

	if r.closeFn != nil {
		r.closeFn()
	}
	r.resolver = nil
	r.closeFn = nil
	r.signature = ""

	resolver, closeFn, err := newGeoIP2RegionResolver(config)
	if err != nil {
		return nil, err
	}
	r.resolver = resolver
	r.closeFn = closeFn
	r.signature = signature
	return resolver, nil
}

func (r *SettingsBackedRegistrationGeoResolver) clearCachedResolver() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closeFn != nil {
		r.closeFn()
	}
	r.resolver = nil
	r.closeFn = nil
	r.signature = ""
}

func executeGeoXDBDownloadTask(ctx context.Context, task models.AsyncTask) (heavyTaskExecutionResult, error) {
	payload, err := parseGeoXDBDownloadPayload(task.PayloadJSON)
	if err != nil {
		return heavyTaskExecutionResult{}, err
	}

	sourceURL, err := geoXDBDownloadSource(payload.Version)
	if err != nil {
		return heavyTaskExecutionResult{}, err
	}

	targetPath, err := geoXDBRuntimePath(payload.TargetPath)
	if err != nil {
		return heavyTaskExecutionResult{}, err
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return heavyTaskExecutionResult{}, fwusecase.E(fwusecase.CodeInternal, "failed to prepare geo data directory", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(targetPath), "."+filepath.Base(targetPath)+"-*.tmp")
	if err != nil {
		return heavyTaskExecutionResult{}, fwusecase.E(fwusecase.CodeInternal, "failed to create geo data file", err)
	}
	tmpPath := tmp.Name()
	removeTmp := true
	defer func() {
		_ = tmp.Close()
		if removeTmp {
			_ = os.Remove(tmpPath)
		}
	}()

	size, err := downloadGeoXDB(ctx, sourceURL, tmp)
	if err != nil {
		return heavyTaskExecutionResult{}, err
	}
	if err := tmp.Close(); err != nil {
		return heavyTaskExecutionResult{}, fwusecase.E(fwusecase.CodeInternal, "failed to save geo data file", err)
	}

	if err := validateDownloadedGeoXDB(payload, tmpPath); err != nil {
		return heavyTaskExecutionResult{}, err
	}

	if err := moveGeoXDBIntoPlace(tmpPath, targetPath); err != nil {
		return heavyTaskExecutionResult{}, fwusecase.E(fwusecase.CodeInternal, "failed to install geo data file", err)
	}
	removeTmp = false

	result := geoXDBDownloadResult{
		Version:  payload.Version,
		Target:   payload.TargetPath,
		Size:     size,
		Source:   sourceURL,
		Finished: timefmt.RFC3339Nano(timefmt.NowUTC()),
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return heavyTaskExecutionResult{}, fwusecase.E(fwusecase.CodeInternal, "failed to encode geo download result", err)
	}
	return heavyTaskExecutionResult{
		ResultJSON: string(encoded),
		Message:    fmt.Sprintf("Geo %s xdb download completed", payload.Version),
	}, nil
}

func parseGeoXDBDownloadPayload(value string) (geoXDBDownloadPayload, error) {
	var payload geoXDBDownloadPayload
	if err := json.Unmarshal([]byte(strings.TrimSpace(value)), &payload); err != nil {
		return geoXDBDownloadPayload{}, fwusecase.E(fwusecase.CodeInternal, "geo download task payload is invalid", err)
	}
	version, err := normalizeGeoXDBVersion(payload.Version)
	if err != nil {
		return geoXDBDownloadPayload{}, err
	}
	targetPath, err := normalizeGeoXDBSettingPath(payload.TargetPath)
	if err != nil {
		return geoXDBDownloadPayload{}, err
	}
	cachePolicy := strings.TrimSpace(payload.CachePolicy)
	if cachePolicy == "" {
		cachePolicy = defaultGeoCachePolicy
	}
	if !isValidGeoCachePolicy(cachePolicy) {
		return geoXDBDownloadPayload{}, fwusecase.E(fwusecase.CodeValidation, "ip2region cache policy is invalid", nil)
	}
	poolSize := payload.SearcherPoolSize
	if poolSize <= 0 {
		poolSize = defaultGeoSearcherPool
	}
	if poolSize < minGeoSearcherPool || poolSize > maxGeoSearcherPool {
		return geoXDBDownloadPayload{}, fwusecase.E(fwusecase.CodeValidation, fmt.Sprintf("ip2region searcher pool size must be between %d and %d", minGeoSearcherPool, maxGeoSearcherPool), nil)
	}

	payload.Version = version
	payload.TargetPath = targetPath
	payload.CachePolicy = cachePolicy
	payload.SearcherPoolSize = poolSize
	return payload, nil
}

func downloadGeoXDB(ctx context.Context, sourceURL string, output *os.File) (int64, error) {
	client, _ := currentGeoXDBDownloader()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return 0, fwusecase.E(fwusecase.CodeInternal, "failed to prepare geo data download", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, fwusecase.E(fwusecase.CodeInternal, "failed to download geo data", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return 0, fwusecase.E(fwusecase.CodeInternal, "failed to download geo data", fmt.Errorf("unexpected status %d", resp.StatusCode))
	}

	size, err := io.Copy(output, resp.Body)
	if err != nil {
		return 0, fwusecase.E(fwusecase.CodeInternal, "failed to save geo data file", err)
	}
	if size <= 0 {
		return 0, fwusecase.E(fwusecase.CodeInternal, "downloaded geo data file is empty", nil)
	}
	return size, nil
}

func validateDownloadedGeoXDB(payload geoXDBDownloadPayload, tmpPath string) error {
	if err := validateGeoIP2RegionXDB(GeoIP2RegionXDBValidatorConfig{
		Version:     payload.Version,
		XDBPath:     tmpPath,
		CachePolicy: payload.CachePolicy,
	}); err != nil {
		return fwusecase.E(fwusecase.CodeInternal, "downloaded geo data file is invalid", err)
	}
	return nil
}

func moveGeoXDBIntoPlace(tmpPath string, targetPath string) error {
	if err := os.Rename(tmpPath, targetPath); err == nil {
		return nil
	}

	backupPath := targetPath + ".bak-" + timefmt.NowUTC().Format("20060102150405.000000000")
	if _, err := os.Stat(targetPath); err == nil {
		if err := os.Rename(targetPath, backupPath); err != nil {
			return err
		}
		defer os.Remove(backupPath)
		if err := os.Rename(tmpPath, targetPath); err != nil {
			_ = os.Rename(backupPath, targetPath)
			return err
		}
		return nil
	}

	return os.Rename(tmpPath, targetPath)
}

func validateGeoXDBFiles(settings geoSettingsJSON) error {
	config, err := geoResolverConfigFromSettings(settings)
	if err != nil {
		return err
	}
	if err := ensureGeoXDBFile(config.V4XDBPath); err != nil {
		return err
	}
	if err := ensureGeoXDBFile(config.V6XDBPath); err != nil {
		return err
	}

	if err := validateGeoIP2RegionXDB(GeoIP2RegionXDBValidatorConfig{
		Version:     GeoXDBVersionV4,
		XDBPath:     config.V4XDBPath,
		CachePolicy: config.CachePolicy,
	}); err != nil {
		return err
	}
	if err := validateGeoIP2RegionXDB(GeoIP2RegionXDBValidatorConfig{
		Version:     GeoXDBVersionV6,
		XDBPath:     config.V6XDBPath,
		CachePolicy: config.CachePolicy,
	}); err != nil {
		return err
	}
	return nil
}

func ensureGeoXDBFile(filename string) error {
	info, err := os.Stat(filename)
	if err != nil {
		return err
	}
	if info.IsDir() || info.Size() <= 0 {
		return fmt.Errorf("invalid xdb file")
	}
	return nil
}

func geoResolverConfigFromSettings(settings geoSettingsJSON) (GeoIP2RegionResolverConfig, error) {
	v4Path, err := geoXDBRuntimePath(settings.V4XDB)
	if err != nil {
		return GeoIP2RegionResolverConfig{}, err
	}
	v6Path, err := geoXDBRuntimePath(settings.V6XDB)
	if err != nil {
		return GeoIP2RegionResolverConfig{}, err
	}
	cachePolicy := strings.TrimSpace(settings.CachePolicy)
	if cachePolicy == "" {
		cachePolicy = defaultGeoCachePolicy
	}
	if !isValidGeoCachePolicy(cachePolicy) {
		return GeoIP2RegionResolverConfig{}, fmt.Errorf("invalid cache policy")
	}
	poolSize := settings.SearcherPoolSize
	if poolSize <= 0 {
		poolSize = defaultGeoSearcherPool
	}
	return GeoIP2RegionResolverConfig{
		V4XDBPath:       v4Path,
		V6XDBPath:       v6Path,
		CachePolicy:     cachePolicy,
		PoolSize:        poolSize,
		SettingsVersion: geoResolverSettingsVersion(settings, v4Path, v6Path),
	}, nil
}

func newGeoIP2RegionResolver(config GeoIP2RegionResolverConfig) (RegistrationGeoResolver, func(), error) {
	geoResolverFactoryMu.RLock()
	factory := geoResolverFactory
	geoResolverFactoryMu.RUnlock()
	if factory == nil {
		return nil, nil, fmt.Errorf("ip2region resolver factory is not configured")
	}
	return factory(config)
}

func validateGeoIP2RegionXDB(config GeoIP2RegionXDBValidatorConfig) error {
	geoResolverFactoryMu.RLock()
	validator := geoXDBValidator
	geoResolverFactoryMu.RUnlock()
	if validator == nil {
		return fmt.Errorf("ip2region xdb validator is not configured")
	}
	return validator(config)
}

func normalizeGeoSettingsForSave(cmd SaveGeoSettingsCmd, previous geoSettingsJSON, found bool) (geoSettingsJSON, error) {
	v4Path, err := normalizeGeoXDBSettingPath(cmd.V4XDB)
	if err != nil {
		return geoSettingsJSON{}, err
	}
	v6Path, err := normalizeGeoXDBSettingPath(cmd.V6XDB)
	if err != nil {
		return geoSettingsJSON{}, err
	}
	cachePolicy := strings.TrimSpace(cmd.CachePolicy)
	if !isValidGeoCachePolicy(cachePolicy) {
		return geoSettingsJSON{}, fwusecase.E(fwusecase.CodeValidation, "ip2region cache policy is invalid", nil)
	}
	if cmd.SearcherPoolSize < minGeoSearcherPool || cmd.SearcherPoolSize > maxGeoSearcherPool {
		return geoSettingsJSON{}, fwusecase.E(fwusecase.CodeValidation, fmt.Sprintf("ip2region searcher pool size must be between %d and %d", minGeoSearcherPool, maxGeoSearcherPool), nil)
	}

	settings := geoSettingsJSON{
		Enabled:          cmd.Enabled,
		V4XDB:            v4Path,
		V6XDB:            v6Path,
		CachePolicy:      cachePolicy,
		SearcherPoolSize: cmd.SearcherPoolSize,
		UpdatedAt:        timefmt.RFC3339Nano(timefmt.NowUTC()),
	}
	if found && geoSettingsCheckKey(previous) == geoSettingsCheckKey(settings) {
		settings.XDBChecked = previous.XDBChecked
	}
	return settings, nil
}

func loadGeoSettings(ctx fwusecase.Context) (geoSettingsJSON, bool, error) {
	setting, err := models.GetAppSetting(ctx.Std(), geoIP2RegionSettingKey)
	if err != nil {
		if errors.Is(err, modelerror.ErrNotFound) {
			return defaultGeoSettings(), false, nil
		}
		return geoSettingsJSON{}, false, fwusecase.E(fwusecase.CodeInternal, "failed to load geo settings", err)
	}

	var stored geoSettingsJSON
	if err := json.Unmarshal([]byte(setting.ValueJSON), &stored); err != nil {
		return geoSettingsJSON{}, false, fwusecase.E(fwusecase.CodeInternal, "failed to parse geo settings", err)
	}
	stored = withGeoSettingsDefaults(stored)
	if stored.UpdatedAt == "" {
		stored.UpdatedAt = setting.UpdatedAt
	}
	return stored, true, nil
}

func saveGeoSettings(ctx fwusecase.Context, settings geoSettingsJSON) error {
	settings = withGeoSettingsDefaults(settings)
	encoded, err := json.Marshal(settings)
	if err != nil {
		return fwusecase.E(fwusecase.CodeInternal, "failed to encode geo settings", err)
	}
	if _, err := models.UpsertAppSetting(ctx.Std(), geoIP2RegionSettingKey, string(encoded)); err != nil {
		return fwusecase.E(fwusecase.CodeInternal, "failed to save geo settings", err)
	}
	return nil
}

func withGeoSettingsDefaults(settings geoSettingsJSON) geoSettingsJSON {
	settings.V4XDB = firstNonEmpty(settings.V4XDB, defaultGeoV4XDBPath)
	settings.V6XDB = firstNonEmpty(settings.V6XDB, defaultGeoV6XDBPath)
	settings.CachePolicy = firstNonEmpty(settings.CachePolicy, defaultGeoCachePolicy)
	if settings.SearcherPoolSize <= 0 {
		settings.SearcherPoolSize = defaultGeoSearcherPool
	}
	return settings
}

func defaultGeoSettings() geoSettingsJSON {
	return geoSettingsJSON{
		Enabled:          true,
		V4XDB:            defaultGeoV4XDBPath,
		V6XDB:            defaultGeoV6XDBPath,
		CachePolicy:      defaultGeoCachePolicy,
		SearcherPoolSize: defaultGeoSearcherPool,
		XDBChecked:       false,
	}
}

func geoSettingsFromStored(settings geoSettingsJSON) GeoSettingsCo {
	settings = withGeoSettingsDefaults(settings)
	return GeoSettingsCo{
		Enabled:          settings.Enabled,
		V4XDB:            settings.V4XDB,
		V6XDB:            settings.V6XDB,
		CachePolicy:      settings.CachePolicy,
		SearcherPoolSize: settings.SearcherPoolSize,
		XDBChecked:       settings.XDBChecked,
		UpdatedAt:        settings.UpdatedAt,
	}
}

func geoSettingsCheckKey(settings geoSettingsJSON) string {
	settings = withGeoSettingsDefaults(settings)
	return strings.Join([]string{
		settings.V4XDB,
		settings.V6XDB,
		settings.CachePolicy,
		fmt.Sprintf("%d", settings.SearcherPoolSize),
	}, "\x00")
}

func geoResolverSignature(config GeoIP2RegionResolverConfig) string {
	return strings.Join([]string{
		config.V4XDBPath,
		config.V6XDBPath,
		config.CachePolicy,
		fmt.Sprintf("%d", config.PoolSize),
		config.SettingsVersion,
	}, "\x00")
}

func geoResolverSettingsVersion(settings geoSettingsJSON, v4Path string, v6Path string) string {
	return strings.Join([]string{
		settings.UpdatedAt,
		geoXDBFileVersion(v4Path),
		geoXDBFileVersion(v6Path),
	}, "\x00")
}

func geoXDBFileVersion(filename string) string {
	info, err := os.Stat(filename)
	if err != nil {
		return err.Error()
	}
	return fmt.Sprintf("%d:%d", info.Size(), info.ModTime().UTC().UnixNano())
}

func normalizeGeoXDBVersion(version string) (string, error) {
	switch strings.TrimSpace(strings.ToLower(version)) {
	case GeoXDBVersionV4:
		return GeoXDBVersionV4, nil
	case GeoXDBVersionV6:
		return GeoXDBVersionV6, nil
	default:
		return "", fwusecase.E(fwusecase.CodeValidation, "geo xdb version is invalid", nil)
	}
}

func normalizeGeoXDBSettingPath(value string) (string, error) {
	raw := strings.ReplaceAll(strings.TrimSpace(value), "\\", "/")
	if raw == "" {
		return "", fwusecase.E(fwusecase.CodeValidation, "ip2region xdb path is required", nil)
	}
	clean := path.Clean(raw)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || strings.HasPrefix(clean, "/") || strings.Contains(clean, ":") {
		return "", fwusecase.E(fwusecase.CodeValidation, "ip2region xdb path is invalid", nil)
	}
	if !strings.HasSuffix(strings.ToLower(clean), ".xdb") {
		return "", fwusecase.E(fwusecase.CodeValidation, "ip2region xdb path must end with .xdb", nil)
	}
	return clean, nil
}

func geoXDBRuntimePath(settingPath string) (string, error) {
	clean, err := normalizeGeoXDBSettingPath(settingPath)
	if err != nil {
		return "", err
	}
	runtimeRel := clean
	if clean != "data" && !strings.HasPrefix(clean, "data/") {
		runtimeRel = path.Join("data", clean)
	}

	absData, err := filepath.Abs("data")
	if err != nil {
		return "", fwusecase.E(fwusecase.CodeInternal, "failed to resolve geo data directory", err)
	}
	absTarget, err := filepath.Abs(filepath.FromSlash(runtimeRel))
	if err != nil {
		return "", fwusecase.E(fwusecase.CodeInternal, "failed to resolve geo data file", err)
	}
	rel, err := filepath.Rel(absData, absTarget)
	if err != nil {
		return "", fwusecase.E(fwusecase.CodeInternal, "failed to resolve geo data file", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fwusecase.E(fwusecase.CodeValidation, "ip2region xdb path is invalid", nil)
	}
	return absTarget, nil
}

func isValidGeoCachePolicy(value string) bool {
	switch strings.TrimSpace(value) {
	case "file", "vectorIndex", "content":
		return true
	default:
		return false
	}
}

func geoXDBDownloadSource(version string) (string, error) {
	_, sources := currentGeoXDBDownloader()
	sourceURL := strings.TrimSpace(sources[version])
	if sourceURL == "" {
		return "", fwusecase.E(fwusecase.CodeInternal, "geo xdb download source is not configured", nil)
	}
	return sourceURL, nil
}

func currentGeoXDBDownloader() (*http.Client, map[string]string) {
	geoXDBDownloaderMu.RLock()
	defer geoXDBDownloaderMu.RUnlock()
	return geoXDBDownloader.client, cloneGeoXDBSources(geoXDBDownloader.sources)
}

func cloneGeoXDBSources(sources map[string]string) map[string]string {
	cloned := make(map[string]string, len(sources))
	for key, value := range sources {
		cloned[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	return cloned
}
