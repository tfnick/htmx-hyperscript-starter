package usecase

import (
	"fmt"
	"sync"

	"github.com/tfnick/go-svelte-starter/api/usecase/integrations/llm"
)

var (
	llmAdaptersMu sync.RWMutex
	llmAdapters   = map[string]llm.Adapter{}
)

func RegisterLLMAdapter(adapterKey string, adapter llm.Adapter) error {
	if adapterKey == "" {
		return fmt.Errorf("LLM adapter key is required")
	}
	if adapter == nil {
		return fmt.Errorf("LLM adapter is required")
	}

	llmAdaptersMu.Lock()
	defer llmAdaptersMu.Unlock()
	llmAdapters[adapterKey] = adapter
	return nil
}

func registeredLLMAdapter(adapterKey string) (llm.Adapter, bool) {
	llmAdaptersMu.RLock()
	defer llmAdaptersMu.RUnlock()
	adapter, ok := llmAdapters[adapterKey]
	return adapter, ok
}
