package usecase_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/tfnick/go-svelte-starter/api/db"
	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
)

var errStop = errors.New("stop")

func TestWithAppTxPreservesUsecaseContextAndRollsBack(t *testing.T) {
	manager := usecaseTestDB(t)
	appDB, err := manager.GetDB("app")
	if err != nil {
		t.Fatalf("get app db: %v", err)
	}
	if _, err := appDB.Exec(`CREATE TABLE records (id TEXT PRIMARY KEY, name TEXT NOT NULL)`); err != nil {
		t.Fatalf("create table: %v", err)
	}

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	ctx.RequestID = "req-1"
	ctx.Actor.Authenticated = true
	ctx.Actor.UserID = "u1"

	err = fwusecase.WithAppTx(ctx, func(txCtx fwusecase.Context) error {
		if txCtx.RequestID != ctx.RequestID {
			t.Fatalf("expected request id to be preserved")
		}
		if txCtx.Actor.UserID != ctx.Actor.UserID {
			t.Fatalf("expected actor to be preserved")
		}
		eng, err := db.EngineFor(txCtx.Std(), "app")
		if err != nil {
			return err
		}
		if _, err := eng.ExecP(`INSERT INTO records (id, name) VALUES (?, ?)`, "1", "rolled back"); err != nil {
			return err
		}
		return errStop
	})
	if err != errStop {
		t.Fatalf("expected callback error, got %v", err)
	}

	var count int
	if err := appDB.Get(&count, `SELECT COUNT(*) FROM records`); err != nil {
		t.Fatalf("count records: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected rollback, found %d records", count)
	}
}

func TestRegisterAfterCommitOutsideAppTxReturnsError(t *testing.T) {
	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceSystem)

	err := fwusecase.RegisterAfterCommit(ctx, func(context.Context) {})
	if !errors.Is(err, fwusecase.ErrNoActiveAppTx) {
		t.Fatalf("expected no active app tx error, got %v", err)
	}
	if fwusecase.InAppTx(ctx) {
		t.Fatalf("expected context to be outside app transaction")
	}
}

func TestAfterCommitRunsAfterSuccessfulAppTx(t *testing.T) {
	usecaseTestDB(t)

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceSystem)
	ran := false
	err := fwusecase.WithAppTx(ctx, func(txCtx fwusecase.Context) error {
		if !fwusecase.InAppTx(txCtx) {
			t.Fatalf("expected active app transaction")
		}
		if err := fwusecase.RegisterAfterCommit(txCtx, func(runCtx context.Context) {
			if fwusecase.InAppTx(ctx.WithStd(runCtx)) {
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
		t.Fatalf("with app tx: %v", err)
	}
	if !ran {
		t.Fatalf("expected after-commit callback to run")
	}
}

func TestAfterCommitCannotRegisterAfterTransactionEnds(t *testing.T) {
	usecaseTestDB(t)

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceSystem)
	var leakedCtx fwusecase.Context
	if err := fwusecase.WithAppTx(ctx, func(txCtx fwusecase.Context) error {
		leakedCtx = txCtx
		return nil
	}); err != nil {
		t.Fatalf("with app tx: %v", err)
	}

	if fwusecase.InAppTx(leakedCtx) {
		t.Fatalf("expected leaked context to be inactive after transaction ends")
	}

	err := fwusecase.RegisterAfterCommit(leakedCtx, func(context.Context) {})
	if !errors.Is(err, fwusecase.ErrNoActiveAppTx) {
		t.Fatalf("expected no active app tx error, got %v", err)
	}
}

func TestAfterCommitDiscardedOnRollback(t *testing.T) {
	usecaseTestDB(t)

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceSystem)
	ran := false
	err := fwusecase.WithAppTx(ctx, func(txCtx fwusecase.Context) error {
		if err := fwusecase.RegisterAfterCommit(txCtx, func(context.Context) {
			ran = true
		}); err != nil {
			return err
		}
		return errStop
	})
	if err != errStop {
		t.Fatalf("expected callback error, got %v", err)
	}
	if ran {
		t.Fatalf("after-commit callback must not run after rollback")
	}
}

func TestNestedAfterCommitRunsAfterOuterCommit(t *testing.T) {
	usecaseTestDB(t)

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceSystem)
	calls := []string{}
	err := fwusecase.WithAppTx(ctx, func(outerCtx fwusecase.Context) error {
		if err := fwusecase.RegisterAfterCommit(outerCtx, func(context.Context) {
			calls = append(calls, "outer")
		}); err != nil {
			return err
		}

		if err := fwusecase.WithAppTx(outerCtx, func(innerCtx fwusecase.Context) error {
			if err := fwusecase.RegisterAfterCommit(innerCtx, func(context.Context) {
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
		t.Fatalf("with nested app tx: %v", err)
	}
	if len(calls) != 2 {
		t.Fatalf("expected two after-commit callbacks, got %v", calls)
	}
}

func usecaseTestDB(t *testing.T) *db.DBManager {
	t.Helper()

	previous := db.DefaultManager
	manager := db.NewDBManager()
	db.DefaultManager = manager
	dir := t.TempDir()
	t.Cleanup(func() {
		_ = manager.Close()
		db.DefaultManager = previous
	})

	if err := manager.Open("app", "sqlite", filepath.Join(dir, "app.db")); err != nil {
		t.Fatalf("open app db: %v", err)
	}
	return manager
}
