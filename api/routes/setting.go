package routes

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	fwcontext "github.com/tfnick/go-svelte-starter/api/framework/http/context"
	"github.com/tfnick/go-svelte-starter/api/framework/http/middleware"
	httpresponse "github.com/tfnick/go-svelte-starter/api/framework/http/response"
	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
	"github.com/tfnick/go-svelte-starter/api/usecase"
)

type SiteSettingsResponse struct {
	LogoURL                     string `json:"logo_url"`
	LogoConfigured              bool   `json:"logo_configured"`
	LogoUpdatedAt               string `json:"logo_updated_at"`
	LogoUploadAvailable         bool   `json:"logo_upload_available"`
	LogoUploadUnavailableReason string `json:"logo_upload_unavailable_reason"`
}

type ClaritySettingsResponse struct {
	Enabled   bool   `json:"enabled"`
	ProjectID string `json:"project_id"`
	UpdatedAt string `json:"updated_at"`
}

type SaveClaritySettingsRequest struct {
	Enabled   bool   `json:"enabled"`
	ProjectID string `json:"project_id"`
}

type GeoSettingsResponse struct {
	Enabled          bool   `json:"enabled"`
	V4XDB            string `json:"v4_xdb"`
	V6XDB            string `json:"v6_xdb"`
	CachePolicy      string `json:"cache_policy"`
	SearcherPoolSize int    `json:"searcher_pool_size"`
	XDBChecked       bool   `json:"xdb_checked"`
	UpdatedAt        string `json:"updated_at"`
}

type SaveGeoSettingsRequest struct {
	Enabled          bool   `json:"enabled"`
	V4XDB            string `json:"v4_xdb"`
	V6XDB            string `json:"v6_xdb"`
	CachePolicy      string `json:"cache_policy"`
	SearcherPoolSize int    `json:"searcher_pool_size"`
}

type GeoXDBDownloadRequest struct {
	Version string `json:"version"`
}

type GeoXDBDownloadResponse struct {
	TaskID  string `json:"task_id"`
	Version string `json:"version"`
	Message string `json:"message"`
}

type GeoXDBCheckResponse struct {
	Valid    bool                `json:"valid"`
	Message  string              `json:"message"`
	Settings GeoSettingsResponse `json:"settings"`
}

func GetSiteSettings(c echo.Context) error {
	settings, err := usecase.GetSiteSettings(fwcontext.InternalUsecaseContext(c), usecase.SiteSettingsQry{})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.OK(c, toSiteSettingsResponse(settings))
}

func UploadSiteLogo(c echo.Context) error {
	fileHeader, err := c.FormFile("logo")
	if err != nil {
		if errors.Is(err, http.ErrMissingFile) {
			return httpresponse.InternalUsecaseError(c, fwusecase.E(fwusecase.CodeValidation, "logo file is required", err))
		}
		return httpresponse.InternalUsecaseError(c, fwusecase.E(fwusecase.CodeValidation, "logo file is required", err))
	}

	file, err := fileHeader.Open()
	if err != nil {
		return httpresponse.InternalUsecaseError(c, fwusecase.E(fwusecase.CodeInternal, "failed to read logo file", err))
	}
	defer file.Close()

	settings, err := usecase.SaveSiteLogo(fwcontext.InternalUsecaseContext(c), usecase.SaveSiteLogoCmd{
		Filename:    fileHeader.Filename,
		ContentType: fileHeader.Header.Get(echo.HeaderContentType),
		Size:        fileHeader.Size,
		Body:        file,
	})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.OK(c, toSiteSettingsResponse(settings))
}

func GetPublicSiteLogo(c echo.Context) error {
	logo, err := usecase.GetSiteLogoObject(fwcontext.InternalUsecaseContext(c), usecase.SiteLogoObjectQry{})
	if err != nil {
		if fwusecase.CodeOf(err) == fwusecase.CodeNotFound {
			return c.Redirect(http.StatusFound, "/logo.png")
		}
		return httpresponse.InternalUsecaseError(c, err)
	}
	defer logo.Body.Close()

	c.Response().Header().Set(echo.HeaderCacheControl, "public, max-age=300")
	return c.Stream(http.StatusOK, logo.ContentType, logo.Body)
}

type PageViewEnabledResponse struct {
	Enabled bool `json:"enabled"`
}

type SavePageViewEnabledRequest struct {
	Enabled bool `json:"enabled"`
}

func GetPageViewEnabled(c echo.Context) error {
	enabled, err := usecase.GetPageViewEnabled(fwcontext.InternalUsecaseContext(c))
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.OK(c, PageViewEnabledResponse{Enabled: enabled})
}

func SavePageViewEnabled(c echo.Context) error {
	var req SavePageViewEnabledRequest
	if err := c.Bind(&req); err != nil {
		return httpresponse.BadRequest(c, "invalid request data")
	}
	enabled, err := usecase.SavePageViewEnabled(fwcontext.InternalUsecaseContext(c), req.Enabled)
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.OK(c, PageViewEnabledResponse{Enabled: enabled})
}

func GetClaritySettings(c echo.Context) error {
	settings, err := usecase.GetClaritySettings(fwcontext.InternalUsecaseContext(c), usecase.ClaritySettingsQry{})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.OK(c, toClaritySettingsResponse(settings))
}

func SaveClaritySettings(c echo.Context) error {
	var req SaveClaritySettingsRequest
	if err := c.Bind(&req); err != nil {
		return httpresponse.BadRequest(c, "invalid request data")
	}

	settings, err := usecase.SaveClaritySettings(fwcontext.InternalUsecaseContext(c), usecase.SaveClaritySettingsCmd{
		Enabled:   req.Enabled,
		ProjectID: req.ProjectID,
	})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.OK(c, toClaritySettingsResponse(settings))
}

func GetGeoSettings(c echo.Context) error {
	settings, err := usecase.GetGeoSettings(fwcontext.InternalUsecaseContext(c), usecase.GeoSettingsQry{})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.OK(c, toGeoSettingsResponse(settings))
}

func SaveGeoSettings(c echo.Context) error {
	var req SaveGeoSettingsRequest
	if err := c.Bind(&req); err != nil {
		return httpresponse.BadRequest(c, "invalid request data")
	}

	settings, err := usecase.SaveGeoSettings(fwcontext.InternalUsecaseContext(c), usecase.SaveGeoSettingsCmd{
		Enabled:          req.Enabled,
		V4XDB:            req.V4XDB,
		V6XDB:            req.V6XDB,
		CachePolicy:      req.CachePolicy,
		SearcherPoolSize: req.SearcherPoolSize,
	})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.OK(c, toGeoSettingsResponse(settings))
}

func DownloadGeoXDB(c echo.Context) error {
	currentUser := middleware.GetCurrentUser(c)
	if currentUser == nil {
		return httpresponse.Unauthorized(c, "not logged in")
	}

	var req GeoXDBDownloadRequest
	if err := c.Bind(&req); err != nil {
		return httpresponse.BadRequest(c, "invalid request data")
	}

	result, err := usecase.EnqueueGeoXDBDownload(fwcontext.InternalUsecaseContext(c), usecase.EnqueueGeoXDBDownloadCmd{
		UserID:  currentUser.ID,
		Version: req.Version,
	})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.Created(c, GeoXDBDownloadResponse{
		TaskID:  result.TaskID,
		Version: result.Version,
		Message: result.Message,
	})
}

func CheckGeoXDB(c echo.Context) error {
	result, err := usecase.CheckGeoXDB(fwcontext.InternalUsecaseContext(c), usecase.CheckGeoXDBCmd{})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.OK(c, GeoXDBCheckResponse{
		Valid:    result.Valid,
		Message:  result.Message,
		Settings: toGeoSettingsResponse(result.Settings),
	})
}

type WorkerLimitResponse struct {
	Limit int `json:"limit"`
}

func GetWorkerLimit(c echo.Context) error {
	limit, err := usecase.GetWorkerLimit(fwcontext.InternalUsecaseContext(c))
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.OK(c, WorkerLimitResponse{Limit: limit})
}

type SaveWorkerLimitRequest struct {
	Limit int `json:"limit"`
}

func SaveWorkerLimit(c echo.Context) error {
	var req SaveWorkerLimitRequest
	if err := c.Bind(&req); err != nil {
		return httpresponse.BadRequest(c, "invalid request data")
	}

	limit, err := usecase.SaveWorkerLimit(fwcontext.InternalUsecaseContext(c), req.Limit)
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.OK(c, WorkerLimitResponse{Limit: limit})
}

func toSiteSettingsResponse(settings usecase.SiteSettingsCo) SiteSettingsResponse {
	return SiteSettingsResponse{
		LogoURL:                     settings.LogoURL,
		LogoConfigured:              settings.LogoConfigured,
		LogoUpdatedAt:               settings.LogoUpdatedAt,
		LogoUploadAvailable:         settings.LogoUploadAvailable,
		LogoUploadUnavailableReason: settings.LogoUploadUnavailableReason,
	}
}

func toClaritySettingsResponse(settings usecase.ClaritySettingsCo) ClaritySettingsResponse {
	return ClaritySettingsResponse{
		Enabled:   settings.Enabled,
		ProjectID: settings.ProjectID,
		UpdatedAt: settings.UpdatedAt,
	}
}

type MarketingDomainResponse struct {
	Domain string `json:"domain"`
}

type SaveMarketingDomainRequest struct {
	Domain string `json:"domain"`
}

func GetMarketingDomain(c echo.Context) error {
	domain, err := usecase.GetMarketingDomain(fwcontext.InternalUsecaseContext(c), usecase.MarketingDomainQry{})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.OK(c, MarketingDomainResponse{Domain: domain.Domain})
}

func SaveMarketingDomain(c echo.Context) error {
	var req SaveMarketingDomainRequest
	if err := c.Bind(&req); err != nil {
		return httpresponse.BadRequest(c, "invalid request data")
	}

	domain, err := usecase.SaveMarketingDomain(fwcontext.InternalUsecaseContext(c), usecase.SaveMarketingDomainCmd{
		Domain: req.Domain,
	})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.OK(c, MarketingDomainResponse{Domain: domain.Domain})
}

func toGeoSettingsResponse(settings usecase.GeoSettingsCo) GeoSettingsResponse {
	return GeoSettingsResponse{
		Enabled:          settings.Enabled,
		V4XDB:            settings.V4XDB,
		V6XDB:            settings.V6XDB,
		CachePolicy:      settings.CachePolicy,
		SearcherPoolSize: settings.SearcherPoolSize,
		XDBChecked:       settings.XDBChecked,
		UpdatedAt:        settings.UpdatedAt,
	}
}
