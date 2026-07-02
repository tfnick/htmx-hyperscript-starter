package usecase_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tfnick/go-svelte-starter/api/db"
	"github.com/tfnick/go-svelte-starter/api/framework/cache"
	"github.com/tfnick/go-svelte-starter/api/framework/queue"
	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
	"github.com/tfnick/go-svelte-starter/api/models"
	"github.com/tfnick/go-svelte-starter/api/usecase"
	"github.com/tfnick/go-svelte-starter/api/usecase/integrations/oss"
)

type siteLogoFakeOSSAdapter struct {
	putKey         string
	putConfig      oss.ProviderConfig
	putContentType string
	getConfig      oss.ProviderConfig
	objects        map[string][]byte
}

type fakeGeoResolverFactory struct {
	geo        usecase.RegistrationGeo
	err        error
	calls      int
	closeCalls int
	configs    []usecase.GeoIP2RegionResolverConfig
}

type fakeGeoXDBValidator struct {
	err     error
	calls   int
	configs []usecase.GeoIP2RegionXDBValidatorConfig
}

func (f *fakeGeoResolverFactory) factory(config usecase.GeoIP2RegionResolverConfig) (usecase.RegistrationGeoResolver, func(), error) {
	f.calls++
	f.configs = append(f.configs, config)
	for _, filename := range []string{config.V4XDBPath, config.V6XDBPath} {
		if strings.TrimSpace(filename) == "" {
			continue
		}
		info, err := os.Stat(filename)
		if err != nil {
			return nil, nil, err
		}
		if info.Size() == 0 {
			return nil, nil, assertErr("empty xdb")
		}
	}
	if f.err != nil {
		return nil, nil, f.err
	}
	return fakeRegistrationGeoResolver{geo: f.geo}, func() {
		f.closeCalls++
	}, nil
}

func (f *fakeGeoXDBValidator) validate(config usecase.GeoIP2RegionXDBValidatorConfig) error {
	f.calls++
	f.configs = append(f.configs, config)
	if strings.TrimSpace(config.Version) == "" {
		return assertErr("missing xdb version")
	}
	info, err := os.Stat(config.XDBPath)
	if err != nil {
		return err
	}
	if info.Size() == 0 {
		return assertErr("empty xdb")
	}
	if f.err != nil {
		return f.err
	}
	return nil
}

func (a *siteLogoFakeOSSAdapter) PutObject(_ context.Context, cfg oss.ProviderConfig, req oss.PutObjectRequest) (oss.PutObjectResult, error) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return oss.PutObjectResult{}, err
	}
	if a.objects == nil {
		a.objects = map[string][]byte{}
	}
	a.putKey = req.Key
	a.putConfig = cfg
	a.putContentType = req.ContentType
	a.objects[req.Key] = body
	return oss.PutObjectResult{Key: req.Key, Size: int64(len(body))}, nil
}

func (a *siteLogoFakeOSSAdapter) GetObject(_ context.Context, cfg oss.ProviderConfig, req oss.GetObjectRequest) (oss.GetObjectResult, error) {
	a.getConfig = cfg
	body := a.objects[req.Key]
	return oss.GetObjectResult{
		Key:         req.Key,
		Body:        io.NopCloser(bytes.NewReader(body)),
		ContentType: a.putContentType,
		Size:        int64(len(body)),
	}, nil
}

func (a *siteLogoFakeOSSAdapter) DeleteObject(context.Context, oss.ProviderConfig, oss.DeleteObjectRequest) (oss.DeleteObjectResult, error) {
	return oss.DeleteObjectResult{}, nil
}

func (a *siteLogoFakeOSSAdapter) PresignObject(context.Context, oss.ProviderConfig, oss.PresignObjectRequest) (oss.PresignObjectResult, error) {
	return oss.PresignObjectResult{}, nil
}

func TestGetSiteSettingsDefaultsToPublicLogo(t *testing.T) {
	setupUsecaseOrderTxDB(t)

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	settings, err := usecase.GetSiteSettings(ctx, usecase.SiteSettingsQry{})
	if err != nil {
		t.Fatalf("get site settings: %v", err)
	}
	if settings.LogoURL != "/logo.png" || settings.LogoConfigured {
		t.Fatalf("expected default logo settings, got %#v", settings)
	}
	if settings.LogoUploadAvailable || settings.LogoUploadUnavailableReason == "" {
		t.Fatalf("expected logo upload to be unavailable without primary OSS, got %#v", settings)
	}
}

func TestGetClaritySettingsDefaultsDisabled(t *testing.T) {
	setupUsecaseOrderTxDB(t)

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	settings, err := usecase.GetClaritySettings(ctx, usecase.ClaritySettingsQry{})
	if err != nil {
		t.Fatalf("get Clarity settings: %v", err)
	}
	if settings.Enabled || settings.ProjectID != "" || settings.UpdatedAt != "" {
		t.Fatalf("expected default disabled Clarity settings, got %#v", settings)
	}

	script, err := usecase.CachedClarityScriptWithError(ctx)
	if err != nil {
		t.Fatalf("get cached Clarity script: %v", err)
	}
	if script != "" {
		t.Fatalf("expected empty script by default, got %q", script)
	}
}

func TestSaveClaritySettingsPersistsAndRefreshesCache(t *testing.T) {
	setupUsecaseOrderTxDB(t)

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	settings, err := usecase.SaveClaritySettings(ctx, usecase.SaveClaritySettingsCmd{
		Enabled:   true,
		ProjectID: "abc123_X-y",
	})
	if err != nil {
		t.Fatalf("save Clarity settings: %v", err)
	}
	if !settings.Enabled || settings.ProjectID != "abc123_X-y" || settings.UpdatedAt == "" {
		t.Fatalf("unexpected saved Clarity settings: %#v", settings)
	}

	script, err := usecase.CachedClarityScriptWithError(ctx)
	if err != nil {
		t.Fatalf("get cached Clarity script: %v", err)
	}
	if !strings.Contains(script, `https://www.clarity.ms/tag/`) || !strings.Contains(script, `"abc123_X-y"`) {
		t.Fatalf("expected Clarity script with project ID, got %s", script)
	}
	if strings.Contains(script, "<script>alert") {
		t.Fatalf("script should be backend generated, got %s", script)
	}
}

func TestSaveClaritySettingsRejectsInvalidProjectID(t *testing.T) {
	setupUsecaseOrderTxDB(t)

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	_, err := usecase.SaveClaritySettings(ctx, usecase.SaveClaritySettingsCmd{
		Enabled:   true,
		ProjectID: `bad"><script>`,
	})
	if err == nil {
		t.Fatalf("expected invalid project ID error")
	}
	if fwusecase.CodeOf(err) != fwusecase.CodeValidation {
		t.Fatalf("expected validation error code, got %q: %v", fwusecase.CodeOf(err), err)
	}
}

func TestClarityCacheMissFallsBackToDBAndRefreshesCache(t *testing.T) {
	setupUsecaseOrderTxDB(t)

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	scriptVal := `<script type="text/javascript">(function(c,l,a,r,i,t,y){c[a]=c[a]||function(){(c[a].q=c[a].q||[]).push(arguments)};t=l.createElement(r);t.async=1;t.src="https://www.clarity.ms/tag/"+i;y=l.getElementsByTagName(r)[0];y.parentNode.insertBefore(t,y);})(window, document, "clarity", "script", "from_db");</script>`
	if _, err := models.UpsertAppSetting(t.Context(), "site.clarity_script", scriptVal); err != nil {
		t.Fatalf("seed Clarity script: %v", err)
	}
	_ = cache.DefaultRistrettoStore.Delete(t.Context(), "app_setting", "site.clarity_script")

	script, err := usecase.CachedClarityScriptWithError(ctx)
	if err != nil {
		t.Fatalf("get cached Clarity script: %v", err)
	}
	if !strings.Contains(script, `from_db`) {
		t.Fatalf("expected DB-backed Clarity script, got %s", script)
	}
}

func TestDisabledClarityCachesEmptyScriptAndKeepsProjectID(t *testing.T) {
	setupUsecaseOrderTxDB(t)

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	settings, err := usecase.SaveClaritySettings(ctx, usecase.SaveClaritySettingsCmd{
		Enabled:   false,
		ProjectID: "saved_for_later",
	})
	if err != nil {
		t.Fatalf("save disabled Clarity settings: %v", err)
	}
	if settings.Enabled || settings.ProjectID != "saved_for_later" {
		t.Fatalf("expected disabled settings with retained project ID, got %#v", settings)
	}

	script, err := usecase.CachedClarityScriptWithError(ctx)
	if err != nil {
		t.Fatalf("get cached Clarity script: %v", err)
	}
	if script != "" {
		t.Fatalf("expected disabled Clarity to cache empty script, got %q", script)
	}
}

func TestGetGeoSettingsDefaultsEnabledUnchecked(t *testing.T) {
	setupUsecaseOrderTxDB(t)

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	settings, err := usecase.GetGeoSettings(ctx, usecase.GeoSettingsQry{})
	if err != nil {
		t.Fatalf("get Geo settings: %v", err)
	}
	if !settings.Enabled || settings.V4XDB != "geo/ip2region_v4.xdb" || settings.V6XDB != "geo/ip2region_v6.xdb" {
		t.Fatalf("unexpected default Geo settings: %#v", settings)
	}
	if settings.CachePolicy != "vectorIndex" || settings.SearcherPoolSize != 20 || settings.XDBChecked {
		t.Fatalf("unexpected default Geo settings: %#v", settings)
	}
}

func TestSaveGeoSettingsValidatesAndResetsCheckedOnDataConfigChange(t *testing.T) {
	setupUsecaseOrderTxDB(t)

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	saved, err := usecase.SaveGeoSettings(ctx, usecase.SaveGeoSettingsCmd{
		Enabled:          true,
		V4XDB:            "geo/v4.xdb",
		V6XDB:            "geo/v6.xdb",
		CachePolicy:      "vectorIndex",
		SearcherPoolSize: 20,
	})
	if err != nil {
		t.Fatalf("save Geo settings: %v", err)
	}
	if saved.XDBChecked {
		t.Fatalf("new settings should not be checked: %#v", saved)
	}

	validator := &fakeGeoXDBValidator{}
	restore := usecase.RegisterGeoIP2RegionXDBValidator(validator.validate)
	defer restore()
	writeGeoTestFile(t, "data/geo/v4.xdb", "v4")
	writeGeoTestFile(t, "data/geo/v6.xdb", "v6")
	checked, err := usecase.CheckGeoXDB(ctx, usecase.CheckGeoXDBCmd{})
	if err != nil {
		t.Fatalf("check Geo xdb: %v", err)
	}
	if !checked.Valid || !checked.Settings.XDBChecked {
		t.Fatalf("expected checked settings, got %#v", checked)
	}

	unchanged, err := usecase.SaveGeoSettings(ctx, usecase.SaveGeoSettingsCmd{
		Enabled:          false,
		V4XDB:            "geo/v4.xdb",
		V6XDB:            "geo/v6.xdb",
		CachePolicy:      "vectorIndex",
		SearcherPoolSize: 20,
	})
	if err != nil {
		t.Fatalf("save Geo enabled flag: %v", err)
	}
	if !unchanged.XDBChecked {
		t.Fatalf("enabled toggle should keep checked state: %#v", unchanged)
	}

	changed, err := usecase.SaveGeoSettings(ctx, usecase.SaveGeoSettingsCmd{
		Enabled:          false,
		V4XDB:            "geo/other-v4.xdb",
		V6XDB:            "geo/v6.xdb",
		CachePolicy:      "vectorIndex",
		SearcherPoolSize: 20,
	})
	if err != nil {
		t.Fatalf("save changed Geo path: %v", err)
	}
	if changed.XDBChecked {
		t.Fatalf("path change should reset checked state: %#v", changed)
	}

	_, err = usecase.SaveGeoSettings(ctx, usecase.SaveGeoSettingsCmd{
		Enabled:          true,
		V4XDB:            "../geo/v4.xdb",
		V6XDB:            "geo/v6.xdb",
		CachePolicy:      "vectorIndex",
		SearcherPoolSize: 20,
	})
	if fwusecase.CodeOf(err) != fwusecase.CodeValidation {
		t.Fatalf("expected invalid path validation error, got %v", err)
	}

	_, err = usecase.SaveGeoSettings(ctx, usecase.SaveGeoSettingsCmd{
		Enabled:          true,
		V4XDB:            "geo/v4.xdb",
		V6XDB:            "geo/v6.xdb",
		CachePolicy:      "invalid",
		SearcherPoolSize: 20,
	})
	if fwusecase.CodeOf(err) != fwusecase.CodeValidation {
		t.Fatalf("expected invalid cache policy validation error, got %v", err)
	}
}

func TestCheckGeoXDBSetsCheckedOnlyWhenBothFilesValidate(t *testing.T) {
	setupUsecaseOrderTxDB(t)
	validator := &fakeGeoXDBValidator{}
	restore := usecase.RegisterGeoIP2RegionXDBValidator(validator.validate)
	defer restore()

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	if _, err := usecase.SaveGeoSettings(ctx, usecase.SaveGeoSettingsCmd{
		Enabled:          true,
		V4XDB:            "geo/check-v4.xdb",
		V6XDB:            "geo/check-v6.xdb",
		CachePolicy:      "file",
		SearcherPoolSize: 2,
	}); err != nil {
		t.Fatalf("save Geo settings: %v", err)
	}

	failed, err := usecase.CheckGeoXDB(ctx, usecase.CheckGeoXDBCmd{})
	if err != nil {
		t.Fatalf("check missing Geo xdb: %v", err)
	}
	if failed.Valid || failed.Settings.XDBChecked {
		t.Fatalf("expected missing files to be invalid, got %#v", failed)
	}

	writeGeoTestFile(t, "data/geo/check-v4.xdb", "v4")
	writeGeoTestFile(t, "data/geo/check-v6.xdb", "v6")
	passed, err := usecase.CheckGeoXDB(ctx, usecase.CheckGeoXDBCmd{})
	if err != nil {
		t.Fatalf("check valid Geo xdb: %v", err)
	}
	if !passed.Valid || !passed.Settings.XDBChecked {
		t.Fatalf("expected files to validate, got %#v", passed)
	}
	if validator.calls != 2 {
		t.Fatalf("expected v4 and v6 xdb validation for successful check, got %d", validator.calls)
	}
	if len(validator.configs) != 2 || !strings.HasSuffix(filepath.ToSlash(validator.configs[0].XDBPath), "data/geo/check-v4.xdb") {
		t.Fatalf("expected data-relative runtime path, got %#v", validator.configs)
	}
}

func TestSettingsBackedRegistrationGeoResolverUsesCheckedSettings(t *testing.T) {
	setupUsecaseOrderTxDB(t)
	factory := &fakeGeoResolverFactory{
		geo: usecase.RegistrationGeo{Country: "China", Region: "Shanghai"},
	}
	restore := usecase.RegisterGeoIP2RegionResolverFactory(factory.factory)
	defer restore()
	validator := &fakeGeoXDBValidator{}
	restoreValidator := usecase.RegisterGeoIP2RegionXDBValidator(validator.validate)
	defer restoreValidator()
	writeGeoTestFile(t, "data/geo/runtime-v4.xdb", "v4")
	writeGeoTestFile(t, "data/geo/runtime-v6.xdb", "v6")

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	if _, err := usecase.SaveGeoSettings(ctx, usecase.SaveGeoSettingsCmd{
		Enabled:          true,
		V4XDB:            "geo/runtime-v4.xdb",
		V6XDB:            "geo/runtime-v6.xdb",
		CachePolicy:      "vectorIndex",
		SearcherPoolSize: 20,
	}); err != nil {
		t.Fatalf("save Geo settings: %v", err)
	}

	resolver := usecase.NewSettingsBackedRegistrationGeoResolver()
	defer resolver.Close()
	geo, err := resolver.ResolveRegistrationGeo(ctx, "127.0.0.1")
	if err != nil {
		t.Fatalf("unchecked resolver should not error: %v", err)
	}
	if geo.Country != "" || factory.calls != 0 {
		t.Fatalf("unchecked settings should not initialize resolver, geo=%#v calls=%d", geo, factory.calls)
	}

	checked, err := usecase.CheckGeoXDB(ctx, usecase.CheckGeoXDBCmd{})
	if err != nil {
		t.Fatalf("check Geo xdb: %v", err)
	}
	if !checked.Valid {
		t.Fatalf("expected check to pass: %#v", checked)
	}

	geo, err = resolver.ResolveRegistrationGeo(ctx, "127.0.0.1")
	if err != nil {
		t.Fatalf("resolve registration geo: %v", err)
	}
	if geo.Country != "China" || geo.Region != "Shanghai" {
		t.Fatalf("unexpected geo result: %#v", geo)
	}
	if factory.calls != 1 {
		t.Fatalf("expected one runtime resolver initialization, got %d", factory.calls)
	}
	if validator.calls != 2 {
		t.Fatalf("expected v4 and v6 xdb validation during check, got %d", validator.calls)
	}

	_, err = usecase.SaveGeoSettings(ctx, usecase.SaveGeoSettingsCmd{
		Enabled:          false,
		V4XDB:            "geo/runtime-v4.xdb",
		V6XDB:            "geo/runtime-v6.xdb",
		CachePolicy:      "vectorIndex",
		SearcherPoolSize: 20,
	})
	if err != nil {
		t.Fatalf("disable Geo settings: %v", err)
	}
	geo, err = resolver.ResolveRegistrationGeo(ctx, "127.0.0.1")
	if err != nil {
		t.Fatalf("disabled resolver should not error: %v", err)
	}
	if geo.Country != "" || factory.closeCalls == 0 {
		t.Fatalf("disabled settings should close cached resolver and return empty geo, geo=%#v close=%d", geo, factory.closeCalls)
	}
}

func TestSettingsBackedRegistrationGeoResolverReloadsAfterGeoCheckRefresh(t *testing.T) {
	setupUsecaseOrderTxDB(t)
	factory := &fakeGeoResolverFactory{
		geo: usecase.RegistrationGeo{Country: "China", Region: "Shanghai"},
	}
	restore := usecase.RegisterGeoIP2RegionResolverFactory(factory.factory)
	defer restore()
	validator := &fakeGeoXDBValidator{}
	restoreValidator := usecase.RegisterGeoIP2RegionXDBValidator(validator.validate)
	defer restoreValidator()
	writeGeoTestFile(t, "data/geo/reload-v4.xdb", "v4")
	writeGeoTestFile(t, "data/geo/reload-v6.xdb", "v6")

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	if _, err := usecase.SaveGeoSettings(ctx, usecase.SaveGeoSettingsCmd{
		Enabled:          true,
		V4XDB:            "geo/reload-v4.xdb",
		V6XDB:            "geo/reload-v6.xdb",
		CachePolicy:      "vectorIndex",
		SearcherPoolSize: 20,
	}); err != nil {
		t.Fatalf("save Geo settings: %v", err)
	}
	if checked, err := usecase.CheckGeoXDB(ctx, usecase.CheckGeoXDBCmd{}); err != nil || !checked.Valid {
		t.Fatalf("initial check Geo xdb: checked=%#v err=%v", checked, err)
	}

	resolver := usecase.NewSettingsBackedRegistrationGeoResolver()
	defer resolver.Close()
	if _, err := resolver.ResolveRegistrationGeo(ctx, "127.0.0.1"); err != nil {
		t.Fatalf("resolve checked geo: %v", err)
	}
	if factory.calls != 1 {
		t.Fatalf("expected one runtime resolver initialization, got %d", factory.calls)
	}

	rewriteGeoTestFile(t, "data/geo/reload-v4.xdb", "v4-refreshed", time.Now().Add(time.Minute))
	if _, err := resolver.ResolveRegistrationGeo(ctx, "127.0.0.1"); err != nil {
		t.Fatalf("resolve refreshed geo: %v", err)
	}
	if factory.calls != 2 {
		t.Fatalf("expected refreshed settings to rebuild cached resolver, got %d calls", factory.calls)
	}
	if factory.closeCalls == 0 {
		t.Fatalf("expected cached resolver to close before rebuild")
	}
}

func TestEnqueueGeoXDBDownloadStoresTaskAndResetsChecked(t *testing.T) {
	setupUsecaseOrderTxDB(t)
	queueManager, err := queue.NewManager()
	if err != nil {
		t.Fatalf("new queue manager: %v", err)
	}
	previousQueue := usecase.DefaultQueueManager
	usecase.DefaultQueueManager = queueManager
	t.Cleanup(func() {
		usecase.DefaultQueueManager = previousQueue
	})

	ctx := authenticatedUsecaseContext(t.Context(), "admin-1", true)
	if _, err := usecase.SaveGeoSettings(ctx, usecase.SaveGeoSettingsCmd{
		Enabled:          true,
		V4XDB:            "geo/download-v4.xdb",
		V6XDB:            "geo/download-v6.xdb",
		CachePolicy:      "vectorIndex",
		SearcherPoolSize: 20,
	}); err != nil {
		t.Fatalf("save Geo settings: %v", err)
	}

	result, err := usecase.EnqueueGeoXDBDownload(ctx, usecase.EnqueueGeoXDBDownloadCmd{
		UserID:  "admin-1",
		Version: "v4",
	})
	if err != nil {
		t.Fatalf("enqueue Geo xdb download: %v", err)
	}
	if result.TaskID == "" || result.Version != "v4" {
		t.Fatalf("unexpected download enqueue result: %#v", result)
	}

	task, err := models.GetAsyncTaskByID(t.Context(), result.TaskID)
	if err != nil {
		t.Fatalf("get async task: %v", err)
	}
	if task.TaskType != usecase.HeavyTaskTypeGeoXDBDownload || task.MessageID == "" {
		t.Fatalf("unexpected async task: %#v", task)
	}

	appDB, err := db.DefaultManager.GetDB("app")
	if err != nil {
		t.Fatalf("get app db: %v", err)
	}
	var body string
	if err := appDB.Get(&body, `SELECT CAST(body AS TEXT) FROM goqite WHERE id = ? AND queue = ?`, task.MessageID, queue.QueueHeavyTasks); err != nil {
		t.Fatalf("get queue message: %v", err)
	}
	if !strings.Contains(body, `"task_type":"geo_xdb_download"`) {
		t.Fatalf("expected geo task message, got %s", body)
	}
}

func TestGeoXDBDownloadTaskDownloadsValidatesAndMovesFile(t *testing.T) {
	setupUsecaseOrderTxDB(t)
	validator := &fakeGeoXDBValidator{}
	restoreFactory := usecase.RegisterGeoIP2RegionXDBValidator(validator.validate)
	defer restoreFactory()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("fake-xdb-content"))
	}))
	defer server.Close()
	restoreDownloader := usecase.ConfigureGeoXDBDownloader(server.Client(), map[string]string{
		"v4": server.URL,
	})
	defer restoreDownloader()

	task := &models.AsyncTask{
		ID:          "geo-download-task",
		UserID:      "admin-1",
		TaskType:    usecase.HeavyTaskTypeGeoXDBDownload,
		Status:      models.AsyncTaskStatusQueued,
		PayloadJSON: `{"version":"v4","target_path":"geo/task-v4.xdb","cache_policy":"vectorIndex","searcher_pool_size":20}`,
		ResultJSON:  "{}",
	}
	if err := models.InsertAsyncTask(t.Context(), task); err != nil {
		t.Fatalf("insert async task: %v", err)
	}
	message := usecase.HeavyTaskMessage{
		TaskID:   task.ID,
		TaskType: task.TaskType,
		UserID:   task.UserID,
	}
	encoded, err := json.Marshal(message)
	if err != nil {
		t.Fatalf("encode message: %v", err)
	}
	if err := usecase.HandleHeavyTaskMessage(t.Context(), encoded); err != nil {
		t.Fatalf("handle geo download task: %v", err)
	}

	installed := filepath.Join("data", "geo", "task-v4.xdb")
	t.Cleanup(func() {
		_ = os.Remove(installed)
	})
	got, err := os.ReadFile(installed)
	if err != nil {
		t.Fatalf("read installed xdb: %v", err)
	}
	if string(got) != "fake-xdb-content" {
		t.Fatalf("unexpected installed xdb content: %q", got)
	}
	if validator.calls != 1 {
		t.Fatalf("expected downloaded file validation, got validator calls %d", validator.calls)
	}
	if len(validator.configs) != 1 {
		t.Fatalf("unexpected downloaded file validation config: %#v", validator.configs)
	}
	validatorPath := filepath.ToSlash(validator.configs[0].XDBPath)
	if validator.configs[0].Version != "v4" || !strings.Contains(validatorPath, "data/geo/.task-v4.xdb-") || !strings.HasSuffix(validatorPath, ".tmp") {
		t.Fatalf("unexpected downloaded file validation config: %#v", validator.configs)
	}
	updated, err := models.GetAsyncTaskByID(t.Context(), task.ID)
	if err != nil {
		t.Fatalf("get updated task: %v", err)
	}
	if updated.Status != models.AsyncTaskStatusCompleted || !strings.Contains(updated.ResultJSON, `"version":"v4"`) {
		t.Fatalf("unexpected task result: %#v", updated)
	}
}

func TestSaveSiteLogoRequiresPrimaryOSSProvider(t *testing.T) {
	setupUsecaseOrderTxDB(t)

	logo := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0, 0, 0, 0}
	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	_, err := usecase.SaveSiteLogo(ctx, usecase.SaveSiteLogoCmd{
		Filename:    "logo.png",
		ContentType: "image/png",
		Size:        int64(len(logo)),
		Body:        bytes.NewReader(logo),
	})
	if err == nil {
		t.Fatalf("expected missing primary OSS provider error")
	}
	if fwusecase.CodeOf(err) != fwusecase.CodeValidation {
		t.Fatalf("expected validation error code, got %q: %v", fwusecase.CodeOf(err), err)
	}
}

func writeGeoTestFile(t *testing.T, name string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0755); err != nil {
		t.Fatalf("create geo test dir: %v", err)
	}
	if err := os.WriteFile(name, []byte(content), 0644); err != nil {
		t.Fatalf("write geo test file %s: %v", name, err)
	}
	t.Cleanup(func() {
		_ = os.Remove(name)
	})
}

func rewriteGeoTestFile(t *testing.T, name string, content string, modTime time.Time) {
	t.Helper()
	if err := os.WriteFile(name, []byte(content), 0644); err != nil {
		t.Fatalf("rewrite geo test file %s: %v", name, err)
	}
	if err := os.Chtimes(name, modTime, modTime); err != nil {
		t.Fatalf("touch geo test file %s: %v", name, err)
	}
}

func TestSaveSiteLogoUsesOSSPortAndPersistsMetadata(t *testing.T) {
	setupUsecaseOrderTxDB(t)
	adapter := &siteLogoFakeOSSAdapter{}
	adapterKey := "oss.test.site_logo"
	if err := usecase.RegisterOSSAdapter(adapterKey, adapter); err != nil {
		t.Fatalf("register OSS adapter: %v", err)
	}
	seedPrimaryOSSChannel(t, adapterKey)

	logo := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0, 0, 0, 0}
	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	settings, err := usecase.SaveSiteLogo(ctx, usecase.SaveSiteLogoCmd{
		Filename:    "logo.png",
		ContentType: "image/png",
		Size:        int64(len(logo)),
		Body:        bytes.NewReader(logo),
	})
	if err != nil {
		t.Fatalf("save site logo: %v", err)
	}
	if adapter.putKey != "settings/site-logo.png" {
		t.Fatalf("expected logo object key, got %q", adapter.putKey)
	}
	if adapter.putContentType != "image/png" {
		t.Fatalf("expected image/png content type, got %q", adapter.putContentType)
	}
	if adapter.putConfig.ChannelCode != "site-logo-primary" || adapter.putConfig.EndpointURL != "https://r2.example.com" || adapter.putConfig.Bucket != "assets" {
		t.Fatalf("expected primary OSS provider config, got %#v", adapter.putConfig)
	}
	if adapter.putConfig.AccessKeyID != "ak-site-logo" || adapter.putConfig.SecretAccessKey != "sk-site-logo" {
		t.Fatalf("expected primary OSS credential, got %#v", adapter.putConfig)
	}
	if adapter.putConfig.UsePathStyle == nil || !*adapter.putConfig.UsePathStyle {
		t.Fatalf("expected primary OSS path-style config, got %#v", adapter.putConfig)
	}
	if !settings.LogoConfigured || !strings.HasPrefix(settings.LogoURL, "/api/public/settings/logo?v=") {
		t.Fatalf("expected configured logo URL, got %#v", settings)
	}
	if !settings.LogoUploadAvailable || settings.LogoUploadUnavailableReason != "" {
		t.Fatalf("expected logo upload to be available, got %#v", settings)
	}

	object, err := usecase.GetSiteLogoObject(ctx, usecase.SiteLogoObjectQry{})
	if err != nil {
		t.Fatalf("get site logo object: %v", err)
	}
	defer object.Body.Close()
	got, err := io.ReadAll(object.Body)
	if err != nil {
		t.Fatalf("read logo object: %v", err)
	}
	if !bytes.Equal(got, logo) {
		t.Fatalf("expected stored logo bytes, got %v", got)
	}
	if adapter.getConfig.ChannelCode != adapter.putConfig.ChannelCode || adapter.getConfig.AdapterKey != adapter.putConfig.AdapterKey {
		t.Fatalf("expected readback through persisted OSS provider metadata, got put=%#v get=%#v", adapter.putConfig, adapter.getConfig)
	}
}

func seedPrimaryOSSChannel(t *testing.T, adapterKey string) {
	t.Helper()

	credential, err := models.CreateIntegrationCredential(t.Context(), models.CreateIntegrationCredentialCmd{
		CredentialType: "s3_access_key",
		ValueText:      `{"access_key_id":"ak-site-logo","secret_access_key":"sk-site-logo"}`,
		Enabled:        true,
	})
	if err != nil {
		t.Fatalf("create OSS credential: %v", err)
	}
	if _, err := models.CreateIntegrationChannel(t.Context(), models.CreateIntegrationChannelCmd{
		Scenario:     models.IntegrationScenarioOSS,
		ChannelCode:  "site-logo-primary",
		ProviderCode: "cloudflare_r2",
		AdapterKey:   adapterKey,
		Environment:  "test",
		Enabled:      true,
		Priority:     1,
		CredentialID: credential.ID,
		IsPrimary:    true,
		ConfigJSON:   `{"endpoint_url":"https://r2.example.com","bucket":"assets","region":"auto","key_prefix":"public","use_path_style":true}`,
		MetadataJSON: "{}",
	}); err != nil {
		t.Fatalf("create primary OSS channel: %v", err)
	}
}
