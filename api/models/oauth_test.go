package models_test

import (
	"errors"
	"testing"

	"github.com/tfnick/go-svelte-starter/api/db"
	"github.com/tfnick/go-svelte-starter/api/framework/data/modelerror"
	"github.com/tfnick/go-svelte-starter/api/models"
)

func TestOAuthStateAndLoginResultAreOneTime(t *testing.T) {
	manager := setupModelsTestDB(t)
	appDB, err := manager.GetDB("app")
	if err != nil {
		t.Fatalf("get app db: %v", err)
	}

	stateToken, err := models.CreateOAuthState(t.Context(), "google", "/orders?tab=mine")
	if err != nil {
		t.Fatalf("create oauth state: %v", err)
	}
	state, err := models.UseOAuthState(t.Context(), "google", stateToken)
	if err != nil {
		t.Fatalf("use oauth state: %v", err)
	}
	if state.RedirectPath != "/orders?tab=mine" {
		t.Fatalf("expected redirect path to round trip, got %q", state.RedirectPath)
	}
	if _, err := models.UseOAuthState(t.Context(), "google", stateToken); !errors.Is(err, modelerror.ErrNotFound) {
		t.Fatalf("expected second state use to fail as not found, got %v", err)
	}

	if _, err := appDB.Exec(`INSERT INTO users (id, name, email, password_hash, email_verified, is_active) VALUES ('oauth-user', 'Ada', 'ada@example.com', '', 1, 1)`); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	resultToken, err := models.CreateOAuthLoginResult(t.Context(), "oauth-user", "/")
	if err != nil {
		t.Fatalf("create oauth login result: %v", err)
	}
	loginResult, err := models.UseOAuthLoginResult(t.Context(), resultToken)
	if err != nil {
		t.Fatalf("use oauth login result: %v", err)
	}
	if loginResult.UserID != "oauth-user" {
		t.Fatalf("expected oauth-user login result, got %#v", loginResult)
	}
	if _, err := models.UseOAuthLoginResult(t.Context(), resultToken); !errors.Is(err, modelerror.ErrNotFound) {
		t.Fatalf("expected second login result use to fail as not found, got %v", err)
	}

	var usedStates int
	if err := appDB.Get(&usedStates, `SELECT COUNT(*) FROM oauth_states WHERE used_at IS NOT NULL`); err != nil {
		t.Fatalf("count used states: %v", err)
	}
	if usedStates != 1 {
		t.Fatalf("expected one used state, got %d", usedStates)
	}
}

func TestOAuthIdentityLookupUsesProviderAndProviderUserID(t *testing.T) {
	setupModelsTestDB(t)

	if err := models.CreateOAuthUser(t.Context(), &models.User{
		ID:       "oauth-user",
		Name:     "Ada",
		Email:    "ada@example.com",
		IsActive: 1,
	}); err != nil {
		t.Fatalf("create oauth user: %v", err)
	}
	if _, err := models.CreateOAuthIdentity(t.Context(), models.OAuthIdentityInput{
		Provider:       "google",
		ProviderUserID: "provider-user-1",
		UserID:         "oauth-user",
		Email:          "ada@example.com",
		EmailVerified:  1,
		DisplayName:    "Ada",
	}); err != nil {
		t.Fatalf("create oauth identity: %v", err)
	}

	found, err := models.GetOAuthIdentityByProviderUserID(t.Context(), "google", "provider-user-1")
	if err != nil {
		t.Fatalf("lookup identity: %v", err)
	}
	if found == nil || found.UserID != "oauth-user" {
		t.Fatalf("expected linked identity, got %#v", found)
	}

	missing, err := models.GetOAuthIdentityByProviderUserID(t.Context(), "github", "provider-user-1")
	if err != nil {
		t.Fatalf("lookup missing identity: %v", err)
	}
	if missing != nil {
		t.Fatalf("expected provider-specific lookup to miss, got %#v", missing)
	}
}

func TestOAuthMigrationCreatesUniqueProviderIdentityConstraint(t *testing.T) {
	manager := setupModelsTestDB(t)
	appDB, err := db.DefaultManager.GetDB("app")
	if err != nil {
		t.Fatalf("get app db: %v", err)
	}
	if appDB == nil || manager == nil {
		t.Fatalf("expected app db manager")
	}

	if _, err := appDB.Exec(`INSERT INTO users (id, name, email, password_hash, email_verified, is_active) VALUES ('oauth-user', 'Ada', 'ada@example.com', '', 1, 1)`); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if _, err := models.CreateOAuthIdentity(t.Context(), models.OAuthIdentityInput{
		Provider:       "google",
		ProviderUserID: "duplicate-provider-user",
		UserID:         "oauth-user",
		Email:          "ada@example.com",
		EmailVerified:  1,
	}); err != nil {
		t.Fatalf("create identity: %v", err)
	}
	if _, err := models.CreateOAuthIdentity(t.Context(), models.OAuthIdentityInput{
		Provider:       "google",
		ProviderUserID: "duplicate-provider-user",
		UserID:         "oauth-user",
		Email:          "ada@example.com",
		EmailVerified:  1,
	}); err == nil {
		t.Fatal("expected duplicate provider identity to fail")
	}
}
