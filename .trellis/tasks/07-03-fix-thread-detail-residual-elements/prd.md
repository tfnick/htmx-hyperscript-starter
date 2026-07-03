# Fix Thread Detail Residual Elements

## Goal

Fix the forum thread detail page so the center content area contains only thread-detail HTML when a post route is active. List-mode elements such as `#thread-list` and `#composer-panel` must not remain in the detail view DOM above or beside `#thread-detail`.

## What I Already Know

* The user reported that the thread detail page still contains HTML elements outside the thread detail, including `id="thread-list"` and `id="composer-panel"`.
* Current `public/index.html` statically renders list-mode sections and `#thread-detail` as siblings inside `.feed-panel`.
* A runtime DOM-swap approach was rejected by the user: list and detail should be different HTML template pages, not one template that creates/removes inner content.
* Previous work added `/post-{thread id}-{post page}` URL handling and a detail-mode switch.

## Requirements

* The forum list and thread detail must be separate HTML templates.
* `/` and `/categories/:slug` should render the list template.
* `/post-{thread id}-{post page}` should render the detail template.
* When viewing a thread detail route, the rendered template must not contain list-mode DOM nodes such as:
  * `#thread-list`
  * `#composer-panel`
  * `.feed-toolbar`
  * `.board-heading`
* Thread detail mode should render the post body and first-page replies using the existing thread detail API response.
* List mode should still render toolbar, board heading, optional composer, pagination, and thread list.
* Navigating from detail mode back to `/` or `/categories/:slug` should load/render the list template and preserve existing list behavior.
* Clicking a thread should keep using `/post-{thread id}-1`.
* Directly loading `/post-{thread id}-{post page}` should render the detail template directly.
* Auth, login/register/logout, thread creation, reply submission, category selection, search, sort, and pagination should remain usable.
* `/login` should render a dedicated `login.html` template with the shared outer layout, and top-right login links should navigate to it.

## Acceptance Criteria

* [ ] On a post detail page, `document.querySelector("#thread-list")` is `null`.
* [ ] On a post detail page, `document.querySelector("#composer-panel")` is `null`.
* [ ] On a post detail page, the center content contains `#thread-detail` with the selected thread and reply list.
* [ ] Clicking a thread changes the path to `/post-{thread id}-1`.
* [ ] Browser back to `/` loads the list template with `#thread-list`, list toolbar, and other list controls.
* [ ] Category/search/sort/pagination flows still work after returning to list mode.
* [ ] Clicking the top-right login link renders `login.html` instead of relying on an inline sidebar form.
* [ ] `node --check public/extensions.js` passes.

## Definition of Done

* Code follows existing frontend style and avoids unnecessary backend changes.
* Focused browser or DOM verification confirms detail-only DOM and list restoration.
* Available project checks are run and results documented.
* Changes are committed before Trellis finish-work.

## Technical Approach

Use route-level template selection. Keep `public/index.html` as the list page. Add `public/post.html` as the detail page. Update frontend routing so list clicks navigate to `/post-{id}-1`; the post template loads the selected thread detail from the existing API.

## Decision (ADR-lite)

**Context**: The previous implementation hid list nodes in detail mode. The user explicitly requires those nodes not to exist in the detail page HTML.

**Decision**: Use separate server-rendered HTML templates for list and detail pages. Keep outer layout visually similar for now, but each page only includes the layout/content nodes it needs.

**Consequences**: DOM assertions match the user expectation at template/source level. Shared frontend JS must be null-safe because not every page has the same controls.

## Out of Scope

* Backend forum API redesign.
* Reply pagination beyond the existing first-page/detail response.
* Redesigning sidebars or top navigation.

## Technical Notes

* Main files: `public/index.html`, `public/post.html`, `public/login.html`, `public/extensions.js`, `index.go`.
* Existing route shell handling for `/post-*` is in `index.go` and should render `post.html`.
