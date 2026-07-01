# Quality Guidelines

> Code quality standards for backend development.

---

## Overview

Backend changes should keep the app buildable as a single Go executable and
preserve the existing Go/Echo/htmx/hyperscript/Pico stack unless a task
explicitly changes that constraint.

## Forbidden Patterns

* Do not add a Node or frontend build pipeline for ordinary UI changes.
* Do not serve HTML templates directly from `public/` with `c.File` or broad
  static routes; route HTML through `api/templates.Resolver`.
* Do not introduce path traversal risks when resolving externally configurable
  template files.

## Required Patterns

* Run `gofmt` on changed Go files.
* Keep default HTML templates and assets embeddable with Go's `embed` package.
* Use external-template overrides as an additive extension point: external file
  present means override, external file absent means fallback to embedded.

## Testing Requirements

* Run `go test ./...` for backend changes.
* Run `go build` when embedding assets, changing flags, or changing route setup.
* Add focused unit tests for reusable resolver or validation behavior.

## Code Review Checklist

* Does the app still build into a self-contained executable?
* Are HTML template paths validated and rendered through the resolver?
* Does external template override behavior still fall back to embedded defaults?
* Are htmx routes returning fragments with appropriate HTTP status codes?
