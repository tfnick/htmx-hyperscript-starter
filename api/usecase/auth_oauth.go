package usecase

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/tfnick/go-svelte-starter/api/framework/data/modelerror"
	"github.com/tfnick/go-svelte-starter/api/framework/integrations/providererror"
	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
	"github.com/tfnick/go-svelte-starter/api/models"
	"github.com/tfnick/go-svelte-starter/api/usecase/integrations/oauth"
)

const appPublicBaseURLEnv = "APP_PUBLIC_BASE_URL"

type OAuthStartCmd struct {
	Provider       string
	RedirectPath   string
	RequestBaseURL string
}

type OAuthStartCo struct {
	AuthorizationURL string
}

type OAuthCallbackCmd struct {
	Provider       string
	Code           string
	State          string
	RequestBaseURL string
	Context        RegistrationContext
}

type OAuthCallbackCo struct {
	ResultToken  string
	RedirectPath string
}

type OAuthExchangeCmd struct {
	Token string
}

func StartOAuthLogin(ctx fwusecase.Context, cmd OAuthStartCmd) (OAuthStartCo, error) {
	provider, err := normalizeOAuthProvider(cmd.Provider)
	if err != nil {
		return OAuthStartCo{}, err
	}
	adapter, cfg, err := oauthAdapterConfig(provider, cmd.RequestBaseURL)
	if err != nil {
		return OAuthStartCo{}, err
	}

	redirectPath := normalizeOAuthRedirectPath(cmd.RedirectPath)
	state, err := models.CreateOAuthState(ctx.Std(), provider, redirectPath)
	if err != nil {
		return OAuthStartCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to start OAuth login", err)
	}

	result, err := adapter.AuthorizationURL(cfg, oauth.AuthorizationRequest{State: state})
	if err != nil {
		return OAuthStartCo{}, oauthProviderUsecaseError("failed to start OAuth login", err)
	}
	if strings.TrimSpace(result.URL) == "" {
		return OAuthStartCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to start OAuth login", fmt.Errorf("empty authorization URL"))
	}
	return OAuthStartCo{AuthorizationURL: result.URL}, nil
}

func CompleteOAuthLogin(ctx fwusecase.Context, cmd OAuthCallbackCmd) (OAuthCallbackCo, error) {
	provider, err := normalizeOAuthProvider(cmd.Provider)
	if err != nil {
		return OAuthCallbackCo{}, err
	}
	if strings.TrimSpace(cmd.Code) == "" || strings.TrimSpace(cmd.State) == "" {
		return OAuthCallbackCo{}, fwusecase.E(fwusecase.CodeValidation, "OAuth callback is missing code or state", nil)
	}
	adapter, cfg, err := oauthAdapterConfig(provider, cmd.RequestBaseURL)
	if err != nil {
		return OAuthCallbackCo{}, err
	}

	state, err := models.UseOAuthState(ctx.Std(), provider, strings.TrimSpace(cmd.State))
	if err != nil {
		if errors.Is(err, modelerror.ErrNotFound) {
			return OAuthCallbackCo{}, fwusecase.E(fwusecase.CodeValidation, "OAuth state is invalid or expired", err)
		}
		return OAuthCallbackCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to verify OAuth state", err)
	}

	identity, err := adapter.FetchIdentity(ctx.Std(), cfg, oauth.CallbackRequest{Code: strings.TrimSpace(cmd.Code)})
	if err != nil {
		return OAuthCallbackCo{}, oauthProviderUsecaseError("OAuth provider request failed", err)
	}
	if err := validateOAuthIdentity(provider, identity); err != nil {
		return OAuthCallbackCo{}, err
	}

	var resultToken string
	err = fwusecase.WithAppTx(ctx, func(txCtx fwusecase.Context) error {
		user, err := resolveOAuthUser(txCtx, provider, identity, cmd.Context)
		if err != nil {
			return err
		}
		token, err := models.CreateOAuthLoginResult(txCtx.Std(), user.ID, state.RedirectPath)
		if err != nil {
			return fwusecase.E(fwusecase.CodeInternal, "failed to create OAuth login result", err)
		}
		resultToken = token
		return nil
	})
	if err != nil {
		return OAuthCallbackCo{}, err
	}

	return OAuthCallbackCo{
		ResultToken:  resultToken,
		RedirectPath: normalizeOAuthRedirectPath(state.RedirectPath),
	}, nil
}

func ExchangeOAuthLoginResult(ctx fwusecase.Context, cmd OAuthExchangeCmd) (AuthCo, error) {
	if strings.TrimSpace(cmd.Token) == "" {
		return AuthCo{}, fwusecase.E(fwusecase.CodeValidation, "OAuth login token is required", nil)
	}

	var loginResult *models.OAuthLoginResult
	err := fwusecase.WithAppTx(ctx, func(txCtx fwusecase.Context) error {
		result, err := models.UseOAuthLoginResult(txCtx.Std(), strings.TrimSpace(cmd.Token))
		if err != nil {
			if errors.Is(err, modelerror.ErrNotFound) {
				return fwusecase.E(fwusecase.CodeValidation, "OAuth login result is invalid or expired", err)
			}
			return fwusecase.E(fwusecase.CodeInternal, "failed to exchange OAuth login result", err)
		}
		loginResult = result
		return nil
	})
	if err != nil {
		return AuthCo{}, err
	}

	user, err := models.GetUserByID(ctx.Std(), loginResult.UserID)
	if err != nil {
		return AuthCo{}, fwusecase.E(fwusecase.CodeUnauthorized, "OAuth login result is invalid or expired", err)
	}
	if user.IsActive == 0 {
		return AuthCo{}, fwusecase.E(fwusecase.CodeForbidden, "account is disabled", nil)
	}
	return AuthCo{User: userCoFromModel(user)}, nil
}

func resolveOAuthUser(ctx fwusecase.Context, provider string, identity oauth.ProviderIdentity, registrationContext RegistrationContext) (*models.User, error) {
	providerUserID := strings.TrimSpace(identity.ProviderUserID)
	email := normalizeOAuthEmail(identity.Email)
	displayName := strings.TrimSpace(identity.DisplayName)

	existingIdentity, err := models.GetOAuthIdentityByProviderUserID(ctx.Std(), provider, providerUserID)
	if err != nil {
		return nil, fwusecase.E(fwusecase.CodeInternal, "failed to load OAuth identity", err)
	}
	if existingIdentity != nil {
		user, err := models.GetUserByID(ctx.Std(), existingIdentity.UserID)
		if err != nil {
			return nil, fwusecase.E(fwusecase.CodeUnauthorized, "OAuth identity is not linked to an active user", err)
		}
		if user.IsActive == 0 {
			return nil, fwusecase.E(fwusecase.CodeForbidden, "account is disabled", nil)
		}
		return user, nil
	}

	user, err := models.GetUserByEmailOptional(ctx.Std(), email)
	if err != nil {
		return nil, fwusecase.E(fwusecase.CodeInternal, "failed to load user by OAuth email", err)
	}
	if user != nil {
		if user.IsActive == 0 {
			return nil, fwusecase.E(fwusecase.CodeForbidden, "account is disabled", nil)
		}
		if _, err := models.CreateOAuthIdentity(ctx.Std(), oauthIdentityInput(provider, providerUserID, user.ID, email, displayName)); err != nil {
			return nil, fwusecase.E(fwusecase.CodeInternal, "failed to link OAuth identity", err)
		}
		return user, nil
	}

	user = &models.User{
		Name:     firstNonEmpty(displayName, emailLocalPart(email), email),
		Email:    email,
		IsActive: 1,
	}
	if err := models.CreateOAuthUser(ctx.Std(), user); err != nil {
		return nil, fwusecase.E(fwusecase.CodeInternal, "failed to create OAuth user", err)
	}
	if err := createRegistrationProfile(ctx, user.ID, registrationContext); err != nil {
		return nil, err
	}
	if _, err := models.CreateOAuthIdentity(ctx.Std(), oauthIdentityInput(provider, providerUserID, user.ID, email, displayName)); err != nil {
		return nil, fwusecase.E(fwusecase.CodeInternal, "failed to link OAuth identity", err)
	}
	return user, nil
}

func oauthIdentityInput(provider string, providerUserID string, userID string, email string, displayName string) models.OAuthIdentityInput {
	return models.OAuthIdentityInput{
		Provider:       provider,
		ProviderUserID: providerUserID,
		UserID:         userID,
		Email:          email,
		EmailVerified:  1,
		DisplayName:    displayName,
	}
}

func validateOAuthIdentity(provider string, identity oauth.ProviderIdentity) error {
	if strings.TrimSpace(identity.ProviderUserID) == "" {
		return fwusecase.E(fwusecase.CodeValidation, "OAuth provider user is missing", nil)
	}
	if identity.Provider != "" && strings.TrimSpace(strings.ToLower(identity.Provider)) != provider {
		return fwusecase.E(fwusecase.CodeValidation, "OAuth provider identity is invalid", nil)
	}
	if normalizeOAuthEmail(identity.Email) == "" {
		return fwusecase.E(fwusecase.CodeValidation, "OAuth verified email is required", nil)
	}
	if !identity.EmailVerified {
		return fwusecase.E(fwusecase.CodeValidation, "OAuth verified email is required", nil)
	}
	return nil
}

func oauthAdapterConfig(provider string, requestBaseURL string) (oauth.Adapter, oauth.ProviderConfig, error) {
	adapter, ok := registeredOAuthAdapter(provider)
	if !ok {
		return nil, oauth.ProviderConfig{}, fwusecase.E(fwusecase.CodeInternal, "OAuth provider is not registered", fmt.Errorf("provider %s is not registered", provider))
	}

	clientID, clientSecret := oauthClientEnv(provider)
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv(appPublicBaseURLEnv)), "/")
	if baseURL == "" {
		baseURL = strings.TrimRight(strings.TrimSpace(requestBaseURL), "/")
	}
	if strings.TrimSpace(clientID) == "" || strings.TrimSpace(clientSecret) == "" || baseURL == "" {
		return nil, oauth.ProviderConfig{}, fwusecase.E(fwusecase.CodeValidation, "OAuth provider is not configured", nil)
	}
	redirectURL, err := oauthRedirectURL(baseURL, provider)
	if err != nil {
		return nil, oauth.ProviderConfig{}, fwusecase.E(fwusecase.CodeValidation, "OAuth public base URL is invalid", err)
	}

	return adapter, oauth.ProviderConfig{
		Provider:     provider,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
	}, nil
}

func oauthClientEnv(provider string) (string, string) {
	switch provider {
	case oauth.ProviderGoogle:
		return os.Getenv("GOOGLE_OAUTH_CLIENT_ID"), os.Getenv("GOOGLE_OAUTH_CLIENT_SECRET")
	case oauth.ProviderGitHub:
		return os.Getenv("GITHUB_OAUTH_CLIENT_ID"), os.Getenv("GITHUB_OAUTH_CLIENT_SECRET")
	default:
		return "", ""
	}
}

func normalizeOAuthProvider(provider string) (string, error) {
	switch strings.TrimSpace(strings.ToLower(provider)) {
	case oauth.ProviderGoogle:
		return oauth.ProviderGoogle, nil
	case oauth.ProviderGitHub:
		return oauth.ProviderGitHub, nil
	default:
		return "", fwusecase.E(fwusecase.CodeValidation, "OAuth provider is not supported", nil)
	}
}

func oauthRedirectURL(baseURL string, provider string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return "", err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("base URL must include scheme and host")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/api/auth/oauth/" + url.PathEscape(provider) + "/callback"
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func normalizeOAuthRedirectPath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "/"
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.IsAbs() || parsed.Host != "" || strings.HasPrefix(value, "//") {
		return "/"
	}
	if !strings.HasPrefix(parsed.Path, "/") {
		return "/"
	}
	switch parsed.Path {
	case "/login", "/register", "/forgot-password", "/reset-password", "/login/oauth/callback",
		"/app/login", "/app/register", "/app/forgot-password", "/app/reset-password", "/app/login/oauth/callback":
		return "/"
	default:
		return parsed.RequestURI()
	}
}

func normalizeOAuthEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func emailLocalPart(email string) string {
	local, _, ok := strings.Cut(email, "@")
	if !ok || strings.TrimSpace(local) == "" {
		return email
	}
	return strings.TrimSpace(local)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func oauthProviderUsecaseError(message string, err error) error {
	if providerErr, ok := providererror.From(err); ok {
		switch providerErr.Category {
		case providererror.CategoryAuth, providererror.CategoryValidation:
			return fwusecase.E(fwusecase.CodeValidation, providerErr.Error(), err)
		default:
			return fwusecase.E(fwusecase.CodeInternal, message, err)
		}
	}
	return fwusecase.E(fwusecase.CodeInternal, message, err)
}
