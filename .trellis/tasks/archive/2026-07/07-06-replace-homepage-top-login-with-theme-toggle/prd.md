# Replace Homepage Top Login With Theme Toggle

## Goal

Remove the homepage top navigation login button and replace it with a light/dark mode prototype toggle button.

## Requirements

- Only change the homepage top navigation, not the login/register/post/new-post pages.
- Remove the top `登录` link from `public/index.html`.
- Add one circular non-navigation light/dark prototype button in the same topbar slot.
- Keep the control keyboard accessible with `button type="button"` and an accessible label.
- Style the control to fit the existing compact DeepFlood topbar without adding dependencies.
- Do not implement persistent theme switching or global dark-mode tokens in this task.

## Acceptance Criteria

- [ ] Homepage topbar no longer contains a `top-login-link` anchor to `/login`.
- [ ] Homepage topbar displays a light/dark prototype toggle control.
- [ ] The new control does not submit the search form or navigate.
- [ ] The control fits desktop and mobile topbar layouts without overflow.
- [ ] Existing root Go template tests pass.

## Definition of Done

- Frontend specs were read.
- Changes are scoped to homepage HTML/CSS.
- Relevant tests/checks are run and results are recorded.
- Existing unrelated Trellis task directories are not modified.

## Technical Approach

Replace the homepage-only `<a class="top-login-link" href="/login">登录</a>` with a single circular `.theme-toggle-prototype` icon button. Add CSS near the existing topbar button styles and update the `topbar` grid to reserve a stable compact width.

## Decision (ADR-lite)

Context: The request asks for a prototype button rather than full theme behavior.

Decision: Implement a static, accessible prototype control in HTML/CSS only.

Consequences: The UI communicates the intended light/dark feature without introducing JS state, storage, or global theme tokens. Full mode switching can be a later task.

## Out of Scope

- No actual light/dark theme switching.
- No localStorage, media query syncing, or app-wide color token refactor.
- No changes to login/register/post/new-post topbar links.

## Technical Notes

- Relevant files: `public/index.html`, `public/styles.css`.
- `rg` found `.top-login-link` usage in CSS and other pages, but homepage top login is not referenced by `extensions.js`.
