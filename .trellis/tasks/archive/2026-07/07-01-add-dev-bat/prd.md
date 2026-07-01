# Add dev.bat

## Goal

Add a Windows development script that mirrors the existing Unix `dev.sh` flow so developers on Windows can start the Go/Echo forum app without translating shell syntax.

## Requirements

* Add `dev.bat` at the repository root.
* Keep the existing technology stack unchanged.
* Prefer `gow -e=go run . --dev` when `gow` is available, matching `dev.sh`.
* Fall back to `go run . --dev` with a clear message when `gow` is not available.
* Do not modify application runtime behavior.

## Acceptance Criteria

* [x] `dev.bat` exists at the repository root.
* [x] Running `dev.bat` starts the app in development mode.
* [x] If `gow` is installed, the script uses hot-reload behavior.
* [x] If `gow` is missing, the script still starts the app with `go run . --dev`.
* [x] `go test ./...` passes.
* [x] `go build` passes.

## Definition of Done

* Script is committed with task metadata.
* Existing Unix `dev.sh` remains unchanged.
* No unrelated files are modified.

## Technical Notes

* Existing Unix script: `dev.sh`
* Existing command: `gow -e=go run . --dev`
* The app's `--dev` flag defaults to true, but the script passes it explicitly to match `dev.sh`.
