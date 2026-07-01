package templates

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestResolverUsesExternalTemplateBeforeEmbedded(t *testing.T) {
	externalRoot := t.TempDir()
	writeTemplate(t, externalRoot, "components/forum/thread-list.html", "external")

	resolver := NewResolver(fstest.MapFS{
		"components/forum/thread-list.html": {Data: []byte("embedded")},
	}, externalRoot)

	resolved, err := resolver.Resolve("components/forum/thread-list.html")
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if string(resolved.Content) != "external" {
		t.Fatalf("expected external template, got %q", resolved.Content)
	}
	if resolved.Source != "external" {
		t.Fatalf("expected external source, got %q", resolved.Source)
	}
}

func TestResolverFallsBackToEmbeddedTemplate(t *testing.T) {
	resolver := NewResolver(fstest.MapFS{
		"index.html": {Data: []byte("embedded index")},
	}, t.TempDir())

	resolved, err := resolver.Resolve("index.html")
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if string(resolved.Content) != "embedded index" {
		t.Fatalf("expected embedded template, got %q", resolved.Content)
	}
	if resolved.Source != "embedded" {
		t.Fatalf("expected embedded source, got %q", resolved.Source)
	}
}

func TestResolverRejectsPathTraversal(t *testing.T) {
	resolver := NewResolver(fstest.MapFS{
		"index.html": {Data: []byte("embedded index")},
	}, t.TempDir())

	_, err := resolver.Resolve("../index.html")
	if !errors.Is(err, ErrInvalidTemplatePath) {
		t.Fatalf("expected ErrInvalidTemplatePath, got %v", err)
	}
}

func TestResolverReportsMissingTemplate(t *testing.T) {
	resolver := NewResolver(fstest.MapFS{}, "")

	_, err := resolver.Resolve("missing.html")
	if !errors.Is(err, ErrTemplateNotFound) {
		t.Fatalf("expected ErrTemplateNotFound, got %v", err)
	}
}

func TestResolverExecutesHTMLTemplate(t *testing.T) {
	resolver := NewResolver(fstest.MapFS{
		"greeting.html": {Data: []byte("<p>Hello, {{.Name}}</p>")},
	}, "")

	var out bytes.Buffer
	if err := resolver.Execute(&out, "greeting.html", struct{ Name string }{"Forum"}); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	if out.String() != "<p>Hello, Forum</p>" {
		t.Fatalf("unexpected rendered output: %s", out.String())
	}
}

func writeTemplate(t *testing.T, root, name, content string) {
	t.Helper()

	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
}
