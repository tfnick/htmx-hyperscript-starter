package main

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestLoadTemplateFallsBackToEmbeddedTemplate(t *testing.T) {
	files := fstest.MapFS{
		"index.html": {Data: []byte("embedded")},
	}

	body, err := loadTemplate(files, "", "index.html")
	if err != nil {
		t.Fatalf("loadTemplate returned error: %v", err)
	}
	if string(body) != "embedded" {
		t.Fatalf("expected embedded template, got %q", string(body))
	}
}

func TestLoadTemplatePrefersExternalTemplate(t *testing.T) {
	files := fstest.MapFS{
		"index.html": {Data: []byte("embedded")},
	}
	externalRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(externalRoot, "index.html"), []byte("external"), 0o600); err != nil {
		t.Fatalf("write external template: %v", err)
	}

	body, err := loadTemplate(files, externalRoot, "index.html")
	if err != nil {
		t.Fatalf("loadTemplate returned error: %v", err)
	}
	if string(body) != "external" {
		t.Fatalf("expected external template override, got %q", string(body))
	}
}

func TestLoadTemplateRejectsUnsafeTemplateNames(t *testing.T) {
	files := fstest.MapFS{
		"index.html": {Data: []byte("embedded")},
	}
	for _, name := range []string{"", "/index.html", "../index.html", "components/../index.html", "styles.css"} {
		t.Run(name, func(t *testing.T) {
			if _, err := loadTemplate(files, "", name); err == nil {
				t.Fatalf("expected %q to be rejected", name)
			}
		})
	}
}
