package usecase

import (
	"fmt"
	"sync"

	"github.com/tfnick/go-svelte-starter/api/usecase/integrations/oauth"
)

var (
	oauthAdaptersMu sync.RWMutex
	oauthAdapters   = map[string]oauth.Adapter{}
)

func RegisterOAuthAdapter(provider string, adapter oauth.Adapter) error {
	if provider == "" {
		return fmt.Errorf("OAuth provider is required")
	}
	if adapter == nil {
		return fmt.Errorf("OAuth adapter is required")
	}

	oauthAdaptersMu.Lock()
	defer oauthAdaptersMu.Unlock()
	oauthAdapters[provider] = adapter
	return nil
}

func registeredOAuthAdapter(provider string) (oauth.Adapter, bool) {
	oauthAdaptersMu.RLock()
	defer oauthAdaptersMu.RUnlock()
	adapter, ok := oauthAdapters[provider]
	return adapter, ok
}
