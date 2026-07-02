package models_test

import (
	"testing"

	"github.com/tfnick/go-svelte-starter/api/db"
	"github.com/tfnick/go-svelte-starter/api/models"
)

func TestAppSettingUpsertAndGet(t *testing.T) {
	setupModelsTestDB(t)

	created, err := models.UpsertAppSetting(t.Context(), "site.logo", `{"object_key":"settings/site-logo.png"}`)
	if err != nil {
		t.Fatalf("upsert app setting: %v", err)
	}
	if created.Key != "site.logo" {
		t.Fatalf("expected setting key, got %q", created.Key)
	}
	if created.CreatedAt == "" || created.UpdatedAt == "" {
		t.Fatalf("expected timestamps, got %#v", created)
	}

	updated, err := models.UpsertAppSetting(t.Context(), "site.logo", `{"object_key":"settings/site-logo.jpg"}`)
	if err != nil {
		t.Fatalf("update app setting: %v", err)
	}
	if updated.ValueJSON != `{"object_key":"settings/site-logo.jpg"}` {
		t.Fatalf("expected updated value, got %q", updated.ValueJSON)
	}

	var count int
	appDB, err := db.DefaultManager.GetDB("app")
	if err != nil {
		t.Fatalf("get app db: %v", err)
	}
	if err := appDB.Get(&count, `SELECT COUNT(*) FROM app_settings WHERE setting_key = 'site.logo'`); err != nil {
		t.Fatalf("count settings: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one setting row, got %d", count)
	}
}
