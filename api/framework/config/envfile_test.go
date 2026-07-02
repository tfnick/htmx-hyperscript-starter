package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadEnvFilesLoadsDotenvValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	content := "\ufeff" + `
# comment
ENVFILE_TEST_ALPHA=plain
ENVFILE_TEST_SPACED = value with spaces # comment
export ENVFILE_TEST_EXPORTED=enabled
ENVFILE_TEST_DOUBLE="http://127.0.0.1:5173/api"
ENVFILE_TEST_QUOTED_COMMENT="quoted value" # comment
ENVFILE_TEST_BACKSLASH="C:\oauth\client"
ENVFILE_TEST_SINGLE='abc#def'
`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("write env file: %v", err)
	}

	keys := []string{
		"ENVFILE_TEST_ALPHA",
		"ENVFILE_TEST_SPACED",
		"ENVFILE_TEST_EXPORTED",
		"ENVFILE_TEST_DOUBLE",
		"ENVFILE_TEST_QUOTED_COMMENT",
		"ENVFILE_TEST_BACKSLASH",
		"ENVFILE_TEST_SINGLE",
	}
	for _, key := range keys {
		os.Unsetenv(key)
		defer os.Unsetenv(key)
	}

	results, err := LoadEnvFiles(path)
	if err != nil {
		t.Fatalf("load env file: %v", err)
	}
	if len(results) != 1 || results[0].Assigned != len(keys) {
		t.Fatalf("unexpected load results: %#v", results)
	}

	expected := map[string]string{
		"ENVFILE_TEST_ALPHA":          "plain",
		"ENVFILE_TEST_SPACED":         "value with spaces",
		"ENVFILE_TEST_EXPORTED":       "enabled",
		"ENVFILE_TEST_DOUBLE":         "http://127.0.0.1:5173/api",
		"ENVFILE_TEST_QUOTED_COMMENT": "quoted value",
		"ENVFILE_TEST_BACKSLASH":      `C:\oauth\client`,
		"ENVFILE_TEST_SINGLE":         "abc#def",
	}
	for key, want := range expected {
		if got := os.Getenv(key); got != want {
			t.Fatalf("unexpected value for %s: got %q want %q", key, got, want)
		}
	}
}

func TestLoadEnvFilesDoesNotOverrideExistingEnv(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("ENVFILE_TEST_EXISTING=file\n"), 0600); err != nil {
		t.Fatalf("write env file: %v", err)
	}

	t.Setenv("ENVFILE_TEST_EXISTING", "system")

	results, err := LoadEnvFiles(path)
	if err != nil {
		t.Fatalf("load env file: %v", err)
	}
	if len(results) != 1 || results[0].Assigned != 0 {
		t.Fatalf("unexpected load results: %#v", results)
	}
	if got := os.Getenv("ENVFILE_TEST_EXISTING"); got != "system" {
		t.Fatalf("expected existing env value to win, got %q", got)
	}
}

func TestLoadEnvFilesSkipsMissingAndDuplicateFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("ENVFILE_TEST_DEDUP=loaded\n"), 0600); err != nil {
		t.Fatalf("write env file: %v", err)
	}
	os.Unsetenv("ENVFILE_TEST_DEDUP")
	defer os.Unsetenv("ENVFILE_TEST_DEDUP")

	results, err := LoadEnvFiles(filepath.Join(t.TempDir(), "missing.env"), path, path)
	if err != nil {
		t.Fatalf("load env files: %v", err)
	}
	if len(results) != 1 || results[0].Assigned != 1 {
		t.Fatalf("unexpected load results: %#v", results)
	}
	if got := os.Getenv("ENVFILE_TEST_DEDUP"); got != "loaded" {
		t.Fatalf("unexpected env value: %q", got)
	}
}

func TestLoadEnvFilesRejectsInvalidLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("NOT_A_PAIR\n"), 0600); err != nil {
		t.Fatalf("write env file: %v", err)
	}

	if _, err := LoadEnvFiles(path); err == nil {
		t.Fatalf("expected invalid env file to fail")
	}
}
