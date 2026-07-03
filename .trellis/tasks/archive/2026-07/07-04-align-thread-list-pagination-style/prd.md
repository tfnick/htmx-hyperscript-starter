# Align Thread List Pagination Style

## Goal

修复帖子列表分页组件与帖子详情回复分页组件风格不一致的问题。帖子列表上下两个分页操作区应采用与回复分页一致的按钮视觉、当前页表现、禁用态和间距风格。

## Requirements

* 帖子列表分页组件风格需要与帖子详情回复分页组件保持一致。
* 列表上方和下方分页操作区都要同步应用一致样式。
* 保持现有列表分页行为：上一页、下一页、分页摘要同步、分类/搜索/排序重置页码不变。
* 优先复用或合并现有 `.reply-pagination` / `.reply-page-button` 的视觉规则，避免复制出第三套分页样式。
* 不改变后端 API、分页数据流、回复分页逻辑或 URL 行为。

## Acceptance Criteria

* [ ] 帖子列表上方分页与详情回复分页在按钮边框、背景、hover/current/disabled、间距上保持一致。
* [ ] 帖子列表下方分页与上方分页状态同步，并同样使用一致风格。
* [ ] 详情回复分页原有视觉不退化。
* [ ] `node --check public/extensions.js` 通过。
* [ ] `git diff --check` 通过，允许 Windows CRLF 提示。

## Definition of Done

* Fix committed.
* Task archived after commit.
* No unrelated backend or API behavior changes.

## Out of Scope

* 不新增列表页码按钮逻辑，除非为达成视觉一致必须复用已有样式。
* 不修改回复分页 API 或帖子列表 API。
* 不调整整体页面布局、颜色体系或响应式断点。

## Technical Notes

* Current list pager markup: `public/index.html` `.feed-pager`, `.pager-button`, `.page-summary`.
* Current reply pager markup: `public/extensions.js` `renderReplyPagination()`, `.reply-pagination`, `.reply-page-button`.
* Likely target: `public/styles.css`.
