# NodeSeek-Style Forum Home and Category Boards

## Goal

Implement a NodeSeek-inspired forum index experience for the existing Go/Echo/htmx forum app. The home page and category boards should match the provided reference layout and visual density: a slim top navigation, floating left category rail, central compact thread feed, and right-side community/action/advertising column.

## What I Already Know

* The user provided a screenshot of the desired layout and style.
* The target screen resembles NodeSeek: white rounded content panels over a subtle grid background, compact Chinese forum navigation, dense thread rows, and a right sidebar.
* Existing app entrypoint is `public/index.html`.
* Existing frontend behavior is in `public/extensions.js`.
* Existing styles are in `public/styles.css`.
* Existing forum APIs already support:
  * listing categories via `GET /api/forum/categories`
  * listing threads by category via `GET /api/forum/categories/:slug/threads`
  * search, sort, page, and page size query parameters
  * loading thread detail via `GET /api/forum/threads/:id`
  * auth-aware thread/reply creation
* Current seed data only creates the `daily` category, so this task should expand backend seed data to make the visible boards real forum categories.

## Requirements

* Redesign the home page to follow the reference layout:
  * sticky top header with `NodeSeek`-style brand, board links, and search box
  * left floating category rail with icons and labels
  * central rounded main content panel with thread tabs, pagination, and compact thread list
  * right sidebar with guest login/register card, quick feature links, and stacked sponsor/ad blocks
* Support board/category display for these visible sections: Daily, Tech, Info, Review, Trade, Carpool, Promotion, Life, Dev, Image, Exposure, Sandbox.
* Keep category selection functional with existing forum APIs:
  * visiting `/` loads a cross-category thread feed
  * visiting `/categories/:slug` loads only that category's threads
  * clicking a category updates the URL to `/categories/:slug`
  * every visible board should be backed by a real seeded forum category
  * selecting a category loads that category's real threads
  * categories without threads show a polished empty state instead of falling back silently
* Seed the visible boards as real backend categories: daily, tech, info, review, trade, carpool, promotion, life, dev, image, exposure, sandbox.
* Render thread rows with the information density shown in the screenshot:
  * avatar or generated placeholder
  * title with optional badges such as pinned/read-only
  * author, views, replies, last poster, relative time
  * category pill aligned to the right
* Preserve existing auth flows:
  * guest card exposes login and register actions
  * logged-in users can still create threads and replies
* Preserve search and pagination behavior.
* Make the layout responsive:
  * desktop keeps the three-column structure
  * tablet/mobile collapses sidebars without overlap
  * all text and buttons fit within their containers
* Use restrained, production-like UI styling instead of a marketing landing page.

## Acceptance Criteria

* [ ] Visiting `/` shows a NodeSeek-inspired forum index rather than the current dashboard/detail split layout.
* [ ] Visiting `/` renders a thread feed spanning multiple categories.
* [ ] Visiting `/categories/tech` and other category paths renders only that category's thread list.
* [ ] Clicking category buttons updates browser history to `/categories/<slug>`.
* [ ] Header, left category rail, central feed, and right sidebar are all visible on desktop widths.
* [ ] Category buttons update the active board state and trigger the thread list load.
* [ ] Existing real forum data renders in the compact feed.
* [ ] All visible boards exist as backend categories after migration/seed.
* [ ] Empty categories show a polished empty state instead of errors or broken markup.
* [ ] Login/register/logout behavior remains usable.
* [ ] Thread creation and reply behavior still work for authenticated users.
* [ ] Search and pagination continue to call the existing API parameters.
* [ ] The page is usable at common mobile widths without incoherent overlap.
* [ ] `go test ./...` or the closest practical project verification is run and documented.

## Definition of Done

* Tests added or updated where behavior changes require it.
* Go files, if changed, are formatted with `gofmt`.
* Frontend files are manually checked in-browser at desktop and mobile widths.
* Lint/type/build/test commands that are available for this project are run.
* Task context is curated before implementation.

## Technical Approach

Prefer a frontend-first implementation using the existing API surface:

* Update `public/index.html` for the new page structure.
* Update `public/styles.css` for the reference layout, grid background, compact panels, thread rows, sidebars, and responsive behavior.
* Update `public/extensions.js` to render board navigation, feed rows, right-side state, and graceful empty states while keeping the existing API calls.
* Add or update migration seed data for the visible forum categories while preserving existing category rows.

## Decision (ADR-lite)

**Context**: The user asked for a layout/style implementation based on a screenshot. The current backend already has enough forum APIs for category/thread/search/pagination flows.

**Decision**: Start with a frontend/template redesign that reuses current API contracts and expand seed data so all visible board names are real backend categories.

**Consequences**: Category navigation will behave consistently for every board. The task touches both frontend files and database seed/migration data, so verification should include app startup or tests that exercise migrations.

## Out of Scope

* Exact clone of NodeSeek branding, logo assets, or third-party advertisements.
* Real ad network integration.
* New moderation/admin flows.
* Deep backend forum model redesign beyond category seed expansion.
* Full thread detail page redesign beyond preserving current functionality in the new shell.

## Open Questions

* None.

## Technical Notes

* Reference image: `D:\腾讯电脑管家截图文件\局部截取_20260702_221342.png`
* Existing main template: `public/index.html`
* Existing browser behavior: `public/extensions.js`
* Existing styles: `public/styles.css`
* Existing category API: `api/routes/forum.go`
* Existing forum migration only seeds `daily`: `api/db/migrations/sqlite/app/035_add_forum.sql`
* Existing architecture/spec context: `.trellis/spec/backend/index.md`, `.trellis/spec/guides/index.md`
