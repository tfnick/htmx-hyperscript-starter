package usecase

import (
	"context"

	"github.com/tfnick/go-svelte-starter/api/db"
)

var ErrNoActiveAppTx = db.ErrNoActiveAppTx

type AfterCommitFunc = db.AfterCommitFunc

// WithAppTx runs fn with an app transaction attached to the usecase context.
func WithAppTx(ctx Context, fn func(Context) error) error {
	return db.WithTx(ctx.Std(), "app", func(txCtxStd context.Context) error {
		return fn(ctx.WithStd(txCtxStd))
	})
}

func InAppTx(ctx Context) bool {
	return db.InTx(ctx.Std(), "app")
}

func RegisterAfterCommit(ctx Context, fn AfterCommitFunc) error {
	return db.RegisterAfterCommit(ctx.Std(), fn)
}
