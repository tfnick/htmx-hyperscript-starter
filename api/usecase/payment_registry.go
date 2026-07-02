package usecase

import (
	"fmt"
	"sync"

	"github.com/tfnick/go-svelte-starter/api/usecase/integrations/payment"
)

var (
	paymentAdaptersMu sync.RWMutex
	paymentAdapters   = map[string]payment.Adapter{}
)

func RegisterPaymentAdapter(adapterKey string, adapter payment.Adapter) error {
	if adapterKey == "" {
		return fmt.Errorf("payment adapter key is required")
	}
	if adapter == nil {
		return fmt.Errorf("payment adapter is required")
	}

	paymentAdaptersMu.Lock()
	defer paymentAdaptersMu.Unlock()
	paymentAdapters[adapterKey] = adapter
	return nil
}

func registeredPaymentAdapter(adapterKey string) (payment.Adapter, bool) {
	paymentAdaptersMu.RLock()
	defer paymentAdaptersMu.RUnlock()
	adapter, ok := paymentAdapters[adapterKey]
	return adapter, ok
}
