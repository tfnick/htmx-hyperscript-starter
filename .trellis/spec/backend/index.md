# Backend Development Guidelines

> Best practices for backend development in this project.

---

## Overview

This project is a small Go 1.22 application using Echo for HTTP routing, htmx
fragments for UI interactions, hyperscript for light browser behavior, Pico.css
for styling, and Go `embed` for packaging default UI assets into the executable.

The backend is intentionally compact: `index.go` wires the server together,
feature packages under `api/` own route handlers and in-memory domain behavior,
and `api/templates` owns the HTML template lookup contract.

## Pre-Development Checklist

Before writing backend code:

* Read [Directory Structure](./directory-structure.md) for package ownership and template layout.
* Read [Error Handling](./error-handling.md) when adding routes, sentinel errors, or validation.
* Read [Quality Guidelines](./quality-guidelines.md) for build/test requirements.
* Read [Database Guidelines](./database-guidelines.md) before adding persistence.
* Read [Logging Guidelines](./logging-guidelines.md) before adding new output or request logging.

## Quality Check

Before finishing backend work:

* Run `gofmt` on changed Go files.
* Run `go test ./...`.
* Run `go build` when routes, flags, embedded assets, or templates changed.
* Verify HTML is rendered through `api/templates.Resolver` when external overrides should apply.
* Confirm any new route has clear success, validation, and not-found behavior.

## Guidelines Index

| Guide | Description | Status |
|-------|-------------|--------|
| [Directory Structure](./directory-structure.md) | Module organization, route ownership, template layout, and embedded-template contract | Filled |
| [Database Guidelines](./database-guidelines.md) | Current no-database state and rules for future persistence | Filled |
| [Error Handling](./error-handling.md) | Sentinel errors, route-level mapping, and template/forum error responses | Filled |
| [Quality Guidelines](./quality-guidelines.md) | Code standards, forbidden patterns, tests, and review checklist | Filled |
| [Logging Guidelines](./logging-guidelines.md) | Current stdout/Echo logging behavior and constraints | Filled |

## Current Runtime Contracts

* CLI flags:
  * `--port` defaults to `3000`.
  * `--dev` defaults to `true`.
  * `--template-path` defaults to embedded templates only.
* Default templates and static assets live under `public/` and are embedded with `//go:embed public`.
* External HTML templates override embedded templates by matching paths relative to `public/`.
* Forum htmx routes live under `/api/forum`.
* Generic component templates can still be requested through `/api/components/*`.

## Language

All documentation in `.trellis/spec/` should be written in English.
