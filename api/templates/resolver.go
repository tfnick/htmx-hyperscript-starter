package templates

import (
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

var (
	ErrInvalidTemplatePath = errors.New("invalid template path")
	ErrTemplateNotFound    = errors.New("template not found")
)

type Resolver struct {
	embedded     fs.FS
	externalRoot string
}

type ResolvedTemplate struct {
	Name    string
	Source  string
	Content []byte
}

func NewResolver(embedded fs.FS, externalRoot string) *Resolver {
	return &Resolver{
		embedded:     embedded,
		externalRoot: strings.TrimSpace(externalRoot),
	}
}

func (r *Resolver) Resolve(name string) (*ResolvedTemplate, error) {
	cleanName, err := cleanTemplatePath(name)
	if err != nil {
		return nil, err
	}

	if r.externalRoot != "" {
		content, err := os.ReadFile(filepath.Join(r.externalRoot, filepath.FromSlash(cleanName)))
		switch {
		case err == nil:
			return &ResolvedTemplate{Name: cleanName, Source: "external", Content: content}, nil
		case !errors.Is(err, os.ErrNotExist):
			return nil, fmt.Errorf("read external template %q: %w", cleanName, err)
		}
	}

	content, err := fs.ReadFile(r.embedded, cleanName)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("%w: %s", ErrTemplateNotFound, cleanName)
		}
		return nil, fmt.Errorf("read embedded template %q: %w", cleanName, err)
	}

	return &ResolvedTemplate{Name: cleanName, Source: "embedded", Content: content}, nil
}

func (r *Resolver) Execute(w io.Writer, name string, data any) error {
	resolved, err := r.Resolve(name)
	if err != nil {
		return err
	}

	tmpl, err := template.New(resolved.Name).Parse(string(resolved.Content))
	if err != nil {
		return fmt.Errorf("parse %s template %q: %w", resolved.Source, resolved.Name, err)
	}

	if err := tmpl.Execute(w, data); err != nil {
		return fmt.Errorf("execute %s template %q: %w", resolved.Source, resolved.Name, err)
	}

	return nil
}

func cleanTemplatePath(name string) (string, error) {
	normalized := strings.TrimSpace(strings.ReplaceAll(name, "\\", "/"))
	normalized = strings.TrimPrefix(normalized, "/")
	if normalized == "" {
		return "", ErrInvalidTemplatePath
	}

	for _, segment := range strings.Split(normalized, "/") {
		if segment == ".." {
			return "", fmt.Errorf("%w: %s", ErrInvalidTemplatePath, name)
		}
	}

	cleanName := path.Clean(normalized)
	if cleanName == "." || strings.HasPrefix(cleanName, "../") || strings.HasPrefix(cleanName, "/") {
		return "", fmt.Errorf("%w: %s", ErrInvalidTemplatePath, name)
	}
	if !strings.HasSuffix(cleanName, ".html") {
		return "", fmt.Errorf("%w: %s", ErrInvalidTemplatePath, name)
	}

	return cleanName, nil
}
