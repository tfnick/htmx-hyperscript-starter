package db

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/tfnick/sqlx"
)

func setupTxTestDB(t *testing.T) *DBManager {
	t.Helper()

	manager := NewDBManager()
	if err := manager.Open("app", "sqlite", filepath.Join(t.TempDir(), "app.db")); err != nil {
		t.Fatalf("open app db: %v", err)
	}
	t.Cleanup(func() {
		_ = manager.Close()
	})

	d, err := manager.GetDB("app")
	if err != nil {
		t.Fatalf("get app db: %v", err)
	}
	if _, err := d.Exec(`CREATE TABLE records (id TEXT PRIMARY KEY, name TEXT NOT NULL)`); err != nil {
		t.Fatalf("create records table: %v", err)
	}

	return manager
}

func TestWithTxCommits(t *testing.T) {
	manager := setupTxTestDB(t)

	err := manager.WithTx(context.Background(), "app", func(ctx context.Context) error {
		eng, err := manager.Engine(ctx, "app")
		if err != nil {
			return err
		}
		_, err = eng.ExecP(`INSERT INTO records (id, name) VALUES (?, ?)`, "1", "committed")
		return err
	})
	if err != nil {
		t.Fatalf("with tx: %v", err)
	}

	eng, err := manager.Engine(context.Background(), "app")
	if err != nil {
		t.Fatalf("engine: %v", err)
	}
	var count int
	if err := eng.GetP(&count, `SELECT COUNT(*) FROM records`); err != nil {
		t.Fatalf("count records: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 committed record, got %d", count)
	}
}

func TestWithTxRollsBackOnError(t *testing.T) {
	manager := setupTxTestDB(t)
	expectedErr := errors.New("stop")

	err := manager.WithTx(context.Background(), "app", func(ctx context.Context) error {
		eng, err := manager.Engine(ctx, "app")
		if err != nil {
			return err
		}
		if _, err := eng.ExecP(`INSERT INTO records (id, name) VALUES (?, ?)`, "1", "rolled back"); err != nil {
			return err
		}
		return expectedErr
	})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected rollback cause %v, got %v", expectedErr, err)
	}

	eng, err := manager.Engine(context.Background(), "app")
	if err != nil {
		t.Fatalf("engine: %v", err)
	}
	var count int
	if err := eng.GetP(&count, `SELECT COUNT(*) FROM records`); err != nil {
		t.Fatalf("count records: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected rollback to remove records, got %d", count)
	}
}

func TestWithTxReusesNestedTransaction(t *testing.T) {
	manager := setupTxTestDB(t)

	err := manager.WithTx(context.Background(), "app", func(ctx context.Context) error {
		outer, err := manager.Engine(ctx, "app")
		if err != nil {
			return err
		}

		if err := manager.WithTx(ctx, "app", func(nestedCtx context.Context) error {
			nested, err := manager.Engine(nestedCtx, "app")
			if err != nil {
				return err
			}
			if outer != nested {
				t.Fatalf("expected nested transaction executor to be reused")
			}
			_, err = nested.ExecP(`INSERT INTO records (id, name) VALUES (?, ?)`, "1", "nested")
			return err
		}); err != nil {
			return err
		}

		var count int
		if err := outer.GetP(&count, `SELECT COUNT(*) FROM records`); err != nil {
			return err
		}
		if count != 1 {
			t.Fatalf("expected outer transaction to see nested write, got %d", count)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("with nested tx: %v", err)
	}
}

func TestInTxReturnsFalseAfterTransactionEnds(t *testing.T) {
	manager := setupTxTestDB(t)

	var leakedCtx context.Context
	err := manager.WithTx(context.Background(), "app", func(ctx context.Context) error {
		if !InTx(ctx, "app") {
			t.Fatalf("expected active app transaction")
		}
		leakedCtx = ctx
		return nil
	})
	if err != nil {
		t.Fatalf("with tx: %v", err)
	}

	if InTx(leakedCtx, "app") {
		t.Fatalf("expected leaked context to be outside active app transaction")
	}
}

func TestEngineUsesActiveTransactionForDynamicSQL(t *testing.T) {
	manager := setupTxTestDB(t)

	err := manager.WithTx(context.Background(), "app", func(ctx context.Context) error {
		eng, err := manager.Engine(ctx, "app")
		if err != nil {
			return err
		}
		_, err = eng.Exec(
			`INSERT INTO records (id, name) VALUES (:id, :name)`,
			map[string]interface{}{"id": "1", "name": "dynamic"},
		)
		return err
	})
	if err != nil {
		t.Fatalf("with tx: %v", err)
	}

	eng, err := manager.Engine(context.Background(), "app")
	if err != nil {
		t.Fatalf("engine: %v", err)
	}
	var name string
	if err := eng.Get(&name, `SELECT name FROM records WHERE id = :id`, map[string]interface{}{"id": "1"}); err != nil {
		t.Fatalf("get record: %v", err)
	}
	if name != "dynamic" {
		t.Fatalf("expected dynamic record, got %q", name)
	}
}

func TestSQLTxUsesActiveTransaction(t *testing.T) {
	manager := setupTxTestDB(t)

	err := manager.WithTx(context.Background(), "app", func(ctx context.Context) error {
		rawTx, ok := manager.SQLTx(ctx, "app")
		if !ok {
			t.Fatalf("expected raw tx in active app transaction")
		}
		_, err := rawTx.Exec(`INSERT INTO records (id, name) VALUES (?, ?)`, "1", "raw")
		return err
	})
	if err != nil {
		t.Fatalf("with tx: %v", err)
	}

	eng, err := manager.Engine(context.Background(), "app")
	if err != nil {
		t.Fatalf("engine: %v", err)
	}
	var name string
	if err := eng.GetP(&name, `SELECT name FROM records WHERE id = ?`, "1"); err != nil {
		t.Fatalf("get raw tx record: %v", err)
	}
	if name != "raw" {
		t.Fatalf("expected raw tx record, got %q", name)
	}

	if _, ok := manager.SQLTx(context.Background(), "app"); ok {
		t.Fatalf("expected no raw tx outside app transaction")
	}
}

func TestRegisterAfterCommitRunsAfterSuccessfulTx(t *testing.T) {
	manager := setupTxTestDB(t)

	ran := false
	err := manager.WithTx(context.Background(), "app", func(ctx context.Context) error {
		if err := RegisterAfterCommit(ctx, func(runCtx context.Context) {
			if InTx(runCtx, "app") {
				t.Fatalf("expected after-commit context to be outside app transaction")
			}
			ran = true
		}); err != nil {
			return err
		}
		if ran {
			t.Fatalf("after-commit callback ran before commit")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("with tx: %v", err)
	}
	if !ran {
		t.Fatalf("expected after-commit callback to run")
	}
}

func TestRegisterAfterCommitOutsideTxReturnsError(t *testing.T) {
	err := RegisterAfterCommit(context.Background(), func(context.Context) {})
	if !errors.Is(err, ErrNoActiveAppTx) {
		t.Fatalf("expected no active app tx error, got %v", err)
	}
}

func TestRegisterAfterCommitDiscardedOnRollback(t *testing.T) {
	manager := setupTxTestDB(t)
	expectedErr := errors.New("stop")

	ran := false
	err := manager.WithTx(context.Background(), "app", func(ctx context.Context) error {
		if err := RegisterAfterCommit(ctx, func(context.Context) {
			ran = true
		}); err != nil {
			return err
		}
		return expectedErr
	})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected rollback cause %v, got %v", expectedErr, err)
	}
	if ran {
		t.Fatalf("after-commit callback must not run after rollback")
	}
}

func TestNestedAfterCommitRunsAfterOuterCommit(t *testing.T) {
	manager := setupTxTestDB(t)

	calls := []string{}
	err := manager.WithTx(context.Background(), "app", func(ctx context.Context) error {
		if err := RegisterAfterCommit(ctx, func(context.Context) {
			calls = append(calls, "outer")
		}); err != nil {
			return err
		}

		if err := manager.WithTx(ctx, "app", func(nestedCtx context.Context) error {
			if err := RegisterAfterCommit(nestedCtx, func(context.Context) {
				calls = append(calls, "inner")
			}); err != nil {
				return err
			}
			if len(calls) != 0 {
				t.Fatalf("nested after-commit callback ran before outer commit")
			}
			return nil
		}); err != nil {
			return err
		}

		if len(calls) != 0 {
			t.Fatalf("after-commit callback ran before outer commit")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("with nested tx: %v", err)
	}
	if len(calls) != 2 {
		t.Fatalf("expected two after-commit callbacks, got %v", calls)
	}
}

func TestRegisterAfterCommitCannotUseLeakedEndedTxContext(t *testing.T) {
	manager := setupTxTestDB(t)

	var leakedCtx context.Context
	if err := manager.WithTx(context.Background(), "app", func(ctx context.Context) error {
		leakedCtx = ctx
		return nil
	}); err != nil {
		t.Fatalf("with tx: %v", err)
	}

	err := RegisterAfterCommit(leakedCtx, func(context.Context) {})
	if !errors.Is(err, ErrNoActiveAppTx) {
		t.Fatalf("expected no active app tx error, got %v", err)
	}
}

func TestWithTxRejectsSharedDatabase(t *testing.T) {
	manager := setupTxTestDB(t)
	called := false

	err := manager.WithTx(context.Background(), "shared", func(ctx context.Context) error {
		called = true
		return nil
	})
	if !errors.Is(err, ErrTransactionsOnlySupportApp) {
		t.Fatalf("expected app-only transaction error, got %v", err)
	}
	if called {
		t.Fatalf("expected shared transaction callback not to run")
	}
}

func TestNewDBManagerDisablesSQLXGlobalLogging(t *testing.T) {
	original := sqlx.Log.Enabled
	t.Cleanup(func() {
		sqlx.Log.Enabled = original
	})

	sqlx.Log.Enabled = true

	_ = NewDBManager()

	if sqlx.Log.Enabled {
		t.Fatalf("expected NewDBManager to disable sqlx global logging")
	}
}
