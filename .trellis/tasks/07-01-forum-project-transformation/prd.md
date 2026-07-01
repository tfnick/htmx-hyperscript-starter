# Forum Project Transformation

## Goal

Transform the existing Go + Echo + htmx + hyperscript starter into a forum project while keeping the current technology stack. The application should remain a lightweight Go executable, and HTML templates should be bundled into the executable by default while still allowing users to override or extend templates from a configured external path.

## What I Already Know

* The user wants a Trellis task created before implementation.
* The existing technology stack must stay in place: Go, Echo, htmx, hyperscript, Pico.css, and the current static/template style.
* The current app serves `public/` directly and loads HTML fragments from `public/components/*.html` through `/api/components/*`.
* The current backend has a small Echo server in `index.go`, one sample API route in `api/routes/user.go`, and simple API types in `api/types/types.go`.
* Packaging must embed HTML templates into the executable by default.
* Users must be able to configure an external template path.
* If an external template path is configured, templates from that path must take priority over embedded templates.
* The purpose of the external path is to let users extend or customize HTML templates without rebuilding the executable.

## Requirements

* Preserve the existing technology stack; do not introduce a new frontend framework, template engine, or separate Node build pipeline.
* Replace the starter/demo experience with a forum-oriented experience.
* Add forum domain concepts suitable for an MVP:
  * Forum home / thread list.
  * Thread detail view.
  * Basic post/reply display.
  * Simple create-thread or create-reply flow using htmx.
* Keep the UI compatible with the current htmx component loading pattern unless implementation discovery finds a simpler project-local equivalent.
* Package HTML templates into the Go executable by default, likely using Go's standard `embed` support.
* Add a configurable external template root, such as a CLI flag or environment variable.
* Template lookup order must be:
  1. External template root, when configured and the requested template exists there.
  2. Embedded template filesystem.
* Missing external templates must gracefully fall back to embedded templates.
* Missing templates from both locations should return a clear HTTP error and log enough detail for debugging.
* Preserve development hot reload behavior where practical.
* Document how to build the executable and how to use the external template path override.

## Acceptance Criteria

* [ ] The app runs as a forum UI instead of the current starter/demo UI.
* [ ] `go build` produces an executable that can serve the forum HTML templates without needing loose `public/**/*.html` template files beside the executable.
* [ ] Static assets needed by the default UI are still served correctly after packaging.
* [ ] A configured external template directory can override an embedded template with the same relative path.
* [ ] Templates not present in the external directory still fall back to the embedded version.
* [ ] The override behavior is covered by focused tests or a small verifiable integration path.
* [ ] README or equivalent docs explain the new forum app, build command, and external template path option.
* [ ] Lint, formatting, and tests/checks pass.

## Definition of Done

* Requirements are implemented without changing the existing technology stack.
* Tests are added or updated for the template resolution behavior.
* `go test ./...` passes.
* `go build` passes.
* Documentation covers normal embedded-template packaging and external-template override usage.
* The task is reviewed against the relevant Trellis backend spec context before completion.

## Technical Approach

Use a small template/static asset resolver around Go's `embed.FS` and optional `os.DirFS` external template root. Route handlers should ask this resolver for templates by relative path, allowing the resolver to centralize precedence rules and error handling.

Recommended implementation direction:

* Embed default HTML templates and default assets from `public/` into the executable.
* Add a CLI flag such as `--template-path` for external template overrides.
* Optionally support an environment variable if it matches the project's configuration style after code inspection.
* Replace direct `c.File("public/components/" + component + ".html")` access with a resolver-backed response path.
* Keep htmx fragment endpoints for forum components to minimize stack churn.
* Keep external template override paths relative and cleaned, preventing path traversal outside the configured root.

## Decision (ADR-lite)

**Context**: The project currently depends on loose HTML files under `public/`. The new forum executable should work out of the box after `go build`, but users also need a customization path for templates.

**Decision**: Prefer a resolver that checks an optional external directory first, then falls back to embedded defaults. Use Go standard library filesystem abstractions where possible.

**Consequences**: The executable becomes self-contained for default templates, while advanced users can customize templates without rebuilding. Implementation must carefully normalize template paths and preserve clear fallback behavior.

## Out of Scope

* Replacing htmx/hyperscript/Pico.css with another frontend stack.
* Adding a database-backed production forum unless the implementation phase explicitly scopes it in.
* Full authentication, moderation, permissions, notifications, search, or pagination beyond what is needed for a forum MVP.
* A plugin system beyond external HTML template override.
* Multi-tenant template resolution.

## Open Questions

* Forum data persistence for the MVP needs confirmation: in-memory sample data, file-backed JSON, or a database. The current recommendation is in-memory/sample data first unless the user wants persistence in this task.
* External override scope needs confirmation: HTML templates only, or HTML templates plus CSS/JS/assets.

## Technical Notes

* Repo inspection date: 2026-07-01.
* Current entry point: `index.go`.
* Current API route example: `api/routes/user.go`.
* Current types package: `api/types/types.go`.
* Current static/templates root: `public/`.
* Current component pattern: `/api/components/*` maps to `public/components/<name>.html`.
* Current build target: `go build -o ./tmp/ .` from `Makefile`.
* Relevant spec context starts at `.trellis/spec/backend/index.md`.
