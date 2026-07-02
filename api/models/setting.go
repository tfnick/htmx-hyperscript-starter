package models

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/tfnick/go-svelte-starter/api/db"
	"github.com/tfnick/go-svelte-starter/api/framework/data/modelerror"
	"github.com/tfnick/go-svelte-starter/api/framework/timefmt"
)

type AppSetting struct {
	Key       string `db:"setting_key"`
	ValueJSON string `db:"value_json"`
	CreatedAt string `db:"created_at"`
	UpdatedAt string `db:"updated_at"`
}

func GetAppSetting(ctx context.Context, key string) (AppSetting, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return AppSetting{}, fmt.Errorf("database unavailable: %w", err)
	}

	query := `
		SELECT setting_key, value_json, created_at, updated_at
		FROM app_settings
		WHERE setting_key = ?
		LIMIT 1
	`
	var setting AppSetting
	if err := d.GetP(&setting, query, key); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AppSetting{}, fmt.Errorf("app setting not found: %w", modelerror.ErrNotFound)
		}
		return AppSetting{}, fmt.Errorf("load app setting failed: %w", err)
	}
	return setting, nil
}

func UpsertAppSetting(ctx context.Context, key string, valueJSON string) (AppSetting, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return AppSetting{}, fmt.Errorf("database unavailable: %w", err)
	}

	now := timefmt.NowSQLiteDateTime()
	if valueJSON == "" {
		valueJSON = "{}"
	}
	query := `
		INSERT INTO app_settings (setting_key, value_json, created_at, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(setting_key) DO UPDATE SET
			value_json = excluded.value_json,
			updated_at = excluded.updated_at
	`
	if _, err := d.ExecP(query, key, valueJSON, now, now); err != nil {
		return AppSetting{}, fmt.Errorf("upsert app setting failed: %w", err)
	}
	return GetAppSetting(ctx, key)
}
