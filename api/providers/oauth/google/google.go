package google

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/tfnick/go-svelte-starter/api/framework/integrations/providererror"
	"github.com/tfnick/go-svelte-starter/api/usecase/integrations/oauth"
)

const (
	authorizationEndpoint = "https://accounts.google.com/o/oauth2/v2/auth"
	tokenEndpoint         = "https://oauth2.googleapis.com/token"
	userInfoEndpoint      = "https://openidconnect.googleapis.com/v1/userinfo"
)

type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

type Adapter struct {
	client HTTPDoer
}

func NewAdapter(client HTTPDoer) *Adapter {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &Adapter{client: client}
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Error       string `json:"error"`
	Description string `json:"error_description"`
}

type userInfoResponse struct {
	ID            string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
}

func (a *Adapter) AuthorizationURL(cfg oauth.ProviderConfig, req oauth.AuthorizationRequest) (oauth.AuthorizationResult, error) {
	if err := validateConfig(cfg); err != nil {
		return oauth.AuthorizationResult{}, err
	}
	if strings.TrimSpace(req.State) == "" {
		return oauth.AuthorizationResult{}, providererror.New(providererror.CategoryValidation, false, "OAuth state is required", nil)
	}

	values := url.Values{}
	values.Set("client_id", strings.TrimSpace(cfg.ClientID))
	values.Set("redirect_uri", strings.TrimSpace(cfg.RedirectURL))
	values.Set("response_type", "code")
	values.Set("scope", "openid email profile")
	values.Set("state", strings.TrimSpace(req.State))

	return oauth.AuthorizationResult{URL: authorizationEndpoint + "?" + values.Encode()}, nil
}

func (a *Adapter) FetchIdentity(ctx context.Context, cfg oauth.ProviderConfig, req oauth.CallbackRequest) (oauth.ProviderIdentity, error) {
	if err := validateConfig(cfg); err != nil {
		return oauth.ProviderIdentity{}, err
	}
	code := strings.TrimSpace(req.Code)
	if code == "" {
		return oauth.ProviderIdentity{}, providererror.New(providererror.CategoryValidation, false, "OAuth code is required", nil)
	}

	accessToken, err := a.exchangeCode(ctx, cfg, code)
	if err != nil {
		return oauth.ProviderIdentity{}, err
	}
	userInfo, err := a.fetchUserInfo(ctx, accessToken)
	if err != nil {
		return oauth.ProviderIdentity{}, err
	}
	if strings.TrimSpace(userInfo.ID) == "" {
		return oauth.ProviderIdentity{}, providererror.New(providererror.CategoryProviderInternal, true, "OAuth provider user is missing", nil)
	}

	return oauth.ProviderIdentity{
		Provider:       oauth.ProviderGoogle,
		ProviderUserID: strings.TrimSpace(userInfo.ID),
		Email:          strings.TrimSpace(userInfo.Email),
		EmailVerified:  userInfo.EmailVerified,
		DisplayName:    strings.TrimSpace(userInfo.Name),
	}, nil
}

func (a *Adapter) exchangeCode(ctx context.Context, cfg oauth.ProviderConfig, code string) (string, error) {
	body := url.Values{}
	body.Set("client_id", strings.TrimSpace(cfg.ClientID))
	body.Set("client_secret", strings.TrimSpace(cfg.ClientSecret))
	body.Set("redirect_uri", strings.TrimSpace(cfg.RedirectURL))
	body.Set("grant_type", "authorization_code")
	body.Set("code", code)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenEndpoint, strings.NewReader(body.Encode()))
	if err != nil {
		return "", providererror.New(providererror.CategoryTemporary, true, "failed to create OAuth token request", err)
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	httpReq.Header.Set("Accept", "application/json")

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return "", providererror.New(providererror.CategoryTemporary, true, "OAuth provider token request failed", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", providererror.New(providererror.CategoryTemporary, true, "failed to read OAuth token response", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", oauthStatusError(resp.StatusCode)
	}

	var parsed tokenResponse
	if err := json.Unmarshal(responseBody, &parsed); err != nil {
		return "", providererror.New(providererror.CategoryProviderInternal, true, "OAuth token response is invalid", err)
	}
	if parsed.Error != "" {
		return "", providererror.New(providererror.CategoryAuth, false, "OAuth token exchange failed", fmt.Errorf("%s: %s", parsed.Error, parsed.Description))
	}
	if strings.TrimSpace(parsed.AccessToken) == "" {
		return "", providererror.New(providererror.CategoryProviderInternal, true, "OAuth token response is incomplete", nil)
	}
	return strings.TrimSpace(parsed.AccessToken), nil
}

func (a *Adapter) fetchUserInfo(ctx context.Context, accessToken string) (userInfoResponse, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, userInfoEndpoint, nil)
	if err != nil {
		return userInfoResponse{}, providererror.New(providererror.CategoryTemporary, true, "failed to create OAuth user request", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+accessToken)
	httpReq.Header.Set("Accept", "application/json")

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return userInfoResponse{}, providererror.New(providererror.CategoryTemporary, true, "OAuth provider user request failed", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return userInfoResponse{}, providererror.New(providererror.CategoryTemporary, true, "failed to read OAuth user response", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return userInfoResponse{}, oauthStatusError(resp.StatusCode)
	}

	var parsed userInfoResponse
	if err := json.Unmarshal(responseBody, &parsed); err != nil {
		return userInfoResponse{}, providererror.New(providererror.CategoryProviderInternal, true, "OAuth user response is invalid", err)
	}
	return parsed, nil
}

func validateConfig(cfg oauth.ProviderConfig) error {
	if strings.TrimSpace(cfg.ClientID) == "" {
		return providererror.New(providererror.CategoryAuth, false, "Google OAuth client ID is required", nil)
	}
	if strings.TrimSpace(cfg.ClientSecret) == "" {
		return providererror.New(providererror.CategoryAuth, false, "Google OAuth client secret is required", nil)
	}
	if strings.TrimSpace(cfg.RedirectURL) == "" {
		return providererror.New(providererror.CategoryValidation, false, "Google OAuth redirect URL is required", nil)
	}
	return nil
}

func oauthStatusError(statusCode int) *providererror.Error {
	category := providererror.CategoryProviderInternal
	retryable := statusCode >= 500 || statusCode == http.StatusTooManyRequests || statusCode == http.StatusRequestTimeout

	switch statusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		category = providererror.CategoryAuth
		retryable = false
	case http.StatusTooManyRequests:
		category = providererror.CategoryRateLimit
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		category = providererror.CategoryValidation
		retryable = false
	default:
		if statusCode >= 400 && statusCode < 500 {
			category = providererror.CategoryPermanent
			retryable = false
		}
	}

	return providererror.New(category, retryable, "OAuth provider request failed", nil)
}
