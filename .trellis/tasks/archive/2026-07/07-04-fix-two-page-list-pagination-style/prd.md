# Fix Two Page List Pagination Style

## Goal

修复帖子列表分页在日常分类存在 2 页数据时仍与帖子详情回复分页样式不一致的问题。列表分页不应只显示“上一页 + 当前页摘要 + 下一页”，而应像回复分页一样显示页码按钮。

## Requirements

* 帖子列表分页组件需要使用与帖子详情回复分页一致的结构和视觉：上一页按钮、页码按钮、下一页按钮。
* 当日常分类有 2 页数据时，列表分页应显示页码按钮 `1`、`2`，当前页使用 `aria-current="page"` 和相同高亮样式。
* 列表上方和下方分页操作区都要同步显示同一套页码按钮。
* 点击列表页码按钮应更新 `forumState.page` 并调用现有 `loadThreads()`。
* 保持现有分类、搜索、排序切换时重置到第 1 页的行为。
* 回复分页行为和后端 API 不变。
* 不实现推广内容插入能力。

## Acceptance Criteria

* [ ] 日常分类存在 2 页数据时，列表分页显示上一页、`1`、`2`、下一页。
* [ ] 当前列表页页码按钮高亮样式与回复分页当前页一致。
* [ ] 列表顶部和底部分页区域显示一致，并任一处点击页码后同步更新。
* [ ] 列表分页仍正确禁用无效上一页/下一页按钮。
* [ ] 回复分页没有视觉或行为退化。
* [ ] `node --check public/extensions.js` 通过。
* [ ] `git diff --check` 通过，允许 Windows CRLF 提示。

## Definition of Done

* Fix committed.
* Task archived after commit.
* No backend/API changes unless required by verification.

## Out of Scope

* 不修改帖子列表 API 或回复分页 API。
* 不新增每页数量选择器、跳页输入框或无限滚动。
* 不实现推广、广告或推荐内容插入列表。
* 不重构整体论坛布局。

## Technical Approach

复用现有回复分页的页码生成思路。可以抽取或复用 `replyPageItems(current, total)`，为帖子列表新增 `renderThreadPagination(page)` / `syncThreadPagers(page)` 的页码按钮渲染。列表分页 DOM 应由 JS 根据 API `pagination` 元数据生成，而不是只更新 `page-summary` 文本。

## Technical Notes

* Current list pager markup is in `public/index.html`.
* Current list pager state sync is `syncThreadPagers(page)` in `public/extensions.js`.
* Current reply pager is `renderReplyPagination(page)` and `replyPageItems(current, total)` in `public/extensions.js`.
* Shared styling lives in `public/styles.css` for `.pager-button`, `.page-summary`, and `.reply-page-button`.
