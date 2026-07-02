package database

import "github.com/tfnick/go-svelte-starter/api/db"

func Reopen(name string) error {
	return db.ReopenDB(name)
}
