package oauth

import "context"

const (
	ProviderGoogle = "google"
	ProviderGitHub = "github"
)

type ProviderConfig struct {
	Provider     string
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

type AuthorizationRequest struct {
	State string
}

type AuthorizationResult struct {
	URL string
}

type CallbackRequest struct {
	Code string
}

type ProviderIdentity struct {
	Provider       string
	ProviderUserID string
	Email          string
	EmailVerified  bool
	DisplayName    string
}

type Adapter interface {
	AuthorizationURL(cfg ProviderConfig, req AuthorizationRequest) (AuthorizationResult, error)
	FetchIdentity(ctx context.Context, cfg ProviderConfig, req CallbackRequest) (ProviderIdentity, error)
}
