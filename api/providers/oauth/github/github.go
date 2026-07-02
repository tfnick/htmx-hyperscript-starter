package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/tfnick/go-svelte-starter/api/framework/integrations/providererror"
	"github.com/tfnick/go-svelte-starter/api/usecase/integrations/oauth"
)

const (
	authorizationEndpoint = "https://github.com/login/oauth/authorize"
	tokenEndpoint         = "https://github.com/login/oauth/access_token"
	userEndpoint          = "https://api.github.com/user"
	emailsEndpoint        = "https://api.github.com/user/emails"
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

type tokenRequest struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	Code         string `json:"code"`
	RedirectURL  string `json:"redirect_uri"`
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Error       string `json:"error"`
	Description string `json:"error_description"`
}

type userResponse struct {
	ID    flexibleString `json:"id"`
	Login string         `json:"login"`
	Name  string         `json:"name"`
	Email string         `json:"email"`
}

type emailResponse struct {
	Email    string `json:"email"`
	Primary  bool   `json:"primary"`
	Verified bool   `json:"verified"`
}

type flexibleString string

func (v *flexibleString) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		*v = ""
		return nil
	}
	if trimmed[0] == '"' {
		var value string
		if err := json.Unmarshal(trimmed, &value); err != nil {
			return err
		}
		*v = flexibleString(value)
		return nil
	}
	var number int64
	if err := json.Unmarshal(trimmed, &number); err != nil {
		return err
	}
	*v = flexibleString(strconv.FormatInt(number, 10))
	return nil
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
	values.Set("scope", "user:email")
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
	user, err := a.fetchUser(ctx, accessToken)
	if err != nil {
		return oauth.ProviderIdentity{}, err
	}
	if strings.TrimSpace(string(user.ID)) == "" {
		return oauth.ProviderIdentity{}, providererror.New(providererror.CategoryProviderInternal, true, "OAuth provider user is missing", nil)
	}

	email, verified, err := a.resolveVerifiedEmail(ctx, accessToken, user.Email)
	if err != nil {
		return oauth.ProviderIdentity{}, err
	}
	displayName := firstNonEmpty(user.Name, user.Login)

	return oauth.ProviderIdentity{
		Provider:       oauth.ProviderGitHub,
		ProviderUserID: strings.TrimSpace(string(user.ID)),
		Email:          email,
		EmailVerified:  verified,
		DisplayName:    displayName,
	}, nil
}

func (a *Adapter) exchangeCode(ctx context.Context, cfg oauth.ProviderConfig, code string) (string, error) {
	body, err := json.Marshal(tokenRequest{
		ClientID:     strings.TrimSpace(cfg.ClientID),
		ClientSecret: strings.TrimSpace(cfg.ClientSecret),
		Code:         code,
		RedirectURL:  strings.TrimSpace(cfg.RedirectURL),
	})
	if err != nil {
		return "", providererror.New(providererror.CategoryValidation, false, "OAuth token request is invalid", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenEndpoint, bytes.NewReader(body))
	if err != nil {
		return "", providererror.New(providererror.CategoryTemporary, true, "failed to create OAuth token request", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
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

func (a *Adapter) fetchUser(ctx context.Context, accessToken string) (userResponse, error) {
	var parsed userResponse
	if err := a.getJSON(ctx, userEndpoint, accessToken, &parsed); err != nil {
		return userResponse{}, err
	}
	return parsed, nil
}

func (a *Adapter) resolveVerifiedEmail(ctx context.Context, accessToken string, profileEmail string) (string, bool, error) {
	var emails []emailResponse
	if err := a.getJSON(ctx, emailsEndpoint, accessToken, &emails); err != nil {
		return "", false, err
	}
	for _, candidate := range emails {
		if candidate.Primary && candidate.Verified && strings.TrimSpace(candidate.Email) != "" {
			return strings.TrimSpace(candidate.Email), true, nil
		}
	}
	for _, candidate := range emails {
		if candidate.Verified && strings.TrimSpace(candidate.Email) != "" {
			return strings.TrimSpace(candidate.Email), true, nil
		}
	}
	if strings.TrimSpace(profileEmail) != "" {
		return strings.TrimSpace(profileEmail), false, nil
	}
	return "", false, nil
}

func (a *Adapter) getJSON(ctx context.Context, endpoint string, accessToken string, target interface{}) error {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return providererror.New(providererror.CategoryTemporary, true, "failed to create OAuth provider request", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+accessToken)
	httpReq.Header.Set("Accept", "application/json")

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return providererror.New(providererror.CategoryTemporary, true, "OAuth provider request failed", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return providererror.New(providererror.CategoryTemporary, true, "failed to read OAuth provider response", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return oauthStatusError(resp.StatusCode)
	}
	if err := json.Unmarshal(responseBody, target); err != nil {
		return providererror.New(providererror.CategoryProviderInternal, true, "OAuth provider response is invalid", err)
	}
	return nil
}

func validateConfig(cfg oauth.ProviderConfig) error {
	if strings.TrimSpace(cfg.ClientID) == "" {
		return providererror.New(providererror.CategoryAuth, false, "GitHub OAuth client ID is required", nil)
	}
	if strings.TrimSpace(cfg.ClientSecret) == "" {
		return providererror.New(providererror.CategoryAuth, false, "GitHub OAuth client secret is required", nil)
	}
	if strings.TrimSpace(cfg.RedirectURL) == "" {
		return providererror.New(providererror.CategoryValidation, false, "GitHub OAuth redirect URL is required", nil)
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
