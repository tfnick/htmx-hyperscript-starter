# Directory Structure

> How backend code is organized in this project.

---

## Overview

Backend code is organized as small Go packages under `api/`, with `index.go`
reserved for Echo setup, flags, embedded assets, and route mounting.

## Directory Layout

```text
index.go
api/
  forum/       forum domain data and htmx route handlers
  routes/      legacy/simple API route examples
  templates/   HTML template resolution and rendering helpers
  types/       shared API data shapes
public/        embedded default UI templates and assets
```

## Module Organization

Feature-specific backend behavior belongs in a package under `api/<feature>/`.
Keep route handlers and small in-memory/service logic together until there is a
clear need to split service/storage layers. Shared cross-feature helpers belong
in their own package, such as `api/templates`.

`index.go` should only assemble dependencies and register routes. Avoid putting
feature business logic directly in `index.go`.

## Naming Conventions

Use short package names that describe the domain, for example `forum` or
`templates`. Keep HTML templates under `public/` with paths that match their
runtime lookup names.

## Examples

* `api/forum` owns forum threads, replies, and htmx fragment routes.
* `api/templates` owns external-first, embedded-fallback HTML lookup.

## Scenario: Embedded Templates With External Overrides

### 1. Scope / Trigger

Use this contract whenever backend code renders HTML templates or adds a new
configurable template path. The app packages default templates into the Go
executable and lets users override individual HTML files from an external path.

### 2. Signatures

* CLI flag: `--template-path <dir>`
* Resolver constructor: `templates.NewResolver(embedded fs.FS, externalRoot string)`
* Resolver render path: `resolver.Execute(w io.Writer, name string, data any) error`
* Generic component route: `GET /api/components/*`

### 3. Contracts

* Template names are slash-separated paths relative to `public/`, such as
  `index.html` or `components/forum/thread-list.html`.
* Lookup order is external root first, then embedded defaults.
* External templates override only files with the same relative path.
* Missing external templates must fall back to embedded templates.
* HTML should be rendered through `api/templates.Resolver`; do not serve
  `public/**/*.html` directly as static files.

### 4. Validation & Error Matrix

* Empty template name -> `templates.ErrInvalidTemplatePath`
* Path traversal segment (`..`) -> `templates.ErrInvalidTemplatePath`
* Non-HTML template request -> `templates.ErrInvalidTemplatePath`
* Missing in external and embedded filesystems -> `templates.ErrTemplateNotFound`

### 5. Good/Base/Bad Cases

* Good: `components/forum/thread-list.html` exists in `--template-path`, so the
  external version renders.
* Base: `components/forum/thread-detail.html` is not external, so the embedded
  version renders.
* Bad: `../index.html` is rejected before touching the external filesystem.

### 6. Tests Required

* Unit test external template precedence.
* Unit test embedded fallback when the external file is absent.
* Unit test path traversal rejection.
* Unit test missing template errors.

### 7. Wrong vs Correct

#### Wrong

```go
return c.File("public/components/" + component + ".html")
```

This bypasses embedded packaging and ignores `--template-path`.

#### Correct

```go
return resolver.Execute(w, "components/"+component+".html", data)
```

This preserves the external-first, embedded-fallback contract.
