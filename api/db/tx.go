package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"

	"github.com/tfnick/sqlx"
)

var ErrTransactionsOnlySupportApp = errors.New("transactions only support app database")
var ErrNoActiveAppTx = errors.New("no active app transaction")

type AfterCommitFunc func(context.Context)

type txContextKey struct{}

type txEntry struct {
	dbName string
	engine *sqlx.Engine
	rawTx  *sql.Tx
	hooks  *txHooks
}

type txContextValue struct {
	entries []txEntry
}

type txHooks struct {
	mu          sync.Mutex
	active      bool
	afterCommit []AfterCommitFunc
}

func (m *DBManager) WithTx(ctx context.Context, name string, fn func(context.Context) error) error {
	if name != "app" {
		return fmt.Errorf("%w: %s", ErrTransactionsOnlySupportApp, name)
	}
	if _, ok := txFromContext(ctx, name); ok {
		return fn(ctx)
	}

	engine, err := m.GetEngine(name)
	if err != nil {
		return err
	}

	hooks := &txHooks{active: true}
	if err := engine.WithTransactionRaw(ctx, nil, func(txEngine *sqlx.Engine, rawTx *sql.Tx) error {
		return fn(contextWithTx(ctx, name, txEngine, rawTx, hooks))
	}); err != nil {
		hooks.deactivate()
		return err
	}

	hooks.runAfterCommit(ctx)
	return nil
}

func (m *DBManager) Engine(ctx context.Context, name string) (*sqlx.Engine, error) {
	if tx, ok := txFromContext(ctx, name); ok {
		return tx.engine, nil
	}
	return m.GetEngine(name)
}

func (m *DBManager) SQLDB(name string) (*sql.DB, error) {
	engine, err := m.GetEngine(name)
	if err != nil {
		return nil, err
	}
	return engine.StdDB(), nil
}

func (m *DBManager) SQLTx(ctx context.Context, name string) (*sql.Tx, bool) {
	tx, ok := txFromContext(ctx, name)
	if !ok {
		return nil, false
	}
	return tx.rawTx, true
}

func contextWithTx(ctx context.Context, name string, engine *sqlx.Engine, rawTx *sql.Tx, hooks *txHooks) context.Context {
	value, _ := ctx.Value(txContextKey{}).(txContextValue)
	next := txContextValue{
		entries: make([]txEntry, 0, len(value.entries)+1),
	}
	next.entries = append(next.entries, value.entries...)
	next.entries = append(next.entries, txEntry{dbName: name, engine: engine, rawTx: rawTx, hooks: hooks})
	return context.WithValue(ctx, txContextKey{}, next)
}

func txFromContext(ctx context.Context, name string) (txEntry, bool) {
	value, ok := ctx.Value(txContextKey{}).(txContextValue)
	if !ok {
		return txEntry{}, false
	}

	for i := len(value.entries) - 1; i >= 0; i-- {
		entry := value.entries[i]
		if entry.dbName == name && entry.isActive() {
			return entry, true
		}
	}
	return txEntry{}, false
}

func (e txEntry) isActive() bool {
	return e.hooks == nil || e.hooks.isActive()
}

func InTx(ctx context.Context, name string) bool {
	_, ok := txFromContext(ctx, name)
	return ok
}

func RegisterAfterCommit(ctx context.Context, fn AfterCommitFunc) error {
	if fn == nil {
		return nil
	}

	tx, ok := txFromContext(ctx, "app")
	if !ok || tx.hooks == nil {
		return ErrNoActiveAppTx
	}
	if !tx.hooks.addAfterCommit(fn) {
		return ErrNoActiveAppTx
	}
	return nil
}

func (h *txHooks) addAfterCommit(fn AfterCommitFunc) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	if !h.active {
		return false
	}
	h.afterCommit = append(h.afterCommit, fn)
	return true
}

func (h *txHooks) runAfterCommit(ctx context.Context) {
	callbacks := h.deactivateAndDrainAfterCommit()
	for _, fn := range callbacks {
		fn(ctx)
	}
}

func (h *txHooks) isActive() bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	return h.active
}

func (h *txHooks) deactivate() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.active = false
	h.afterCommit = nil
}

func (h *txHooks) deactivateAndDrainAfterCommit() []AfterCommitFunc {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.active = false
	callbacks := h.afterCommit
	h.afterCommit = nil
	if len(callbacks) == 0 {
		return nil
	}

	next := make([]AfterCommitFunc, len(callbacks))
	copy(next, callbacks)
	return next
}

func WithTx(ctx context.Context, name string, fn func(context.Context) error) error {
	return DefaultManager.WithTx(ctx, name, fn)
}

func EngineFor(ctx context.Context, name string) (*sqlx.Engine, error) {
	return DefaultManager.Engine(ctx, name)
}

func SQLDBFor(name string) (*sql.DB, error) {
	return DefaultManager.SQLDB(name)
}

func SQLTxFor(ctx context.Context, name string) (*sql.Tx, bool) {
	return DefaultManager.SQLTx(ctx, name)
}
