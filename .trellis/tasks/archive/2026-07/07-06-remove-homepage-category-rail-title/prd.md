# Remove Homepage Category Rail Title

## Goal

Remove the visible `▦ 所有版块` heading from the homepage left category navigation area.

## Requirements

- Remove the visible `h2` text `▦ 所有版块` from the homepage category rail.
- Keep the category list mount point `#category-list` unchanged.
- Keep the surrounding semantic `aside` and its `aria-label` so the navigation still has an accessible name.
- Do not change category loading, ordering, CSS behavior, or other pages.

## Acceptance Criteria

- [ ] `public/index.html` no longer renders `▦ 所有版块` in the category card.
- [ ] `#category-list` remains inside the left category rail.
- [ ] Homepage template tests pass.

## Definition of Done

- Frontend specs were read.
- Change is scoped to homepage HTML.
- Relevant tests/checks are run and recorded.

## Technical Approach

Delete the `h2` element inside `.category-card` in `public/index.html`; no CSS change should be necessary.

## Out of Scope

- No category list styling changes.
- No route/API/JS changes.
- No changes to other active Trellis tasks.

## Technical Notes

- `rg` found the target text only in `public/index.html`.
