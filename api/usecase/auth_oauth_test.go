package usecase_test

import (
	"context"
	"net/url"
	"strings"
	"testing"

	"github.com/tfnick/go-svelte-starter/api/db"
	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
	"github.com/tfnick/go-svelte-starter/api/usecase"
	"github.com/tfnick/go-svelte-starter/api/usecase/integrations/oauth"
)

type fakeOAuthAdapter struct {
	identity oauth.ProviderIdentity
	state    string
	cfg      oauth.ProviderConfig
}

func (a *fakeOAuthAdapter) AuthorizationURL(cfg oauth.ProviderConfig, req oauth.AuthorizationRequest) (oauth.AuthorizationResult, error) {
	a.cfg = cfg
	a.state = req.State
	values := url.Values{}
	values.Set("state", req.State)
	return oauth.AuthorizationResult{URL: "https://oauth.example/start?" + values.Encode()}, nil
}

func (a *fakeOAuthAdapter) FetchIdentity(ctx context.Context, cfg oauth.ProviderConfig, req oauth.CallbackRequest) (oauth.ProviderIdentity, error) {
	a.cfg = cfg
	return a.identity, nil
}

func TestOAuthLoginCreatesVerifiedUserAndExchangesResultOnce(t *testing.T) {
	manager := setupUsecaseOrderTxDB(t)
	appDB, err := manager.GetDB("app")
	if err != nil {
		t.Fatalf("get app db: %v", err)
	}

	adapter := &fakeOAuthAdapter{identity: oauth.ProviderIdentity{
		Provider:       oauth.ProviderGoogle,
		ProviderUserID: "google-user-1",
		Email:          "Ada@Example.COM",
		EmailVerified:  true,
		DisplayName:    "Ada Lovelace",
	}}
	registerOAuthTestAdapter(t, adapter)
	usecase.RegisterRegistrationGeoResolver(fakeRegistrationGeoResolver{
		geo: usecase.RegistrationGeo{
			Country: "United States",
			Region:  "California",
		},
	})
	t.Cleanup(func() {
		usecase.RegisterRegistrationGeoResolver(nil)
	})

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	start, err := usecase.StartOAuthLogin(ctx, usecase.OAuthStartCmd{
		Provider:       "google",
		RedirectPath:   "/app/orders?tab=mine",
		RequestBaseURL: "http://127.0.0.1:3000",
	})
	if err != nil {
		t.Fatalf("start oauth login: %v", err)
	}
	if !strings.HasPrefix(start.AuthorizationURL, "https://oauth.example/start?") || adapter.state == "" {
		t.Fatalf("expected fake authorization url and captured state, got url=%q state=%q", start.AuthorizationURL, adapter.state)
	}
	if adapter.cfg.RedirectURL != "https://app.example.test/api/auth/oauth/google/callback" {
		t.Fatalf("expected env public base URL to define callback, got %q", adapter.cfg.RedirectURL)
	}

	callback, err := usecase.CompleteOAuthLogin(ctx, usecase.OAuthCallbackCmd{
		Provider:       "google",
		Code:           "provider-code",
		State:          adapter.state,
		RequestBaseURL: "http://127.0.0.1:3000",
		Context: usecase.RegistrationContext{
			IP:        "203.0.113.20",
			UserAgent: "oauth-registration-test",
		},
	})
	if err != nil {
		t.Fatalf("complete oauth login: %v", err)
	}
	if callback.ResultToken == "" || callback.RedirectPath != "/app/orders?tab=mine" {
		t.Fatalf("unexpected oauth callback result: %#v", callback)
	}

	auth, err := usecase.ExchangeOAuthLoginResult(ctx, usecase.OAuthExchangeCmd{Token: callback.ResultToken})
	if err != nil {
		t.Fatalf("exchange oauth result: %v", err)
	}
	if auth.User.Email != "ada@example.com" || auth.User.Name != "Ada Lovelace" || !auth.User.EmailVerified {
		t.Fatalf("unexpected OAuth-created user: %#v", auth.User)
	}
	if _, err := usecase.ExchangeOAuthLoginResult(ctx, usecase.OAuthExchangeCmd{Token: callback.ResultToken}); fwusecase.CodeOf(err) != fwusecase.CodeValidation {
		t.Fatalf("expected second exchange to fail validation, got %v", err)
	}

	var identityCount int
	if err := appDB.Get(&identityCount, `SELECT COUNT(*) FROM oauth_identities WHERE provider = 'google' AND provider_user_id = 'google-user-1'`); err != nil {
		t.Fatalf("count identities: %v", err)
	}
	if identityCount != 1 {
		t.Fatalf("expected one linked identity, got %d", identityCount)
	}

	var profile struct {
		RegistrationIP        string `db:"registration_ip"`
		RegistrationCountry   string `db:"registration_country"`
		RegistrationRegion    string `db:"registration_region"`
		RegistrationUserAgent string `db:"registration_user_agent"`
	}
	if err := appDB.Get(&profile, `SELECT registration_ip, registration_country, registration_region, registration_user_agent FROM user_registration_profiles WHERE user_id = ?`, auth.User.ID); err != nil {
		t.Fatalf("get OAuth registration profile: %v", err)
	}
	if profile.RegistrationIP != "203.0.113.20" || profile.RegistrationUserAgent != "oauth-registration-test" {
		t.Fatalf("unexpected OAuth registration request metadata: %#v", profile)
	}
	if profile.RegistrationCountry != "United States" || profile.RegistrationRegion != "California" {
		t.Fatalf("unexpected OAuth registration geo metadata: %#v", profile)
	}
}

func TestOAuthLoginAutoLinksExistingVerifiedEmail(t *testing.T) {
	manager := setupUsecaseOrderTxDB(t)
	appDB, err := manager.GetDB("app")
	if err != nil {
		t.Fatalf("get app db: %v", err)
	}
	if _, err := appDB.Exec(`INSERT INTO users (id, name, email, password_hash, email_verified, is_active) VALUES ('existing-user', 'Existing Ada', 'ada@example.com', '', 1, 1)`); err != nil {
		t.Fatalf("insert existing user: %v", err)
	}

	adapter := &fakeOAuthAdapter{identity: oauth.ProviderIdentity{
		Provider:       oauth.ProviderGoogle,
		ProviderUserID: "google-user-2",
		Email:          "ADA@example.com",
		EmailVerified:  true,
		DisplayName:    "Google Ada",
	}}
	registerOAuthTestAdapter(t, adapter)

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	start, err := usecase.StartOAuthLogin(ctx, usecase.OAuthStartCmd{Provider: "google", RedirectPath: "/", RequestBaseURL: "http://127.0.0.1:3000"})
	if err != nil || start.AuthorizationURL == "" {
		t.Fatalf("start oauth login: %v", err)
	}
	callback, err := usecase.CompleteOAuthLogin(ctx, usecase.OAuthCallbackCmd{Provider: "google", Code: "code", State: adapter.state, RequestBaseURL: "http://127.0.0.1:3000"})
	if err != nil {
		t.Fatalf("complete oauth login: %v", err)
	}
	auth, err := usecase.ExchangeOAuthLoginResult(ctx, usecase.OAuthExchangeCmd{Token: callback.ResultToken})
	if err != nil {
		t.Fatalf("exchange oauth result: %v", err)
	}
	if auth.User.ID != "existing-user" || auth.User.Name != "Existing Ada" {
		t.Fatalf("expected existing user to be linked, got %#v", auth.User)
	}

	var linkedUserID string
	if err := appDB.Get(&linkedUserID, `SELECT user_id FROM oauth_identities WHERE provider = 'google' AND provider_user_id = 'google-user-2'`); err != nil {
		t.Fatalf("get linked identity: %v", err)
	}
	if linkedUserID != "existing-user" {
		t.Fatalf("expected identity linked to existing user, got %q", linkedUserID)
	}
}

func TestOAuthLoginRejectsUnverifiedEmail(t *testing.T) {
	setupUsecaseOrderTxDB(t)
	adapter := &fakeOAuthAdapter{identity: oauth.ProviderIdentity{
		Provider:       oauth.ProviderGoogle,
		ProviderUserID: "google-user-3",
		Email:          "ada@example.com",
		EmailVerified:  false,
		DisplayName:    "Ada",
	}}
	registerOAuthTestAdapter(t, adapter)

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	if _, err := usecase.StartOAuthLogin(ctx, usecase.OAuthStartCmd{Provider: "google", RedirectPath: "/", RequestBaseURL: "http://127.0.0.1:3000"}); err != nil {
		t.Fatalf("start oauth login: %v", err)
	}
	_, err := usecase.CompleteOAuthLogin(ctx, usecase.OAuthCallbackCmd{Provider: "google", Code: "code", State: adapter.state, RequestBaseURL: "http://127.0.0.1:3000"})
	if fwusecase.CodeOf(err) != fwusecase.CodeValidation {
		t.Fatalf("expected validation error for unverified email, got %v", err)
	}
}

func TestOAuthLoginRejectsDisabledLinkedUser(t *testing.T) {
	manager := setupUsecaseOrderTxDB(t)
	appDB, err := db.DefaultManager.GetDB("app")
	if err != nil || manager == nil {
		t.Fatalf("get app db: %v", err)
	}
	if _, err := appDB.Exec(`INSERT INTO users (id, name, email, password_hash, email_verified, is_active) VALUES ('disabled-user', 'Disabled Ada', 'disabled@example.com', '', 1, 0)`); err != nil {
		t.Fatalf("insert disabled user: %v", err)
	}
	if _, err := appDB.Exec(`INSERT INTO oauth_identities (id, provider, provider_user_id, user_id, email, email_verified, display_name) VALUES ('identity-disabled', 'google', 'google-disabled', 'disabled-user', 'disabled@example.com', 1, 'Disabled Ada')`); err != nil {
		t.Fatalf("insert identity: %v", err)
	}

	adapter := &fakeOAuthAdapter{identity: oauth.ProviderIdentity{
		Provider:       oauth.ProviderGoogle,
		ProviderUserID: "google-disabled",
		Email:          "disabled@example.com",
		EmailVerified:  true,
		DisplayName:    "Disabled Ada",
	}}
	registerOAuthTestAdapter(t, adapter)

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	if _, err := usecase.StartOAuthLogin(ctx, usecase.OAuthStartCmd{Provider: "google", RedirectPath: "/", RequestBaseURL: "http://127.0.0.1:3000"}); err != nil {
		t.Fatalf("start oauth login: %v", err)
	}
	_, err = usecase.CompleteOAuthLogin(ctx, usecase.OAuthCallbackCmd{Provider: "google", Code: "code", State: adapter.state, RequestBaseURL: "http://127.0.0.1:3000"})
	if fwusecase.CodeOf(err) != fwusecase.CodeForbidden {
		t.Fatalf("expected forbidden error for disabled linked user, got %v", err)
	}
}

func registerOAuthTestAdapter(t *testing.T, adapter *fakeOAuthAdapter) {
	t.Helper()
	t.Setenv("APP_PUBLIC_BASE_URL", "https://app.example.test")
	t.Setenv("GOOGLE_OAUTH_CLIENT_ID", "google-client")
	t.Setenv("GOOGLE_OAUTH_CLIENT_SECRET", "google-secret")
	if err := usecase.RegisterOAuthAdapter(oauth.ProviderGoogle, adapter); err != nil {
		t.Fatalf("register oauth adapter: %v", err)
	}
}
