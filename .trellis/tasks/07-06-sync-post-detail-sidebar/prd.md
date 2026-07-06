# 同步帖子详情右侧栏内容

## Goal

让帖子详情页右侧区域的内容与首页帖子列表区域右侧内容一致，避免同一论坛主体验在列表页和详情页之间出现不同的登录卡片、快捷入口和社区信息。

## What I already know

* 用户要求创建一个任务：帖子详情区域右侧内容应与首页帖子列表区域右侧内容一致。
* 首页 `public/index.html` 的右侧栏位于 `aside.right-sidebar`，内容包括 `side-card user-card`、`side-card quick-card` 和 `side-card members-card`。
* 帖子详情页 `public/post.html` 当前右侧栏内容仍是旧结构：`side-card auth-card`、`side-card quick-card` 和 `sponsor-stack`。
* 首页右侧栏包含登录/已登录 profile 面板、多个 `data-create-post-link` 入口、用户统计卡片和新用户展示；详情页应与这些内容保持一致。
* `public/extensions.js` 会更新 `data-create-post-link`、`#create-post-link`、`#create-post-link-sidebar` 等入口，需要保持 DOM ID/data attribute 可用。
* 当前仓库还有其他未提交改动，本任务实现和提交时需要只纳入本任务相关变更。

## Requirements

* 帖子详情页右侧栏内容应与首页帖子列表页右侧栏内容一致。
* 同步后的详情页右侧栏应保留登录态切换所需的 `logged-out-panel`、`logged-in-panel`、`current-user-name`、`logout-button` 等节点。
* 发帖入口应使用首页一致的 `data-create-post-link` 约定，确保当前分类同步逻辑继续可用。
* 不改变帖子详情主体、回复渲染、API 请求或路由语义。
* 保持移动端响应式行为沿用现有 `.right-sidebar` / `.feed-panel` CSS。

## Acceptance Criteria

* [ ] `public/post.html` 的 `aside.right-sidebar` 内容与 `public/index.html` 对应右侧栏结构一致。
* [ ] 详情页不再显示旧的 sponsor stack 右侧内容。
* [ ] `node --check public/extensions.js` 通过。
* [ ] `go test . -run "TestPostRoute|TestFrontend|Template|External"` 通过。

## Definition of Done

* HTML 结构同步完成，必要 ID 和 `data-*` 契约保留。
* 相关前端语法和模板路由测试通过。
* 不提交与本任务无关的已有工作区改动。

## Technical Approach

优先用最小模板同步：将首页 `aside.right-sidebar` 的内部内容复制到详情页对应 `aside.right-sidebar`，并只调整必要锚点（例如详情页内的快捷链接目标）以避免无效跳转。若发现重复结构后续需要抽成组件，应另起任务或在 PRD 中记录，不在本任务扩大范围。

## Decision (ADR-lite)

**Context**: 首页与详情页都展示论坛主体验，但右侧栏内容分叉，导致用户在详情页看到旧版卡片和赞助区。

**Decision**: 本任务先做页面内容同步，让详情页直接对齐首页右侧栏；不引入新组件抽象。

**Consequences**: 改动直观、风险小；右侧栏仍存在复制结构，未来若继续多页面复用可再抽 `public/components/**` 片段。

## Out of Scope

* 不重构公共右侧栏组件。
* 不调整首页右侧栏设计。
* 不改变登录、发帖、分类或回复相关 JS 行为。

## Technical Notes

* Relevant files inspected: `public/index.html`, `public/post.html`, `public/extensions.js`, `public/styles.css`.
* Relevant spec: `.trellis/spec/frontend/page-guidelines.md`.
* Follow-up reply layout: detail-page replies now render avatar, author/time, floor number, body, and footer action links/counters.
* Follow-up verification: `node --check public/extensions.js` and `go test . -run "TestPostRoute|TestFrontend|Template|External"` passed.
* Follow-up detail actions: main post detail now renders like/dislike/quote/reply actions, and the reply section no longer shows a visible `回复` heading or empty no-replies copy.
* Follow-up verification: `node --check public/extensions.js`, `go test . -run "TestPostRoute|TestFrontend|Template|External"`, and diff checks passed.
* Implemented by replacing `public/post.html` detail sidebar content with the homepage-style `user-card`, `quick-card`, and `members-card` structure.
* Preserved auth/profile DOM contracts: `logged-out-panel`, `logged-in-panel`, `current-user-avatar`, `current-user-name`, `logout-button`, `create-post-link`, and `data-create-post-link`.
* Removed the old detail-page `auth-card` and `sponsor-stack` sidebar content.
* Verification passed: `node --check public/extensions.js`.
* Verification passed: `go test . -run "TestPostRoute|TestFrontend|Template|External"`.
