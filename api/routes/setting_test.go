package routes_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	fwcontext "github.com/tfnick/go-svelte-starter/api/framework/http/context"
	"github.com/tfnick/go-svelte-starter/api/framework/queue"
	"github.com/tfnick/go-svelte-starter/api/models"
	"github.com/tfnick/go-svelte-starter/api/routes"
	"github.com/tfnick/go-svelte-starter/api/usecase"
	"github.com/tfnick/go-svelte-starter/api/usecase/integrations/oss"
)

type routeSiteLogoFakeOSSAdapter struct{}

func (routeSiteLogoFakeOSSAdapter) PutObject(_ context.Context, _ oss.ProviderConfig, req oss.PutObjectRequest) (oss.PutObjectResult, error) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return oss.PutObjectResult{}, err
	}
	return oss.PutObjectResult{Key: req.Key, Size: int64(len(body))}, nil
}

func (routeSiteLogoFakeOSSAdapter) GetObject(context.Context, oss.ProviderConfig, oss.GetObjectRequest) (oss.GetObjectResult, error) {
	return oss.GetObjectResult{Body: io.NopCloser(strings.NewReader(""))}, nil
}

func (routeSiteLogoFakeOSSAdapter) DeleteObject(context.Context, oss.ProviderConfig, oss.DeleteObjectRequest) (oss.DeleteObjectResult, error) {
	return oss.DeleteObjectResult{}, nil
}

func (routeSiteLogoFakeOSSAdapter) PresignObject(context.Context, oss.ProviderConfig, oss.PresignObjectRequest) (oss.PresignObjectResult, error) {
	return oss.PresignObjectResult{}, nil
}

func TestGetSiteSettingsReturnsDefaultLogoDTO(t *testing.T) {
	setupRouteTestDBs(t)

	router := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/settings/site", nil)
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)

	if err := routes.GetSiteSettings(c); err != nil {
		t.Fatalf("get site settings: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var envelope struct {
		Success bool                        `json:"success"`
		Data    routes.SiteSettingsResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !envelope.Success || envelope.Data.LogoURL != "/logo.png" || envelope.Data.LogoConfigured {
		t.Fatalf("unexpected settings response: %s", rec.Body.String())
	}
	if envelope.Data.LogoUploadAvailable || envelope.Data.LogoUploadUnavailableReason == "" {
		t.Fatalf("expected unavailable logo upload state: %s", rec.Body.String())
	}
}

func TestUploadSiteLogoReturnsConfiguredLogoDTO(t *testing.T) {
	setupRouteTestDBs(t)
	adapterKey := "oss.test.route.site_logo"
	if err := usecase.RegisterOSSAdapter(adapterKey, routeSiteLogoFakeOSSAdapter{}); err != nil {
		t.Fatalf("register OSS adapter: %v", err)
	}
	seedRoutePrimaryOSSChannel(t, adapterKey)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("logo", "logo.png")
	if err != nil {
		t.Fatalf("create multipart file: %v", err)
	}
	if _, err := part.Write([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0, 0, 0, 0}); err != nil {
		t.Fatalf("write multipart file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	router := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/settings/site/logo", &body)
	req.Header.Set(echo.HeaderContentType, writer.FormDataContentType())
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)

	if err := routes.UploadSiteLogo(c); err != nil {
		t.Fatalf("upload site logo: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var envelope struct {
		Success bool                        `json:"success"`
		Data    routes.SiteSettingsResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !envelope.Success || !envelope.Data.LogoConfigured || !strings.HasPrefix(envelope.Data.LogoURL, "/api/public/settings/logo?v=") {
		t.Fatalf("unexpected upload response: %s", rec.Body.String())
	}
	if !envelope.Data.LogoUploadAvailable || envelope.Data.LogoUploadUnavailableReason != "" {
		t.Fatalf("expected available logo upload state: %s", rec.Body.String())
	}
}

func TestUploadSiteLogoWithoutPrimaryOSSReturnsValidationError(t *testing.T) {
	setupRouteTestDBs(t)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("logo", "logo.png")
	if err != nil {
		t.Fatalf("create multipart file: %v", err)
	}
	if _, err := part.Write([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0, 0, 0, 0}); err != nil {
		t.Fatalf("write multipart file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	router := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/settings/site/logo", &body)
	req.Header.Set(echo.HeaderContentType, writer.FormDataContentType())
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)

	if err := routes.UploadSiteLogo(c); err != nil {
		t.Fatalf("upload site logo: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"primary OSS provider is not configured"`) {
		t.Fatalf("expected primary OSS validation message, got %s", rec.Body.String())
	}
}

func TestClaritySettingsRoutesReturnDTOAndValidation(t *testing.T) {
	setupRouteTestDBs(t)

	router := echo.New()
	saveReq := httptest.NewRequest(http.MethodPut, "/api/admin/settings/clarity", strings.NewReader(`{"enabled":true,"project_id":"route_123"}`))
	saveReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	saveRec := httptest.NewRecorder()
	saveCtx := router.NewContext(saveReq, saveRec)

	if err := routes.SaveClaritySettings(saveCtx); err != nil {
		t.Fatalf("save Clarity settings: %v", err)
	}
	if saveRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, saveRec.Code, saveRec.Body.String())
	}

	var saveEnvelope struct {
		Success bool                           `json:"success"`
		Data    routes.ClaritySettingsResponse `json:"data"`
	}
	if err := json.Unmarshal(saveRec.Body.Bytes(), &saveEnvelope); err != nil {
		t.Fatalf("decode save response: %v", err)
	}
	if !saveEnvelope.Success || !saveEnvelope.Data.Enabled || saveEnvelope.Data.ProjectID != "route_123" || saveEnvelope.Data.UpdatedAt == "" {
		t.Fatalf("unexpected save response: %s", saveRec.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/admin/settings/clarity", nil)
	getRec := httptest.NewRecorder()
	getCtx := router.NewContext(getReq, getRec)

	if err := routes.GetClaritySettings(getCtx); err != nil {
		t.Fatalf("get Clarity settings: %v", err)
	}
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, getRec.Code, getRec.Body.String())
	}
	var getEnvelope struct {
		Success bool                           `json:"success"`
		Data    routes.ClaritySettingsResponse `json:"data"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &getEnvelope); err != nil {
		t.Fatalf("decode get response: %v", err)
	}
	if !getEnvelope.Success || !getEnvelope.Data.Enabled || getEnvelope.Data.ProjectID != "route_123" {
		t.Fatalf("unexpected get response: %s", getRec.Body.String())
	}

	invalidReq := httptest.NewRequest(http.MethodPut, "/api/admin/settings/clarity", strings.NewReader(`{"enabled":true,"project_id":"bad<script>"}`))
	invalidReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	invalidRec := httptest.NewRecorder()
	invalidCtx := router.NewContext(invalidReq, invalidRec)

	if err := routes.SaveClaritySettings(invalidCtx); err != nil {
		t.Fatalf("save invalid Clarity settings: %v", err)
	}
	if invalidRec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusBadRequest, invalidRec.Code, invalidRec.Body.String())
	}
	if !strings.Contains(invalidRec.Body.String(), "Clarity project ID is invalid") {
		t.Fatalf("expected validation message, got %s", invalidRec.Body.String())
	}
}

func TestGeoSettingsRoutesReturnDTOAndValidation(t *testing.T) {
	setupRouteTestDBs(t)

	router := echo.New()
	saveReq := httptest.NewRequest(http.MethodPut, "/api/admin/settings/geo", strings.NewReader(`{"enabled":true,"v4_xdb":"geo/route-v4.xdb","v6_xdb":"geo/route-v6.xdb","cache_policy":"file","searcher_pool_size":3}`))
	saveReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	saveRec := httptest.NewRecorder()
	saveCtx := router.NewContext(saveReq, saveRec)

	if err := routes.SaveGeoSettings(saveCtx); err != nil {
		t.Fatalf("save Geo settings: %v", err)
	}
	if saveRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, saveRec.Code, saveRec.Body.String())
	}
	var saveEnvelope struct {
		Success bool                       `json:"success"`
		Data    routes.GeoSettingsResponse `json:"data"`
	}
	if err := json.Unmarshal(saveRec.Body.Bytes(), &saveEnvelope); err != nil {
		t.Fatalf("decode save response: %v", err)
	}
	if !saveEnvelope.Success || saveEnvelope.Data.V4XDB != "geo/route-v4.xdb" || saveEnvelope.Data.CachePolicy != "file" || saveEnvelope.Data.SearcherPoolSize != 3 {
		t.Fatalf("unexpected save response: %s", saveRec.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/admin/settings/geo", nil)
	getRec := httptest.NewRecorder()
	getCtx := router.NewContext(getReq, getRec)
	if err := routes.GetGeoSettings(getCtx); err != nil {
		t.Fatalf("get Geo settings: %v", err)
	}
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, getRec.Code, getRec.Body.String())
	}
	var getEnvelope struct {
		Success bool                       `json:"success"`
		Data    routes.GeoSettingsResponse `json:"data"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &getEnvelope); err != nil {
		t.Fatalf("decode get response: %v", err)
	}
	if !getEnvelope.Success || getEnvelope.Data.V6XDB != "geo/route-v6.xdb" {
		t.Fatalf("unexpected get response: %s", getRec.Body.String())
	}

	invalidReq := httptest.NewRequest(http.MethodPut, "/api/admin/settings/geo", strings.NewReader(`{"enabled":true,"v4_xdb":"geo/route-v4.xdb","v6_xdb":"geo/route-v6.xdb","cache_policy":"bad","searcher_pool_size":3}`))
	invalidReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	invalidRec := httptest.NewRecorder()
	invalidCtx := router.NewContext(invalidReq, invalidRec)
	if err := routes.SaveGeoSettings(invalidCtx); err != nil {
		t.Fatalf("save invalid Geo settings: %v", err)
	}
	if invalidRec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusBadRequest, invalidRec.Code, invalidRec.Body.String())
	}
	if !strings.Contains(invalidRec.Body.String(), "ip2region cache policy is invalid") {
		t.Fatalf("expected validation message, got %s", invalidRec.Body.String())
	}
}

func TestGeoSettingsDownloadRouteEnqueuesTask(t *testing.T) {
	setupRouteTestDBs(t)
	queueManager, err := queue.NewManager()
	if err != nil {
		t.Fatalf("new queue manager: %v", err)
	}
	previousQueue := usecase.DefaultQueueManager
	usecase.DefaultQueueManager = queueManager
	t.Cleanup(func() {
		usecase.DefaultQueueManager = previousQueue
	})

	router := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/settings/geo/download", strings.NewReader(`{"version":"v6"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)
	fwcontext.SetCurrentUser(c, &models.User{ID: "route-admin", Name: "Route Admin", Email: "route-admin@example.com", IsAdmin: 1, IsActive: 1})

	if err := routes.DownloadGeoXDB(c); err != nil {
		t.Fatalf("download Geo xdb: %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusCreated, rec.Code, rec.Body.String())
	}
	var envelope struct {
		Success bool                          `json:"success"`
		Data    routes.GeoXDBDownloadResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode download response: %v", err)
	}
	if !envelope.Success || envelope.Data.TaskID == "" || envelope.Data.Version != "v6" {
		t.Fatalf("unexpected download response: %s", rec.Body.String())
	}
}

func TestGeoSettingsCheckRouteReturnsSafeInvalidDTO(t *testing.T) {
	setupRouteTestDBs(t)

	router := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/settings/geo/check", nil)
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)

	if err := routes.CheckGeoXDB(c); err != nil {
		t.Fatalf("check Geo xdb: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}
	var envelope struct {
		Success bool                       `json:"success"`
		Data    routes.GeoXDBCheckResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode check response: %v", err)
	}
	if !envelope.Success || envelope.Data.Valid || envelope.Data.Settings.XDBChecked {
		t.Fatalf("expected invalid check response with unchecked settings, got %s", rec.Body.String())
	}
}

func seedRoutePrimaryOSSChannel(t *testing.T, adapterKey string) {
	t.Helper()

	credential, err := models.CreateIntegrationCredential(t.Context(), models.CreateIntegrationCredentialCmd{
		CredentialType: "s3_access_key",
		ValueText:      `{"access_key_id":"ak-route-logo","secret_access_key":"sk-route-logo"}`,
		Enabled:        true,
	})
	if err != nil {
		t.Fatalf("create OSS credential: %v", err)
	}
	if _, err := models.CreateIntegrationChannel(t.Context(), models.CreateIntegrationChannelCmd{
		Scenario:     models.IntegrationScenarioOSS,
		ChannelCode:  "route-site-logo-primary",
		ProviderCode: "cloudflare_r2",
		AdapterKey:   adapterKey,
		Environment:  "test",
		Enabled:      true,
		Priority:     1,
		CredentialID: credential.ID,
		IsPrimary:    true,
		ConfigJSON:   `{"endpoint_url":"https://r2.example.com","bucket":"assets","region":"auto"}`,
		MetadataJSON: "{}",
	}); err != nil {
		t.Fatalf("create primary OSS channel: %v", err)
	}
}
