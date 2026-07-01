# HTMX Forum

A lightweight forum project built with Go, Echo, htmx, hyperscript, and Pico.css.

The default HTML templates and static assets are embedded into the Go executable, so a built binary can serve the forum UI without loose template files next to it. You can still point the app at an external template directory to override individual HTML templates without rebuilding.

## Quick Start

```bash
go run .
```

Then open `http://localhost:3000`.

For development with browser reload:

```bash
./dev.sh
```

On Windows:

```bat
dev.bat
```

## Build

```bash
go build -o ./tmp/forum .
./tmp/forum
```

The executable includes the default files under `public/`.

## Template Overrides

Use `--template-path` to provide external HTML templates. Files in this directory take priority over the embedded templates. Any template not found externally falls back to the embedded version.

```bash
./tmp/forum --template-path ./templates
```

Template paths are relative to `public/`. For example, to override the thread list, create:

```text
templates/components/forum/thread-list.html
```

Common templates:

- `index.html`
- `components/forum/thread-list.html`
- `components/forum/thread-detail.html`
- `components/forum/error.html`

## Options

- `--port`: Port to serve the app from. Defaults to `3000`.
- `--dev`: Enables browser hot reload. Defaults to `true`.
- `--template-path`: External HTML template directory. Defaults to embedded templates only.

## Project Shape

- `index.go`: Echo server setup, embedded asset setup, and route mounting.
- `api/forum`: In-memory forum data and htmx routes.
- `api/templates`: Template resolver with external-path override support.
- `public`: Embedded default UI templates and assets.

## Dependencies

- Go
- Echo
- htmx
- hyperscript
- Pico.css
- aarol/reload for browser reload during development
