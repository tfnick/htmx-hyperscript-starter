# Remove Homepage Feed Heading Area

## Goal

Remove the intermediate homepage feed heading area between the `新评论` / `新帖子` tabs and the first thread row.

## Requirements

- Remove the visible board heading area from the homepage feed.
- Remove the redundant secondary sort select in that area.
- Keep the `新评论` / `新帖子` tab buttons and their sorting behavior.
- Keep `#thread-list` unchanged so thread rendering continues to work.
- Preserve an accessible name for the feed panel after removing `#board-title`.
- Do not change backend APIs, thread rendering, or category loading.

## Acceptance Criteria

- [ ] The first thread list item appears directly after the feed toolbar area.
- [ ] `public/index.html` no longer contains `.board-heading`, `#board-title`, or `#sort-select`.
- [ ] Sort tabs still exist and can update `forumState.sort`.
- [ ] Root Go template tests pass.

## Definition of Done

- Frontend specs were read.
- Changes are scoped to homepage HTML/CSS.
- Relevant tests/checks are run and recorded.

## Technical Approach

Delete the `.board-heading` section from `public/index.html`, replace the feed panel `aria-labelledby` with a stable `aria-label`, and remove unused `.board-heading` / `.sort-select` CSS. Keep `.eyebrow` because other pages and rendered thread cards still use it.

## Out of Scope

- No changes to thread list item rendering.
- No changes to sort tab behavior.
- No backend/API changes.

## Technical Notes

- `extensions.js` guards both `#sort-select` and `#board-title` before using them.
