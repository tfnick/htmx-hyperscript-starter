# 修复帖子详情区域布局与首页一致

## Goal

修复帖子详情页的内容区域显示问题，让 `/post-{id}-{page}` 页面在桌面和响应式布局下与首页保持一致的结构：左侧分类栏、中央内容面板，以及嵌在内容面板右侧的社区功能侧栏。

## What I already know

* 用户要求创建任务并修复帖子详情区域显示问题，布局需要与首页一致。
* 首页 `public/index.html` 使用 `main.page-shell` 承载左侧分类栏和 `section.feed-panel` 两列；右侧栏位于 `feed-panel` 内部。
* 详情页 `public/post.html` 当前把 `aside.right-sidebar` 放在 `page-shell` 的直接子元素位置，而 `.page-shell` CSS 只有两列，容易导致详情页出现额外列、错位或宽度不一致。
* 帖子详情正文由 `public/extensions.js` 渲染到 `#thread-detail`，本任务不需要改变 API 或数据字段。

## Requirements

* 详情页的主要 HTML 外壳应与首页的布局层级一致。
* 桌面宽度下，帖子详情内容和右侧栏应同处 `feed-panel` 内部，沿用首页的 `feed-panel` 两列布局。
* 中小屏下继续沿用现有响应式规则，内容区和右侧栏应自然堆叠。
* 保持 `extensions.js` 的详情渲染入口 `#thread-detail`、登录面板 ID、发帖入口 ID 和 toast 节点可用。
* 不改变后端 API、路由语义或帖子详情数据结构。

## Acceptance Criteria

* [ ] `post.html` 的布局结构与 `index.html` 的首页主布局保持一致。
* [ ] `/post-*` 路由仍能渲染帖子详情模板。
* [ ] `node --check public/extensions.js` 通过。
* [ ] `go test ./...` 通过。

## Definition of Done

* Tests added/updated where appropriate.
* Lint / syntax checks pass for touched JS.
* Go tests pass for template route behavior.
* Existing user changes are preserved.

## Technical Approach

优先进行最小 HTML/CSS 调整：把详情页右侧栏移动进 `feed-panel`，使其与首页共用同一套 CSS grid 规则；必要时补少量 CSS，让详情页内容区域和首页列表区域在同一布局容器下表现一致。

## Decision (ADR-lite)

**Context**: 首页和详情页使用同一套 `page-shell` / `feed-panel` 样式，但详情页的 DOM 层级偏离首页，导致布局显示不一致。

**Decision**: 复用首页的 DOM 层级和现有 CSS，而不是新增详情页专用大布局。

**Consequences**: 改动范围小，外观一致性更强；详情页仍保留独立的 `thread-detail-panel` 和 `thread-detail-card` 供正文样式使用。

## Out of Scope

* 不重做帖子详情内容卡片视觉风格。
* 不调整回复分页、回复表单、API 请求或 SEO 内容策略。
* 不处理当前仓库中与本任务无关的未提交改动。

## Technical Notes

* Relevant files inspected: `public/index.html`, `public/post.html`, `public/styles.css`, `public/extensions.js`, `public/components/forum/thread-detail.html`.
* Relevant spec: `.trellis/spec/frontend/page-guidelines.md`.
* Verification: `node --check public/extensions.js` passed.
* Verification: `go test .` passed.
* Verification: `go test . -run "TestPostRoute|TestFrontend|Template|External"` passed.
* Full-suite note: `go test ./...` currently fails in pre-existing unrelated backend tests: `api/framework/archguard` import boundary checks and `api/usecase` KB embedding tests.
* Follow-up bugfix: removed the internal background, shadow, radius, padding, and border from the post detail card and reply rows while keeping the outer `feed-panel` layout.
* Follow-up verification: `node --check public/extensions.js`, `go test .`, and `go test . -run "TestPostRoute|TestFrontend|Template|External"` passed.
