# Error Handling

> How errors are handled in this project.

---

## Overview

Prefer package-level sentinel errors for reusable backend contracts, then map
them to HTTP responses at the route or Echo error-handler boundary.

## Error Types

* `templates.ErrInvalidTemplatePath`: a requested template path is empty,
  traverses upward, or is not an HTML template.
* `templates.ErrTemplateNotFound`: a requested template is missing from both
  the external template root and embedded defaults.
* `forum.ErrThreadNotFound`: a forum thread ID does not exist.
* `forum.ErrInvalidInput`: a required forum form field is missing.

## Error Handling Patterns

Use `errors.Is` to classify sentinel errors. Keep lower-level packages free of
Echo response details; return errors upward and translate them in route handlers
or `router.HTTPErrorHandler`.

## API Error Responses

* Invalid template path -> `400 Bad Request`
* Missing template -> `404 Not Found`
* Missing forum thread -> `404 Not Found` with the forum error fragment
* Invalid forum input -> `400 Bad Request` with the forum error fragment

## Common Mistakes

* Do not directly concatenate user-controlled paths into `c.File`; use the
  template resolver so path validation and fallback behavior are centralized.
* Do not swallow missing external templates as fatal errors; missing external
  files are expected and should fall back to embedded defaults.
