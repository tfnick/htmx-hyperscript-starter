# Logging Guidelines

> How logging is done in this project.

---

## Overview

The project does not use a structured logging library. It currently uses:

* `fmt.Println` / `fmt.Printf` for startup and development-only messages in `index.go`.
* Echo's logger for server startup, fatal server errors, and errors that occur while writing error responses.

Echo request logging middleware is intentionally commented out in `index.go`.
Do not enable broad request logging unless a task asks for it.

## Log Levels

There is no project-level log-level abstraction yet. Use these current patterns:

* Startup/info: `fmt.Printf("Listening on port %s\n", *port)`
* Development-only component fetches: guard with `if *isDevelopment`
* Response-write failures inside the HTTP error handler: `c.Logger().Error(handleErr)`
* Fatal server startup/runtime error: `router.Logger.Fatal(router.Start(...))`

## Structured Logging

Structured logging is not configured. If a future task adds it, document:

* Logger package and initialization location.
* Required fields for requests and errors.
* Whether logs are JSON, text, or Echo defaults.
* How dev/prod log levels are configured.

## What to Log

* Server startup port.
* Hot reload enabled state in development.
* Component template requests only in development mode.
* Errors encountered while writing HTTP error responses.

## What NOT to Log

* Do not log form bodies or reply/thread text by default.
* Do not log secrets, future auth tokens, cookies, or personal data.
* Do not leave noisy debug output outside `if *isDevelopment` guards.

## Common Mistakes

* Avoid adding `fmt.Println` in reusable packages such as `api/forum` or `api/templates`.
  Return errors and let the route or server boundary decide whether to log.
* Avoid enabling Echo's request logger while also adding ad hoc request prints;
  duplicated logs make htmx interactions noisy.
