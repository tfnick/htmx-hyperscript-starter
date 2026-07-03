# Fix Reply Pagination Alignment

## Goal

修复帖子详情页回复分页操作区的视觉对齐问题：回复分页应在回复列表区域内居右显示，而不是居左。

## Requirements

* 只调整帖子详情回复分页操作区的对齐方式。
* `.reply-pagination` 应居右显示。
* 保持现有回复页码按钮、上一页/下一页、禁用态和当前页状态不变。
* 不改变帖子列表分页、后端 API、回复分页数据流或 URL 行为。
* 不引入新的布局组件或分页逻辑。

## Acceptance Criteria

* [ ] 帖子详情页回复分页操作区在视觉上居右。
* [ ] 回复页码按钮仍可点击并保持当前页、禁用态样式。
* [ ] `node --check public/extensions.js` 通过，确认未破坏共享脚本。
* [ ] CSS 改动范围局限在回复分页对齐相关样式。

## Definition of Done

* Fix committed.
* Task archived after commit.
* No unrelated frontend or backend behavior changes.

## Out of Scope

* 不修改回复分页 API。
* 不修改帖子列表分页。
* 不修改页码按钮生成逻辑。
* 不做浏览器整体样式重构。

## Technical Notes

* Likely target: `public/styles.css` `.reply-pagination`.
* Related generated markup: `public/extensions.js` `renderReplyPagination()`.
