package usecase

import (
	"fmt"
	"sync"

	"github.com/tfnick/go-svelte-starter/api/usecase/integrations/oss"
)

var (
	ossAdaptersMu sync.RWMutex
	ossAdapters   = map[string]oss.Adapter{}
)

func RegisterOSSAdapter(adapterKey string, adapter oss.Adapter) error {
	if adapterKey == "" {
		return fmt.Errorf("OSS adapter key is required")
	}
	if adapter == nil {
		return fmt.Errorf("OSS adapter is required")
	}

	ossAdaptersMu.Lock()
	defer ossAdaptersMu.Unlock()
	ossAdapters[adapterKey] = adapter
	return nil
}

func registeredOSSAdapter(adapterKey string) (oss.Adapter, bool) {
	ossAdaptersMu.RLock()
	defer ossAdaptersMu.RUnlock()
	adapter, ok := ossAdapters[adapterKey]
	return adapter, ok
}
