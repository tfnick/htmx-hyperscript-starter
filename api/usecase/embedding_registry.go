package usecase

import (
	"fmt"
	"strings"
	"sync"

	"github.com/tfnick/go-svelte-starter/api/usecase/integrations/embedding"
)

var (
	embeddingAdapterMu sync.RWMutex
	embeddingAdapters  = map[string]embedding.Adapter{}
)

func RegisterEmbeddingAdapter(adapterKey string, adapter embedding.Adapter) error {
	key := strings.TrimSpace(adapterKey)
	if key == "" {
		return fmt.Errorf("embedding adapter key is required")
	}
	if adapter == nil {
		return fmt.Errorf("embedding adapter %s is nil", key)
	}

	embeddingAdapterMu.Lock()
	defer embeddingAdapterMu.Unlock()
	if _, exists := embeddingAdapters[key]; exists {
		return fmt.Errorf("embedding adapter %s already registered", key)
	}
	embeddingAdapters[key] = adapter
	return nil
}

func registeredEmbeddingAdapter(adapterKey string) (embedding.Adapter, bool) {
	key := strings.TrimSpace(adapterKey)
	if key == "" {
		return nil, false
	}
	embeddingAdapterMu.RLock()
	defer embeddingAdapterMu.RUnlock()
	adapter, ok := embeddingAdapters[key]
	return adapter, ok
}
